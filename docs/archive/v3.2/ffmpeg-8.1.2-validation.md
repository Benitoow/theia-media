# FFmpeg 8.1.2-4 et correction de lecture Edge

11 septembre 2026. Ce lot remplace le runtime FFmpeg 6.1.1 de la preview 4,
ferme l'échec de première lecture observé sur une installation neuve et corrige
la saturation du tampon MediaSource dans Microsoft Edge. Il ne change ni le
contrat HTTP de lecture, ni la cible de transcodage H.264 SDR, ni le budget de
concurrence.

## Pourquoi 8.1.2-4, et non « le dernier numéro »

Le fournisseur historique `eugeneware/ffmpeg-static` s'arrête à FFmpeg 6.1.1.
FFmpeg 9.0.1 est disponible en source, mais aucun fournisseur de binaires évalué
ne publie la même famille de build immuable pour les six cibles de Theia :
Windows, Linux et macOS, chacun en AMD64 et ARM64. Assembler plusieurs
fournisseurs aurait donné des codecs et filtres différents selon l'OS, donc une
compatibilité impossible à promettre ou à diagnostiquer.

Le choix est **Jellyfin FFmpeg 8.1.2-4**, publication GitHub immuable du
6 septembre 2026 :

- six paquets GPL natifs, dont enfin un Windows ARM64 qui ne dépend plus de
  l'émulation x64 ;
- un même fabricant de build et une même version déclarée
  (`ffmpeg version 8.1.2-Jellyfin`) partout ;
- sous Windows AMD64, présence vérifiée de `h264_amf`, `h264_mf`,
  `h264_nvenc`, `h264_qsv`, `libx264`, `zscale` et `tonemap` ;
- licence GPL compatible avec le dépôt GPL-3.0. Le seul module Go ajouté,
  `github.com/ulikunitz/xz` 0.5.16, est BSD 3-Clause et compilé statiquement :
  il n'ajoute aucune dépendance d'exécution.

La publication 8.1.2-4 elle-même corrige notamment l'extradata de l'encodeur
AMD `hevc_vaapi`. Le saut depuis 6.1.1 apporte surtout deux générations de
correctifs de décodeurs, démultiplexeurs et filtres, dont de nombreux contrôles
de bornes pour Matroska, MOV, HEVC et AAC. FFmpeg 7 a aussi parallélisé les
étages demux/décodage/filtrage/encodage/mux du CLI ; FFmpeg 8 a réactivé
davantage d'autovectorisation sur x86, ARM et AArch64. Ces changements peuvent
expliquer une partie des gains, mais Theia ne leur attribue pas un chiffre sans
profilage : seuls les résultats de bout en bout ci-dessous sont revendiqués.

Les nouveaux encodeurs D3D12, codecs VVC/APV et filtres GPU présents dans les
branches 7 et 8 ne changent pas automatiquement le produit. Theia continue de
sonder et de choisir uniquement les chaînes déjà autorisées. Leur présence est
une possibilité future, pas une promesse de cette release.

Sources :

- <https://github.com/jellyfin/jellyfin-ffmpeg/releases/tag/v8.1.2-4>
- <https://github.com/FFmpeg/FFmpeg/blob/n8.1.2/Changelog>

## Téléchargement et provenance

Jellyfin livre des archives plutôt qu'un exécutable nu. Le gestionnaire sait
donc maintenant lire ZIP (Windows) et tar.xz (Linux/macOS), mais extrait
**uniquement** `ffmpeg` ou `ffmpeg.exe` : `ffprobe` ne devient pas une nouvelle
dépendance. Le chemin de confiance est :

1. téléchargement sur un nom temporaire non exécutable, borné à 256 Mio ;
2. vérification SHA-256 du paquet contre le digest de la release GitHub ;
3. extraction bornée à 256 Mio du seul exécutable attendu ;
4. vérification d'un second SHA-256, calculé sur cet exécutable ;
5. permission d'exécution, renommage atomique puis contrôle de la version
   déclarée avant le premier lancement utile.

Les six archives ont été réellement téléchargées et leurs deux hashes ont été
recalculés. Les valeurs complètes sont la source unique dans
`internal/ffmpeg/ffmpeg.go` et ressortent dans `/api/diagnostics`. Sous Windows
AMD64, l'installation neuve observée a produit :

- paquet : `a6821d72985ee6d5a8af16925b468d1c4ec1f652b582a0a2a5039282c26ffca5` ;
- exécutable : `546580347aa7553ea9ac5fbcba05b787e423f9ce0685a71e7500b554cc21bc6c` ;
- version exécutée : `8.1.2-Jellyfin` ;
- archive téléchargée : 64,67 Mio contre 78,96 Mio pour l'ancien exécutable
  nu ; exécutable installé : 87,88 Mio.

`/info`, le seek et les sous-titres gardent leur règle : ils ne dépendent jamais
d'un téléchargement. Seul un premier besoin réel prépare FFmpeg.

## Cause et correction de l'échec Edge

Le message Edge « format non pris en charge » masquait une saturation MSE, pas
un crash de l'encodeur AMD. Le lecteur demandait 30 secondes d'avance quel que
soit le débit. À 54,95 Mb/s, le remux UHD réel représente environ **206 Mo**
pour ces seules 30 secondes. Edge levait `QuotaExceededError` (`SourceBuffer is
full`), annulait la requête, puis le serveur voyait normalement une connexion
abandonnée et arrêtait FFmpeg. Le `exit status 1` en aval était une conséquence
de cette annulation.

Le transport conserve maintenant le bloc refusé au lieu de le perdre, retire
les données déjà lues, puis retente ce même bloc. Chaque saturation réduit la
cible de 30 à 15, 7,5 puis 6 secondes. Les six secondes de réserve de démarrage
restent intactes ; la récupération ne saute donc aucune image. Si même ce
minimum échoue, le lecteur montre enfin `stream_buffer_full` au lieu du message
natif trompeur.

Une seconde course existait sur une installation neuve : le premier `/info`
pouvait répondre `transcode.available=false`, puis FFmpeg finir son installation
pendant la tentative HEVC. Avant d'abandonner sur
`browser_cannot_decode_video`, le lecteur recharge désormais `/info` **une
seule fois**. La lecture bascule alors vers le transcode disponible sans faire
attendre l'endpoint ni créer une boucle.

Aucun redémarrage aveugle du transcode n'est ajouté : il aurait répété le
travail GPU sans corriger le quota du consommateur. La nouvelle tentative porte
sur l'append MSE refusé, à l'endroit où l'échec est réellement récupérable. La
fin de stderr bornée côté serveur reste journalisée si un encodeur meurt pour
une autre raison.

## Gains mesurés sur le même remux UHD HDR

Machine : Ryzen AI 9 HX 370 / Radeon 890M, Windows 11. Source : le remux UHD
HDR10 HEVC Main 10 de 53,79 Gio décrit dans
[`real-media-validation.md`](real-media-validation.md). Arguments de production,
FFmpeg ancien et nouveau alternés, trois passages par cas ; la médiane est
retenue.

| Chemin | Fenêtre | FFmpeg 6.1.1 | FFmpeg 8.1.2 | Gain observé |
|---|---:|---:|---:|---:|
| Remux vidéo + TrueHD→AAC | 60 s | 15,87× | **53,20×** | **3,35×** ; temps mur −70 % |
| HEVC HDR→H.264 SDR 1080p, AMF | 15 s | 1,68× | **1,94×** | **+16 %** ; temps mur −14 % |
| HEVC HDR→H.264 SDR 2160p, AMF | 10 s | 0,98× | **1,08×** | +10 % sur cette courte fenêtre |

Le dernier chiffre ne réhabilite pas le rung 2160p : 8 % de marge sur dix
secondes ne prouve pas une lecture soutenue de 140 minutes, et les campagnes
plus longues l'ont déjà mesuré sous 1×. L'escalade HDR reste donc à 1080p et un
tone map continue de consommer tout le budget.

### Reproduction du candidat de release - 12 septembre 2026

Une nouvelle alternance de trois passages par runtime a été exécutée avant la
rédaction de la note de version, sur les deux fichiers réels et avec les mêmes
arguments de production. Elle n'a pas reproduit l'ampleur du gain de la campagne
ci-dessus : ce résultat historique reste la trace de sa session, mais ne doit
pas servir seul de titre public.

| Chemin | FFmpeg 6.1.1 | FFmpeg 8.1.2 | Écart actuel |
|---|---:|---:|---:|
| Remux UHD HDR 53,79 Gio, 60 s | 12,789 s (4,69×) | **11,262 s (5,33×)** | temps mur **−11,9 %** |
| Remux MKV HEVC 7,21 Gio, 60 s | 3,314 s (18,11×) | **2,522 s (23,79×)** | temps mur **−23,9 %** |
| Tone mapping 1080p, 15 s | 11,768 s (1,27×) | 11,749 s (1,28×) | stable |

La note de version reprend ces chiffres plus conservateurs. Le bénéfice
reproductible de 8.1.2 est ici le remux ; l'amélioration du tone mapping observée
la veille n'est pas confirmée par cette seconde campagne.

Sur le parcours produit réel avec le nouveau runtime : image transcodée à
8,4 s après le clic, seek à 3 600 s, pause/reprise, aucune erreur console et
un processus FFmpeg pendant la lecture puis zéro après fermeture. L'ancien
parcours donnait une image autour de 10–11 s ; cette comparaison de sessions
est indicative, pas un microbenchmark.

## Validation Microsoft Edge

Microsoft Edge 152 a été lancé contre un data-dir neuf, sans binaire dans
`bin/`. Theia a téléchargé, vérifié et extrait 8.1.2-4, puis Edge a décodé le
remux HEVC 3840×2160. Après le dernier seek volontaire, le flux a livré
648 252 768 octets pendant 2 min 41 s sans `playback_error` ni
`SourceBuffer is full`. La fermeture du harnais a annulé la connexion comme
prévu et n'a laissé aucun FFmpeg.

Le chemin de compatibilité réel a aussi été validé dans un navigateur sans
HEVC : événement `quality_adapted` avec
`reason_code=browser_cannot_decode_video`, tone mapping 1080p via `h264_amf`,
image décodée, seek profond, pause/reprise, aucune erreur console et aucun
orphelin.

Le binaire Theia Linux AMD64 cross-compilé a ensuite été exécuté sous WSL dans
un data-dir neuf. Il a téléchargé et extrait son propre tar.xz en 41,6 s,
vérifié la version et le hash `9302bc18…`, sondé `libx264`, puis lancé le même
remux réel. Le client de validation a reçu 7 939 480 675 octets pendant 3 min
18 s de flux avant son timeout volontaire ; l'annulation a tué FFmpeg et aucun
processus n'est resté.

## Vérifications

- `go test ./...` et `go vet ./...` : verts ;
- 35 tests unitaires frontend : verts ;
- Playwright interface : 81 réussites, 3 skips conditionnels attendus ;
- Playwright playback : 10/10, dont la course `/info` neuve et une
  `QuotaExceededError` MSE injectée avec reprise du bloc refusé ;
- build complet frontend + binaire Windows : vert ;
- six cross-builds `CGO_ENABLED=0` : Windows/Linux/macOS × AMD64/ARM64 ;
- installation et exécution réelles des archives Windows AMD64 et Linux AMD64
  (sous WSL) ; les quatre binaires ARM64/macOS ont été extraits et hashés, mais
  ne peuvent pas être exécutés sur cette machine. Leur exécution reste à
  confirmer sur les OS correspondants ou en CI.
