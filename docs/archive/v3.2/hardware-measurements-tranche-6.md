# Mesures matérielles - tranche 6 de la refonte de la lecture

Campagne du 11 septembre 2026 sur la machine du mainteneur : AMD Ryzen AI 9 HX
370 avec Radeon 890M (iGPU RDNA 3.5, mémoire partagée), 24 processeurs
logiques, Windows 11. Runtime FFmpeg épinglé `b6.1.1`, SHA-256
`04e1307997530f9cf2fe35cba2ca7e8875ca91da02f89d6c7243df819c94ad00` - vérifié
avant la campagne. Le script reproductible est
`scripts/measure-hardware` : chaque candidat fait le travail, rien n'est déduit
d'une liste ; deux passages chronométrés par scénario, le plus rapide conservé.
Cette campagne reste la référence historique de 6.1.1. Le runtime a ensuite été
qualifié en 8.1.2-4 ; son A/B sur le même média et sa provenance sont dans
[`ffmpeg-8.1.2-validation.md`](ffmpeg-8.1.2-validation.md).

Le contrat de livraison n'a pas bougé : la cible reste H.264 SDR, l'escalade
`?video=transcode` reste celle du navigateur, et aucune capacité n'a été
adoptée sur la foi d'une documentation seule (décision 58).

## 1. Ce que la machine peut faire (sonde, une image par candidat)

| Codec | Encodeur | Verdict |
|---|---|---|
| H.264 | `h264_nvenc` | refusé (pas de NVIDIA) |
| H.264 | `h264_qsv` | refusé (pas d'Intel) |
| H.264 | `h264_amf` | **utilisable** |
| H.264 | `h264_mf` | utilisable |
| H.264 | `libx264` | utilisable |
| HEVC | `hevc_nvenc` / `hevc_qsv` / `hevc_mf` | refusés |
| HEVC | `hevc_amf` | **utilisable** |
| HEVC | `libx265` | utilisable |
| AV1 | `av1_nvenc` / `av1_qsv` / `libsvtav1` | refusés |
| AV1 | `av1_amf` | **utilisable** |

Décodage matériel de la source 4K HEVC (10 s, vers 720p, sans encodeur) :

| Décodeur | Temps mur | Temps réel |
|---|---:|---:|
| aucun (logiciel) | 717 ms | **13,95x** |
| `d3d11va` | 1,74 s | 5,75x |
| `dxva2` | 1,86 s | 5,38x |

Le décodage logiciel bat l'accélération de 2,4x sur cette machine : l'iGPU
partage la mémoire du CPU, et la copie de retour de chaque frame coûte plus que
le décodage n'économise. C'est la même conclusion que la sonde de découte de
`decoders.go`, qui avait déjà mesuré 5,85x logiciel contre 4,07x `d3d11va` sur
un clip 1080p H.264 - ici sur la vraie question 4K HEVC, l'écart se creuse. Le
choix du runtime (repère par machine, logiciel par défaut) est confirmé ; rien
à changer.

## 2. Les chaînes : source 4K HEVC SDR → 720p H.264

Scénario d'acceptation de la tranche 6. Sources synthétiques (testsrc2, 10 s) ;
mêmes formes d'arguments que la livraison (mux MP4 fragmenté, `-movflags`
compris) :

| Encode | Decode | Temps mur | Temps réel |
|---|---|---:|---:|
| `h264_mf` | aucun | 1,12 s | 8,95x |
| `libx264` | aucun | 1,12 s | 8,90x |
| `h264_amf` | aucun | 1,43 s | 7,01x |
| `libx264` | `d3d11va` | 1,99 s | 5,03x |
| `h264_amf` | `d3d11va` | 2,05 s | 4,89x |
| `h264_mf` | `d3d11va` | 2,18 s | 4,58x |
| `libx264` | `dxva2` | 2,27 s | 4,40x |
| `h264_mf` | `dxva2` | 2,55 s | 3,92x |
| `h264_amf` | `dxva2` | 3,21 s | 3,12x |

Cas quotidien pour référence (1080p H.264 → 720p) : `h264_mf`+aucun 15,4x,
`h264_amf`+aucun 11,4x, `libx264`+aucun 13,1x. Le classement reste le même.
HEVC pour mémoire : `hevc_amf` encode la même source à 6,99x temps réel - un
jour de cible HEVC, l'encodeur existe sur cette machine.

## 3. Pourquoi la chaîne la plus rapide n'est pas celle retenue

`h264_mf` mesure 28 % plus vite que `h264_amf` sur la source 4K. La raison pour
laquelle la liste de candidats le place derrière les encodeurs spécifiques au
constructeur est documentée : `h264_mf` est un shim au-dessus de ce que Windows
décide, et il n'expose pas les contrôles que la pipeline impose. Mesuré
directement, avec une source rendue bruyante et un plafond serré de 2 Mb/s
(`-b:v 2M -maxrate 2M -bufsize 3M`, 6 s de sortie) :

| Encode | Débit produit | Plafond demandé |
|---|---:|---:|
| `h264_amf` | 2,19 Mb/s | 2 Mb/s |
| `h264_mf` | **2,87 Mb/s** | 2 Mb/s |
| `libx264` | 1,86 Mb/s | 2 Mb/s |

`h264_mf` dépasse le plafond de 44 % : un film transcodé avec lui sort de la
fourchette que la grille de débit et le lecteur dimensionnent, sur le réseau
d'une maison. C'est exactement la « régression vérifiée » que la tranche 6
refuse d'acheter contre de la vitesse. La vitesse mesurée de `libx264` (8,90x,
essentiellement parce que cette machine a 24 cœurs) ne le rend pas non plus
retenu : un transcodage logiciel consomme la marge entière du CPU - c'est
pourquoi le budget transcode lui donne un seul créneau (limiter de
`internal/playback`) là où le matériel en prend trois.

**La chaîne la plus rapide qui respecte les contraintes de livraison est donc
`h264_amf` + décodage logiciel - celle que le runtime choisit déjà.** La sonde
de disponibilité et l'ordre de priorité de `Best()` sont confirmés par la
mesure ; aucun changement du moteur n'est adopté sur cette machine. Les
résultats des autres familles (NVIDIA, Intel, Apple) attendent une campagne
sur les machines correspondantes avec le même script.

## 4. Qualité

PSNR de la chaîne la plus rapide (`h264_mf`+aucun) contre la chaîne tout
logiciel (`libx264`+aucun), même cible 720p H.264 : **51,49 dB** -
indiscernable, ce qui ferme la question de la qualité et laisse la discipline
de débit trancher.

## 4bis. Les étages de l'escalade de compatibilité, mesurés (11 septembre 2026)

Symptôme rapporté sur la machine de lecture (RTX 5070 Ti) : les remux H.264
jouent sans problème, les x265 font « pose, reprend, pause », déjà en preview
3. Le chemin côté serveur, mesuré avec les arguments de production exacts
(`stream.TranscodeArgs`, `h264_amf`, sources synthétiques 10 s, plus rapide de
deux passages) : à la taille source 4K, un transcode tone-mappé tourne à
**1,17x temps réel** - le flux n'arrive que 17 % plus vite qu'il ne se lit,
ce qui est exactement la signature d'un tampon qui se vide, pose, récupère,
re-pose. La mesure fraîche confirme le 1,09x historique de la décision 87.

Les étages que le lecteur choisit réellement (`initialCompatibilityHeight`,
puis `nextLowerHeight` à chaque série d'à-coups, décision 99) : une source 4K
HDR escalade vers le **1080p tone-mappé - 2,98x** ; une 4K SDR vers la taille
source - **5,93x** ; une 1080p HDR vers la taille source tone-mappée -
**4,00x**. Aucun de ces étages n'est sur le fil du rasoir sur cette machine
serveur.

Conséquence pour le diagnostic : si le saccadement persiste après l'escalade,
il ne vient pas du throughput du transcode sur cette machine serveur. Les
candidates restantes se lisent dans les diagnostics de la session (décisions
98–102) : le `reason_code` de l'escalade (`slow_decode` /
`remembered_slow_decode` / `browser_cannot_decode_video` - le décodage HEVC du
navigateur lui-même, qui est logiciel dans MediaSource quel que soit le GPU,
ce que la note du lecteur documentait déjà), la profondeur de tampon après
escalade (réseau, wifi), et les `quality_adapted` successifs. Le NVDEC de la
5070 Ti n'intervient jamais dans ce chemin : c'est Chrome qui choisit son
décodeur pour MediaSource, et `canPlayType`/`decodingInfo` répondent « smooth »
sur les machines où ça ne marche pas - d'où la garde de rythme mesurée.

## 5. Ce qui reste hors de cette campagne

Les cibles de diffusion HEVC et AV1 (changement de contrat), le tone mapping
GPU (le runtime épinglé n'emporte aucun filtre GPU de tone mapping - `vpp_amf`
absent, voir la preview 3), les chaînes GPU complètes pour source SDR (aucun
scaler GPU AMD dans ce build), le 4:2:2, HLS, ffprobe. Les machines des neuf
autres foyers de test-terrain produiront leurs propres tableaux avec
`go run ./scripts/measure-hardware -ffmpeg <ffmpeg>`.
