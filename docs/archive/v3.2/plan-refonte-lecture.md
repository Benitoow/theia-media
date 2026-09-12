# THEIA - Plan de refonte du backend de lecture

Document de travail du 10 septembre 2026. Le périmètre (« tout »), le gel du
contrat HTTP et la méthode (plan validé avant implémentation) ont été arrêtés
avec le mainteneur le même jour ; **aucune ligne de code n'est écrite avant la
validation de ce plan.**

---

## État d'avancement (11 septembre 2026, clôture du chantier)

Suivi du 12 septembre : la preview 5 ne rouvre pas cette refonte backend. Elle
ajoute autour d'elle le découpage ciblé du lecteur, le cycle de pause longue,
la matrice de navigateurs, l'endurance et le bout-en-bout de mise à jour. Voir
[`engine-modernisation-3.2-preview5.md`](engine-modernisation-3.2-preview5.md)
et les décisions 111 à 115.

Règle permanente : chaque tranche finit avec `go test ./...` et `go vet ./...`
verts, une revue du diff, et un rapport de ce qui est vérifié contre ce qui ne
l'est pas. Aucun commit n'a été fait ; l'état Codex (preview 3, décisions
98–102) reste non commité et sert de base, comme convenu.

| Tranche | État | Preuves |
|---|---|---|
| 1. Filet doré | **Terminée** | `internal/api/playback_contract_test.go` : 8 tests verts, 0 ligne de production touchée à l'époque de la tranche. Épingle : décisions conteneur et mesurées, codes 400/404/415/503, seek sans binaire, équivalence legacy (et son écart `?audio=` documenté), asymétrie `?h` film/épisode, sidecar externe rebasé, piste bitmap refusée, épisode non mesuré |
| 2. Plan unique | **Terminée** | `internal/playback/plan.go` câblé sur les 4 sites ; `decisionReasonCode` et la branche morte de `qualityLadder` supprimés. Les corps dorés (`internal/api/testdata/*.golden.json`, 6 fichiers) sont générés et stables entre exécutions - la première génération avait échoué parce que `goldenBody` écrivait dans `testdata/` sans créer le répertoire (`os.MkdirAll` ajouté, cause consignée ici pour la prochaine fois) ; le test passe désormais en comparaison pure, deux fois de suite |
| 3. Cycle de vie | **Terminée** | `internal/playback` : service d'exécution unique + `Sessions` (réserve avant spawn, attachement du processus, `KillAll`). `main.go` garde la référence du `*api.Server` : kill avant le drain gracieux et avant chaque `os.Exit` (mise à jour), filet `defer` pour les autres sorties. Vérifié de bout en bout : `scripts/verify-shutdown` (binaire réel, film généré, 3 flux vivants, 3 processus ffmpeg comptés dans la liste des processus, interruption console) → sortie en 20 ms, zéro orphelin. Le plafond des flux remux est de 4, refus en forme `transcode_busy` (503 + `Retry-After: 1`), le seul code que `media-transport.js` retente ; les transcodes sont enregistrés pour le kill mais budgétés par le limiteur transcode |
| 4. Adaptateurs minces | **Terminée** | Les corps jumeaux film/épisode (info, direct, remux) vivent dans `stream_twin.go` derrière `streamTwin` ; les adaptateurs gardent routes, tables et identité (décision 39). Le direct legacy partage `serveDirectContent` et garde son écart `?audio=` épinglé. Suite dorée verte ; logs unifiés (`kind`, `parent_id`) - les logs ne sont pas le contrat |
| 5. Solidité résiduelle | **Terminée** | Extraction de sous-titres bornée : `boundedio.Head`, plafond 8 Mio ; échec toujours en 415 avant toute écriture ; un dépassement commet la réponse et streame la suite plutôt que tronquer en silence. Plafond du limiteur transcode dérivé une fois du sonde de capacités (portée `Available()`, jamais de téléchargement). `Flush` après `WriteHeader` **adopté sur mesure A/B** : 75→48 ms client simple, 78→60 ms traversée gzip (10 passages chacun, même film, même machine) |
| 6. Matériel mesuré | **Terminée sur cette machine** | `scripts/measure-hardware` + [`hardware-measurements-tranche-6.md`](hardware-measurements-tranche-6.md). 4K HEVC → 720p : décodage logiciel 13,95x contre 5,75x `d3d11va` (la sonde de `decoders.go` confirmée sur la vraie question) ; `h264_amf` retenu : `h264_mf` mesure 28 % plus vite mais produit 2,87 Mb/s sous un plafond de 2 Mb/s (discipline de débit vérifiée - disqualification mesurée, la priorité documentée de `Best()` vindiquée). Le contrat ne bouge pas : cible H.264 SDR, aucune capacité adoptée sur la foi d'une documentation (décision 58). Les autres familles attendent leurs machines |

Ancre de fin de chantier : l'agrégat SHA-256 des 62 fichiers sous `web/src`
et `web/tests` vaut
`58D8EE16C11DBEC14C1BCD21E05757D6958546966DA0DF203BCB1815CEBDAD99`, calculé
par `node web/scripts/frontend-anchor.mjs` - le script est la spécification de
la méthode (chemins relatifs `web/` triés, concaténation chemin + condensat,
re-hachage). L'ancre historique de la preview 3
(`3B28D055…E515C`) n'a pas pu être reproduite depuis sa seule description -
douze lectures de sa phrase d'agrégation ont été essayées ; le script la
remplace plutôt que de répéter son ambiguïté. L'état du frontend est par ailleurs
prouvé autrement : l'ensemble des fichiers modifiés et ajoutés sous `web/` est
identique à celui du début du chantier (10 modifiés, 7 nouveaux - les changements
interface des décisions 98–102, dont aucun n'a bougé pendant la refonte).

Ancres pour la fin de chantier : remplacées par le script `web/scripts/frontend-anchor.mjs` (voir l'état d'avancement). Le build épinglé vérifié localement :
`Temp\theia-audit-20260905-001717\data\bin\ffmpeg.exe`, SHA-256 identique à la
preview 3, inventaire `-hwaccels`/`-encoders`/`-filters` consigné en §4.

---

Le but : un seul service Go possède la politique de lecture - la décision, la
résolution, l'exécution et le cycle de vie des processus - et les handlers
HTTP deviennent minces. Aujourd'hui cette politique est disséminée entre trois
familles de routes, recalculée à des endroits différents avec des prédicats
différents, et le cycle de vie d'un processus FFmpeg se réduit au contexte de
sa requête. Le frontend ne bouge pas d'un octet : les routes, paramètres,
statuts et corps JSON existants restent identiques pour des entrées identiques.

Ce plan est le détail d'exécution, côté serveur, de la §2 (*architecture
visée*) et du lot 1 (*contrats et filet*) de
[`plan-modernisation-moteur.md`](plan-modernisation-moteur.md), étendu du
chantier de cycle de vie des processus validé par le mainteneur. Une recherche
matérielle en ligne du même jour (matrice NVIDIA, docs FFmpeg, wiki AMF,
annonces Apple, support Intel) alimente la §4 ; elle reste soumise à la règle
de mesure. Il ne supersede aucune décision : 16, 23, 39, 58, 59, 87, 90, 92,
101 et 102 restent écrits tels quels.

**1. Point de départ vérifié**

Inspection de `main` à `84d9ae7` avec les modifications locales non commitées
de la préversion V3.2 (décisions 98–102). Sur cet état exact : `go test ./...`
et `go vet ./...` passent (vérifiés le 10 septembre 2026 sous Go 1.26.6), la
suite Go étant entièrement exécutée, pas seulement lue en cache.

| Observation actuelle (vérifiée dans le code) | Conséquence pour le plan |
|---|---|
| La décision de diffusion est calculée trois fois avec trois prédicats : `stream.Decide` pur (`stream/stream.go`), puis mutation dans chaque handler `/info` (`movie_file_stream.go`, `episode_stream.go`), puis re-fourche dans `serveConvertedFile` (`converted_stream.go:60` : `unsupported \|\| ?h \|\| ?video=transcode`) | `info.mode` ne prédit pas ce que fera le `remux` suivant. Une seule fonction de plan doit répondre aux deux, avec les paramètres de la requête en entrée |
| Les jumeaux film/épisode : `/info` quasi ligne pour ligne, direct play en trois copies, remux en deux, résolveurs en deux, inspect en deux, bloc « audio par défaut » en quatre exemplaires | Les règles se partagent dans le service ; les tables et routes restent distinctes (décision 39, non négociable) |
| Aucun registre de sessions. Le chemin de redémarrage draine le HTTP puis sort par `os.Exit(0)` (`cmd/theia/main.go:284`) sans tuer les processus FFmpeg suivis | Un film en lecture pendant une mise à jour peut orpheliner un FFmpeg. Le shutdown a besoin d'un filet explicite |
| Le remux n'a aucun plafond de concurrence : `transcodes.acquire` n'est appelé que dans la branche transcode (`converted_stream.go:67`) | N remux parallèles = N processus FFmpeg. Un plafond distinct du budget transcode est nécessaire |
| `workload.BeginInteractive` est tenu pendant tout le flux converti (`converted_stream.go:21-24`) | La politique « la lecture passe avant le fond » est grossière mais délibérée : la garder, la documenter, ne la raffiner qu'avec des preuves |
| Le plafond du limiter transcode est muté globalement à chaque requête (`setKind` depuis `converted_stream.go:66` et `transcode.go`) | Le dériver une fois de la sonde de capacités, pas à chaque appel |
| Aucun `Flush` après `WriteHeader` ; le client attend le premier chunk de `bufio` (`converted_stream.go:127-128`) | À décider sur mesure A/B dans la suite de lecture, jamais pour l'esthétique |
| L'extraction de sous-titres embarqués bufferise tout le WebVTT via `cmd.Output()` (`subtitles.go:157`) | Flotter ou borner ; le stderr seul est borné aujourd'hui |
| La famille de routes legacy `/api/stream/{id}` délègue en réécrivant `file_id` (`stream.go`) ; son `/info` ne porte pas les flags de transcode (décision 59) | La garder, câblée sur le même service. Son écart documenté reste ouvert : le corriger changerait le contrat, hors périmètre |
| La décision conteneur-seule de `/info` peut dire « direct » pour un MP4 au codec illisible (`stream.go` le documente comme délibéré) | Angle mort assumé, inchangé : le corriger changerait le contrat |

**Limites de cet examen :** pas de relance de lecture réelle pour préparer ce
plan ; les faits viennent de la lecture du code et de l'exécution de la suite
Go d'aujourd'hui. Les mesures de performance de la preview 3 ne sont pas
re-mesurées ici et ce chantier n'en promet aucune - c'est un chantier de
fiabilité et de structure, pas de vitesse.

**2. Contrat gelé et invariants**

- **Routes et réponses existantes identiques.** Mêmes chemins, mêmes
  paramètres, mêmes statuts, corps JSON identiques octet pour octet pour des
  entrées identiques, y compris pour la famille legacy. La seule réponse
  nouvelle autorisée est le refus au plafond des flux convertis (§3), qui
  réutilise la forme 503 + `Retry-After` déjà employée par `transcode_busy` ;
  le code exact est choisi pendant la tranche 3 après lecture du traitement
  d'erreurs de `web/src/lib/media-transport.js`, pour que le frontend actuel
  dégrade proprement sans modification.
- **Contrat de transport V3** : direct play = `http.ServeContent` avec Range ;
  flux converti = MP4 fragmenté, `no-store`, `Accept-Ranges: none` ; pas de
  200 mensonger (porte sur le premier octet conservée).
- **Horloge du seek** (16, 90, 92) : départ sur image-clé des deux flux copiés,
  timestamps rebasés à zéro, `/seek` ne corrige que vers l'arrière, mêmes
  `-ss` pour les sous-titres.
- **Promesse M1** : `/info`, `/seek`, `/preview` et les diagnostics ne
  téléchargent jamais FFmpeg ; la sonde n'a lieu que si le binaire est déjà
  sur disque, dédupliquée par fichier et version (101).
- **Tone mapping** (87) : le filtre partagé unique ; un transcode HDR coûte tout
  le budget transcode.
- **Honnêteté des capacités** (58, 59) : encodeurs et décodeurs mesurés, jamais
  déduits d'une liste ; rien au-dessus de la source ; le navigateur arbitre
  l'escalade `?video=transcode`.
- **Mise à jour** (23, 68) : admission par `activity`, `update_restarting`,
  bail de heartbeat ; une installation n'interrompt jamais une lecture.
- **Identités** (39) : tables et routes film/épisode distinctes ; le
  partage se fait dans le service, jamais dans les tables ni le SQL.
- **Réseau** : règles LAN/Origin/Host et allowlist distante énumérée de
  `remoteaccess` inchangées.
- **Sous-titres** (3) : texte seul, pistes image nommées puis refusées,
  rebasage sur `?t=`.
- **Middleware** : compression jamais sur Range ni vidéo, `Unwrap()` préservé,
  pas de WriteTimeout sur le serveur vidéo.
- **Contraintes §3 de la spec** : pas de CGO, FFmpeg seule dépendance runtime,
  sorties de sous-processus bornées, `boundedio` conservé.

Geler le contrat a une conséquence assumée : le trou de la décision 59 sur le
`/info` legacy et l'angle mort conteneur-seule **ne sont pas corrigés** par
cette refonte. Ce sont des changements de contrat, à traiter séparément.

**3. Architecture visée**

Un nouveau package `internal/playback`, quatre pièces petites, interfaces
définies là où elles sont consommées, pas de framework :

```mermaid
flowchart TD
    H1[Handler film par fichier] --> S
    H2[Handler épisode par fichier] --> S
    H3[Routes legacy /api/stream/{id}] --> S
    S[Service de lecture] --> R[Résolution : identité et chemin sûr]
    S --> P[Plan : la décision unique]
    P --> X[Exécution : seul endroit qui spawn FFmpeg]
    X --> SES[Registre de sessions : plafond et shutdown]
    X --> FF[FFmpeg]
    SES --> W[Workload : la lecture passe avant le fond]
```

- **Résolution.** Résoudre fichier + chemin sûr (confinement, symlinks) depuis
  l'identité film ou épisode. Les adaptateurs gardent leurs routes et leurs
  appels bibliothèque ; la décision 39 reste entière.
- **Plan.** Une fonction : (média mesuré ou conteneur-seul, audio demandé,
  hauteur demandée, transcode forcé, capacités connues sans sonde) → plan de
  diffusion {mode, piste audio + index, raison, tone map, hauteur}. `/info`
  l'interroge sans sonde ; le `remux` l'interroge après sa sonde si besoin.
  Les paramètres de la requête entrent dans la même fonction : `/info` et le
  flux ne peuvent plus diverger.
- **Exécution.** Le propriétaire unique de `exec.CommandContext` pour la
  lecture : arguments (le package `stream` reste ce qu'il est), stderr borné,
  porte du premier octet, en-têtes, copie, kill sur erreur de copie, `Wait`,
  journaux. Le contenu de `converted_stream.go` y déménage.
- **Registre de sessions.** Tout flux converti s'y enregistre : processus,
  heure de début, identité, mode. Il applique un plafond aux flux convertis
  hors budget transcode - valeur initiale proposée : 4 simultanés, ajustable à
  l'implémentation avec justification (ménage d'une seule maison, largement au-
  dessus de l'usage réel, assez bas pour arrêter un emballement). Il expose le
  kill de masse au shutdown : `main.go` le consulte avant toute sortie, le
  drain gracieux du HTTP reste la première ligne, le kill des enfants suivis
  est le filet qui rend `os.Exit` inoffensif. `Process.Kill` sur les commandes
  suivies suffit sur les trois plateformes, sans Job Object et sans CGO.
- **Workload.** `BeginInteractive` reste tenu pendant tout le flux converti :
  un film en lecture passe avant les previews, c'est la politique écrite. La
  préemption retryable des previews est préservée. Un raffinement futur
  (lecture en pause n'empêchant plus les previews) est hors périmètre.

**4. Matrice matérielle - candidats documentés et faits mesurés**

Recherche en ligne du 10 septembre 2026. Règle inchangée, décisions 58 et 59 :
ce tableau dit ce qu'il faut sonder et concevoir, il n'autorise rien par
lui-même. Toute capacité adoptée doit d'abord avoir été mesurée par les sondes
du moteur sur la machine réelle.

Faits mesurés localement le jour même sur le runtime alors épinglé (build
`6.1.1-essentials_build-www.gyan.dev`, SHA-256 identique à celui enregistré
par la preview 3, `04e130…ad00`). Cette table est la référence historique de
la refonte ; le runtime qualifié a ensuite été remplacé par 8.1.2-4, décision
110 et [`ffmpeg-8.1.2-validation.md`](ffmpeg-8.1.2-validation.md) :

| Constat du binaire épinglé (Windows AMD64) | Valeur |
|---|---|
| `hwaccels` | `cuda`, `dxva2`, `qsv`, `d3d11va` |
| Encodeurs H.264 matériels | `h264_nvenc`, `h264_qsv`, `h264_amf`, `h264_mf` |
| Encodeurs HEVC matériels | `hevc_nvenc`, `hevc_qsv`, `hevc_amf`, `hevc_mf` |
| Encodeurs AV1 matériels | `av1_nvenc`, `av1_qsv`, `av1_amf` |
| Filtres GPU présents | `scale_cuda`, `colorspace_cuda`, `scale_qsv`, `vpp_qsv`, `hwupload_cuda`, `yadif_cuda` |
| Filtres absents de ce build | `tonemap_opencl`, `tonemap_vaapi`, `libplacebo`, `scale_vt`, `vpp_amf`, filtres Vulkan |

Ce dernier point tranchait une question pour la campagne : **sur le runtime
6.1.1 mesuré, aucun tone mapping GPU n'était possible.** La chaîne CPU
`zscale` + `tonemap` (décision 87) reste la seule pour HDR→SDR. Les chemins GPU existent dans FFmpeg
(`tonemap_opencl` avec reinhard/hable/mobius, `tonemap_vaapi`, `libplacebo`
sur Vulkan, `scale_vt` via VTPixelTransferSession) mais dans des builds
différents - c'est le lot 4 du plan moteur, hors de ce plan. Noter aussi que
la doc officielle ne donne à `scale_cuda` que l'échelle et le format de
pixels : il n'existe **pas** de `tonemap_cuda`.

Candidats par famille - la colonne « à sonder » marque ce que la sonde devra
trancher sur machine réelle :

| Famille | H.264 | HEVC | HEVC 10-bit | AV1 enc. | Décodeur matériel | Tone mapping GPU |
|---|---|---|---|---|---|---|
| GeForce RTX 50 (9ᵉ gén NVENC) | nvenc | nvenc | oui (matrice NVIDIA) | oui | NVDEC : AV1 8/10, HEVC 8/10/12 ; 4:2:2 aussi | aucun chemin CUDA documenté |
| GeForce RTX 40 (8ᵉ) | nvenc | nvenc | oui | oui | AV1 8/10, HEVC 8/10/12 | idem |
| GeForce RTX 30 (7ᵉ) | nvenc | nvenc | oui | **non** | AV1 8/10, HEVC 8/10/12 | idem |
| Radeon 6000/7000/9000 + iGPU Ryzen | amf (Win), vaapi (Linux) | amf (Win), vaapi (Linux) | amf : profil `main` seul documenté - **à sonder** ; vaapi : à sonder | amf dès RDNA2 ; `av1_vaapi` (Linux) | `d3d11va`/`dxva2` (Win), vaapi (Linux) | `vpp_amf`, `tonemap_vaapi` hors runtime épinglé |
| Intel iGPU (UHD/Iris/Arc) | qsv | qsv | oui - `main10` documenté | qsv : Arc et iGPU Xe-LPG (Meteor Lake)+ **à sonder** ; UHD Alder/Raptor : non | qsv (H.264/HEVC/AV1/VP9/VP8/VC-1 documentés), `d3d11va` | `scale_qsv`/`vpp_qsv` présents mais sans tone mapping documenté |
| Apple M1–M2 / M3–M5 | videotoolbox | videotoolbox | oui via `p010le` (pratique documentée, à mesurer sur le build mac) | non - VideoToolbox n'expose pas d'encodeur AV1, M5 décode AV1 seulement | videotoolbox ; AV1 décode matériel dès M3 | `scale_vt` (exemple HDR→SDR documenté) - présent seulement dans un FFmpeg récent ; à vérifier contre le build mac épinglé |

Sources : matrice officielle NVENC/NVDEC de NVIDIA - générations 9ᵉ/8ᵉ/7ᵉ,
**12 sessions NVENC concurrentes déclarées** pour toute la ligne GeForce
(valeur dépendante du pilote dans l'histoire : 3, puis 5 ; le plafond interne
de Theia reste autoritaire jusqu'à mesure contraire) ; documentation officielle
FFmpeg codecs (`hevc_qsv` profil `main10`, `av1_qsv` sous libvpl, décodeurs
QSV, encodeurs VAAPI et option `low_power`) et filtres ; wiki officiel AMF
(paramètres et profils, HEVC limité à `main`) ; annonces Apple M5/M5 Pro/M5
Max de mars 2026 et Mac Studio d'août 2026 (Media Engine : encodage
H.264/HEVC/ProRes, décodage AV1 ; deux moteurs d'encodage sur M5 Max) ; page
Intel « Video Codecs Supported by Intel Arc GPUs ».

[https://developer.nvidia.com/video-encode-and-decode-gpu-support-matrix-new](https://developer.nvidia.com/video-encode-and-decode-gpu-support-matrix-new)
· [https://www.ffmpeg.org/ffmpeg-codecs.html](https://www.ffmpeg.org/ffmpeg-codecs.html)
· [https://www.ffmpeg.org/ffmpeg-filters.html](https://www.ffmpeg.org/ffmpeg-filters.html)
· [https://github.com/GPUOpen-LibrariesAndSDKs/AMF/wiki/AMF%20Encoder%20Settings%20and%20Tuning%20in%20FFmpeg](https://github.com/GPUOpen-LibrariesAndSDKs/AMF/wiki/AMF%20Encoder%20Settings%20and%20Tuning%20in%20FFmpeg)
· [https://www.apple.com/newsroom/2026/03/apple-debuts-m5-pro-and-m5-max-to-supercharge-the-most-demanding-pro-workflows/](https://www.apple.com/newsroom/2026/03/apple-debuts-m5-pro-and-m5-max-to-supercharge-the-most-demanding-pro-workflows/)
· [https://www.intel.com/content/www/us/en/support/articles/000098345/graphics.html](https://www.intel.com/content/www/us/en/support/articles/000098345/graphics.html)

Pipelines candidats à prototyper - l'avertissement du lot 4 du plan moteur
s'applique : une chaîne qui décode, traite puis encode en GPU ne gagne que si
ses copies GPU↔CPU restent mesurées comme acceptables.

- **Chaîne GPU complète pour source SDR** : QSV (`qsv` → `scale_qsv` →
  `h264_qsv`/`hevc_qsv`), CUDA (cuvid → `scale_cuda` → `h264_nvenc`),
  VideoToolbox (→ `scale_vt` si le build le porte). Pour source HDR, repli CPU
  obligatoire dans le runtime 6.1.1 de cette référence.
- **Chaînes hybrides** : décodage matériel (`d3d11va`, cuda, qsv,
  videotoolbox) → `hwdownload` → chaîne CPU `zscale` → encodeur matériel.
  Candidat pertinent quand le décodage CPU d'un 4K HEVC 10-bit monopolise le
  CPU pendant que l'encodeur matériel tourne.
- **Encodeurs H.264 comparés sur machine réelle** : `h264_nvenc`,
  `h264_qsv`, `h264_amf`, `h264_mf`, `h264_vaapi`, `h264_videotoolbox`,
  `libx264`. Le `Best()` actuel ne sonde que la disponibilité, pas la qualité
  ni la vitesse ; `decoders.go` a déjà le patron de benchmark à suivre.
- ARM64 Windows sous émulation x64 : NVENC, AMF et QSV non vérifiés ; la sonde
  décide, aucun chemin présumé.

**5. Tranches**

Chaque tranche livre une petite marche testée, dans l'ordre, sans regrouper.
L'état réel de chaque tranche est tenu à jour dans la section « État
d'avancement » en tête de ce document.

**Tranche 1 - Le filet avant la main.** Tests HTTP dorés qui épinglent le
contrat observable d'aujourd'hui, écrits contre le code actuel et verts avant
tout déménagement : corps de `/info` sur média mesuré et non mesuré, direct
play avec Range, en-têtes et démarrage d'un flux converti, codes d'erreur
atteignables en test (400, 415, 502, 503), borne arrière de `/seek`, rebasage
des sous-titres, équivalence des routes legacy. Si le harnais actuel ne permet
pas d'épingler un comportement, la tranche commence par l'étendre. Acceptation
: suite verte, aucune modification de production.

**Tranche 2 - Le plan unique.** Extraire le plan de lecture ; `/info` et
`remux`, côté film et côté épisode, l'appellent ; les adaptateurs gardent les
routes. Acceptation : suite dorée verte ; comparaison octet pour octet des
réponses `/info` et `info` avant/après sur un banc synthétique de 250 films,
méthode de la preview 3.

**Tranche 3 - Exécution et cycle de vie.** Déménager l'exécution dans
`internal/playback` ; registre de sessions et plafond des flux convertis ;
câbler le kill de masse dans `main.go`. Acceptation : après shutdown forcé
pendant trois flux vivants, aucun processus FFmpeg orphelin dans la liste des
processus ; refus au plafond testé ; les huit scénarios de lecture réelle
restent verts ; les six compilations croisées `CGO_ENABLED=0` passent.

**Tranche 4 - Adaptateurs minces.** Les handlers film, épisode et legacy
deviennent des adaptateurs sur le service ; les blocs dupliqués sont supprimés
et comptés. Aucune route ni table ne change ; le `/info` legacy garde son
corps exact, gelé. Acceptation : suite dorée verte ; la réduction de lignes est
constatée comme conséquence, pas visée comme but ; `go vet` et `gofmt` nets.

**Tranche 5 - Solidité résiduelle.** Extraction de sous-titres embarqués
flottante ou bornée au lieu du `cmd.Output()` entier ; plafond du limiter
dérivé une fois de la sonde de capacités ; décision sur le `Flush` après
premier octet prise sur un A/B mesuré dans la suite de lecture, adopté
seulement s'il aide. Acceptation : une grosse piste embarquée reste bornée en
mémoire ; suites vertes.

**Tranche 6 - Cibles et pipelines matériels mesurés** (dépend des tranches
1–3 ; indépendante de 4 et 5). Étendre la sonde aux encodeurs HEVC et AV1 de
chaque plateforme et au décodage matériel, sur le patron de `decoders.go` :
clip synthétique, chronométrage, seuil de victoire. Construire et mesurer les
chaînes candidates de la §4 : GPU complet pour source SDR, hybride décodage
matériel + chaîne CPU + encode matériel, et comparaison des encodeurs H.264
disponibles. Publier les mesures par famille de matériel dans le rapport,
sans changer le contrat : le codec cible reste H.264 SDR, l'escalade
navigateur reste ce qu'elle est. Acceptation : sur chaque machine disponible,
une source SDR 4K HEVC passe par la chaîne mesurée la plus rapide sans
régression de qualité vérifiée (comparaison d'images clés rastérisées), un
film difficile en entier, et les huit scénarios de lecture restent verts ; les
machines sans matériel adapté conservent exactement le chemin CPU
d'aujourd'hui.

**6. Ordre de livraison et critères de sortie**

| Porte | Critère |
|---|---|
| Chaque tranche | Suite Go complète et `go vet` verts ; suite dorée verte ; aucun changement de contrat constaté |
| Fin de chantier | Hash agrégé des 62 fichiers sous `web/src` et `web/tests` identique avant/après, méthode de la preview 3 ; huit scénarios de lecture réelle verts ; six cibles `CGO_ENABLED=0` compilées |
| Matériel (tranche 6) | Sondes multi-codec et décodeur matériel mesurées sur machine réelle ; aucune capacité adoptée sans mesure ; contrat inchangé ; machines sans matériel adapté servies par le chemin CPU d'aujourd'hui, octet pour octet |
| Lecture réelle | Sur le port 8395 avec la bibliothèque réelle : un film difficile en entier, au moins 30 minutes, seeks proche et lointain, changement de piste audio et de sous-titres, pause et reprise ; redémarrage pendant la lecture puis reprise du profil |
| Rapport | Ce qui a été vérifié et ce qui ne l'a pas été, dit séparément, comme la preview 3 |

**7. Hors périmètre - validation séparée**

Tout changement de réponse ou de route, y compris fermer le trou de la
décision 59 sur le `/info` legacy et l'angle mort conteneur-seule ; le
découpage de `Player.svelte` (lot 2 du plan moteur, chantier frontend
distinct) ; **les cibles de diffusion HEVC et AV1** (changement produit et de
contrat ; l'escalade `?video=transcode` et le module de compatibilité navigateur
déjà en préversion restent arbitres) ; **le tone mapping GPU**
(`tonemap_opencl`, `tonemap_vaapi`, `libplacebo`, `scale_vt`) et tout
renouvellement du runtime FFmpeg - lot 4 du plan moteur ; le 4:2:2 ; HLS,
ffprobe, build FFmpeg maison, médias préconvertis ; raffinement du workload
pour la lecture en pause ; empreinte du binaire et PGO (lot 6 du plan moteur).

**8. Points d'entrée pour exécuter le plan**

| Domaine | Sources à relire au démarrage de chaque tranche |
|---|---|
| Contrat produit | `CLAUDE.md`, `docs/spec-fondatrice.md`, `docs/DECISIONS.md` (16, 23, 39, 58, 59, 68, 87, 90, 92, 101, 102), `docs/v3.md`, `plan-modernisation-moteur.md` |
| Handlers de lecture | `internal/api/stream.go`, `movie_file_stream.go`, `episode_stream.go`, `converted_stream.go`, `seek.go`, `subtitles.go`, `media.go`, `episode_media.go`, `transcode.go` |
| Décision et arguments | `internal/stream/stream.go` et sa suite de tests |
| FFmpeg et workload | `internal/ffmpeg/ffmpeg.go`, `capabilities.go`, `decoders.go`, `seek.go`, `internal/workload/coordinator.go`, `internal/boundedio/tail.go`, `internal/preview/preview.go` |
| Cycle de vie | `cmd/theia/main.go` (shutdown et chemins de sortie), `internal/activity/activity.go` |
| Réseau | `internal/remoteaccess/http.go` (allowlist distante, règles LAN) |
| Matériel | `internal/ffmpeg/capabilities.go`, `decoders.go` (patron de bench), le build épinglé lui-même (`-hwaccels`, `-encoders`, `-filters` à re-exécuter par plateforme) |
| Filet | `internal/api/movie_file_stream_test.go`, `web/tests/playback.spec.js`, `web/tests/unit/` |

**9. Décisions à consigner pendant l'implémentation**

`DECISIONS.md` est append-only : ces entrées sont proposées au moment où le
code les rend vraies, chacune avec sa raison et le test qui la garde - (a) le
plan de lecture a une source unique ; (b) les flux convertis sont enregistrés
et tués au shutdown ; (c) le plafond des flux convertis et la forme de son
refus ; (d) la politique workload documentée comme délibérée ; (e)
l'extraction de sous-titres bornée ; (f) la sonde multi-codec et le décodage
matériel mesurés par plateforme ; (g) aucune capacité matérielle adoptée sur
la foi d'une documentation seule - la décision 58 est rappelée, pas
supersédée. Aucune ne supersede une décision existante.
