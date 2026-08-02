# THEIA — Piste backend V2

> File de travail backend de la V2. À lire après `spec-fondatrice.md`,
> `DECISIONS.md` et `theia-v2-roadmap.md`. Le roadmap décide du produit ; ce
> document décrit ce que le serveur doit livrer au frontend.

## 1. Règle de livraison

Le backend avance avec Codex, un jalon à la fois. Un jalon backend n'est prêt
pour le frontend qu'après fusion sur `main`, tests automatisés et vérification
sur une copie de la vraie bibliothèque.

Chaque handoff doit remplacer les mentions « à définir » de son jalon par :

- le hash du commit fusionné et le statut réel ;
- les migrations et leur effet sur les données existantes ;
- les routes, paramètres, payloads JSON et codes d'erreur exacts ;
- au moins un exemple de réponse réaliste réutilisable comme fixture frontend ;
- les invariants de sécurité et les limites connues ;
- les commandes de validation et les observations faites sur la vraie
  bibliothèque.

Une réponse imaginée n'est pas un contrat. Toute rupture ultérieure met à jour
ce fichier, `theia-v2-frontend.md` et `DECISIONS.md` dans le même commit.

## 2. État des jalons backend

### M1-BE — Plusieurs fichiers par film

**Statut : implémenté et vérifié dans
[`8518bab`](https://github.com/Benitoow/theia-media/commit/8518bab69a84a0f1a5073a16694e4efd52b0a02e),
publié par la [PR #4](https://github.com/Benitoow/theia-media/pull/4). M1-FE
part de ce commit lorsqu'il est présent sur `main`.**

Le catalogue expose un film, pas une ligne par fichier. Le serveur ne choisit
jamais une « meilleure qualité » : la fiche renvoie les fichiers, le frontend
présente le choix, puis transmet leurs identifiants.

#### Migration et invariants de données

`0006_movie_files.sql` ajoute :

- `movie_files`, source de vérité des chemins, tailles, dates, état
  d'inspection et caractéristiques vidéo ;
- `movie_file_audio_tracks`, pistes audio mesurées, avec un identifiant SQLite
  stable et l'index absolu du stream ffmpeg ;
- une clé étrangère avec suppression en cascade, un chemin unique et un index
  partiel garantissant exactement un fichier principal au maximum par film.

La migration copie chaque ancienne ligne `movies` dans `movie_files` sans
changer son identifiant de film, ses métadonnées ni sa progression. Les colonnes
de fichier historiques restent temporairement dans `movies` et reflètent le
fichier principal : le frontend actuel et les routes v1 continuent donc de
fonctionner jusqu'à M1-FE. Le chemin absolu n'est plus sérialisé dans l'API.

Au premier démarrage puis après chaque scan, une consolidation idempotente :

1. garde comme film canonique le plus ancien `movies.id` ;
2. déplace dessous tous les `movie_files` prouvés équivalents ;
3. conserve les métadonnées reconnues les plus récentes ;
4. conserve la progression au `watched_at` le plus récent ;
5. préfère un titre parsé avec année à une variante sans année ;
6. promeut un nouveau fichier principal si l'ancien a disparu.

#### Règle d'association réellement codée

L'ordre compte :

1. un chemin déjà connu rafraîchit le même `movie_file.id` ;
2. un seul fichier non revu dans le scan, avec la même taille et le même
   `modified_at`, est traité comme un déplacement/renommage et garde ses IDs ;
   si le titre parsé change, les anciennes données TMDB sont effacées avant une
   nouvelle recherche au lieu de rester visibles sous le mauvais film ;
3. un titre normalisé par le parser existant avec la même année non nulle rejoint
   le même film, sauf si deux identifiants TMDB non nuls se contredisent ;
4. sans année, seul le même nom de base hors casse et extension est regroupé ;
5. après enrichissement, le même `tmdb_id` constitue la preuve finale, ce qui
   couvre les titres localisés et les noms sans année (`1917`, `WALL-E`, etc.).

Une collision `titre + année` déjà associée à deux TMDB différents reste deux
films. Une ressemblance de mots, de taille ou de résolution ne suffit jamais.

#### Contrat JSON de la fiche

`GET /api/library/movies/{movie_id}` ajoute `files` uniquement sur la fiche.
Les listes et l'accueil ne chargent pas ces données, pour ne pas multiplier le
poids de 248 cartes. Chaque fichier possède :

- `id`, `file_name`, `size_bytes`, `modified_at`, `extension`, `is_primary` ;
- `media.status` : `pending`, `ok` ou `error` ;
- après mesure : `container`, `duration_seconds`, `video`, `audio_tracks` et
  `inspected_at` ;
- aucune propriété `path`.

Fixture capturée sur le vrai serveur avec deux petits médias générés par le
ffmpeg épinglé du projet :

```json
{
  "id": 6029,
  "file_name": "M1 Validation Film 2026 360p.mp4",
  "size_bytes": 31358,
  "title": "M1 Validation Film",
  "year": 2026,
  "metadata": { "status": "pending" },
  "progress": {
    "position_seconds": 0,
    "duration_seconds": 3,
    "finished": false
  },
  "files": [
    {
      "id": 275,
      "file_name": "M1 Validation Film 2026 360p.mp4",
      "size_bytes": 31358,
      "extension": "mp4",
      "is_primary": true,
      "media": {
        "status": "ok",
        "container": "mp4",
        "duration_seconds": 3,
        "video": {
          "stream_index": 0,
          "codec": "h264",
          "width": 640,
          "height": 360
        },
        "audio_tracks": [
          {
            "id": 1,
            "stream_index": 1,
            "codec": "aac",
            "language": "eng",
            "channels": "mono",
            "is_default": true
          }
        ]
      }
    },
    {
      "id": 276,
      "file_name": "M1 Validation Film 2026 720p.mkv",
      "size_bytes": 60226,
      "extension": "mkv",
      "is_primary": false,
      "media": {
        "status": "ok",
        "container": "matroska",
        "duration_seconds": 3.02,
        "video": {
          "stream_index": 0,
          "codec": "h264",
          "width": 1280,
          "height": 720
        },
        "audio_tracks": [
          {
            "id": 2,
            "stream_index": 1,
            "codec": "aac",
            "language": "eng",
            "title": "English stereo",
            "channels": "mono",
            "is_default": true
          },
          {
            "id": 3,
            "stream_index": 2,
            "codec": "aac",
            "language": "fra",
            "title": "Français stéréo",
            "channels": "mono",
            "is_default": false
          }
        ]
      }
    }
  ]
}
```

Les dates existantes restent dans la vraie réponse ; elles sont retirées ici
pour garder la fixture lisible. Les nombres dépendent naturellement de la base,
pas le nom des champs.

#### Inspection média

`POST /api/library/movies/{movie_id}/files/{file_id}/inspect` lance une mesure
explicite et renvoie le fichier mis à jour. C'est la seule action de fiche qui
peut préparer/télécharger ffmpeg. Un `GET` de fiche ou de stream info ne le fait
jamais. Si taille ou date du fichier change lors d'un scan, codec, résolution et
pistes mis en cache repassent à `pending` avant d'être exposés.

#### Routes de lecture M1

| Méthode et route | Rôle |
|---|---|
| `GET /api/stream/{movie_id}/files/{file_id}/info` | Décision direct/remux/refus pour ce fichier |
| `GET /api/stream/{movie_id}/files/{file_id}` | Lecture directe avec requêtes Range |
| `GET /api/stream/{movie_id}/files/{file_id}/remux` | Remux du fichier sélectionné |

Paramètres :

- `audio={audio_track_id}` sur `info` et `remux` sélectionne une piste mesurée ;
- `t={secondes}` sur `remux` conserve le seek/reprise corrigé ;
- une sélection audio force le remux même pour un MP4 directement lisible,
  parce qu'envoyer le fichier entier ne garantit pas la piste choisie.

`info` renvoie `movie_id`, `file_id`, `audio_track_id` lorsqu'il est demandé,
`mode`, `reason_code`, `container`, `media_status`, disponibilité ffmpeg,
durée et progression. Les `reason_code` possibles sont `container_direct`,
`container_remux`, `direct_play`, `audio_track_selected`, `audio_transcode` et
`video_transcode_required`.

Les routes v1 `/api/stream/{movie_id}`, `/info` et `/remux` restent actives et
lisent le fichier principal. M1-FE doit utiliser les routes avec `file_id`.

#### Erreurs stables à traduire côté frontend

| HTTP | `error` | Cas |
|---|---|---|
| 400 | `invalid_movie_id` | ID film invalide |
| 400 | `invalid_file_id` | ID fichier invalide |
| 400 | `invalid_audio_track_id` | ID piste invalide |
| 400 | `audio_selection_requires_remux` | piste passée à la route directe |
| 403 | `file_outside_library` | garde-fou de chemin |
| 404 | `movie_not_found` | film absent |
| 404 | `file_not_found` | fichier absent ou appartenant à un autre film |
| 404 | `audio_track_not_found` | piste absente ou appartenant à un autre fichier |
| 404 | `media_file_unavailable` | fichier disparu au moment de l'ouverture |
| 409 | `media_not_inspected` | piste demandée avant inspection |
| 415 | `media_unreadable` | ffmpeg ne reconnaît pas le média |
| 415 | `video_transcode_required` | codec vidéo hors M1, réservé à M6 |
| 500 | `movie_unavailable`, `file_unavailable`, `audio_track_unavailable` | lecture SQLite impossible |
| 500 | `media_file_unreadable`, `media_inspection_not_saved`, `stream_start_failed` | échec local après résolution |
| 501 | `ffmpeg_unsupported` | aucune build épinglée pour la plateforme |
| 503 | `ffmpeg_unavailable` | préparation locale de ffmpeg impossible |

#### Sécurité, compatibilité et limites

- Le navigateur ne transmet jamais un chemin ni une expression ffmpeg : les
  trois IDs sont vérifiés dans leur relation film → fichier → piste.
- Le chemin résolu doit rester sous une racine configurée après résolution des
  liens symboliques/jonctions.
- Le tracker d'activité encadre toujours direct play et remux ; l'updater ne
  redémarre pas au milieu d'un film.
- Aucune suppression de fichier absent n'est appliquée si la marche du disque
  ou une seule écriture SQLite du scan a signalé un problème.
- Une panne de téléchargement ou d'exécution de ffmpeg renvoie
  `ffmpeg_unavailable` et ne marque pas le fichier comme média illisible.
- La progression reste au niveau du film et survit au changement de fichier.
- `scan.added`, `updated` et `removed` comptent des fichiers ; `merged` compte
  les anciennes fiches supprimées par consolidation ; `stats.movies` compte
  les films.
- Une inspection `error` est retentable avec le même `POST`.
- Le transcodage vidéo, la réduction 2160p → 720p et le choix automatique par
  débit restent explicitement hors M1, réservés à M6.

#### Vérification effectuée

- Migration et serveur exécutés sur une sauvegarde SQLite de la bibliothèque
  locale, jamais sur l'original ; hash et date de l'original identiques après
  le test.
- Avant : 274 lignes `movies`. Après : 248 films, 274 fichiers, 25 films
  multi-fichiers, 26 doublons absorbés, zéro fichier orphelin et exactement un
  principal par film.
- Les 260 métadonnées `ok`, 14 `not_found` et l'unique progression existante ont
  été conservées : après fusion des doublons, 234 `ok` + 14 `not_found` sur 248
  films, progression `60/240` toujours présente.
- Le scan réel a retrouvé 274 fichiers, zéro ajout/suppression et zéro problème
  en 409 ms sur la copie migrée.
- Les six vrais clips jouables du dossier `_Tests` ont été inspectés : MP4/H264/
  AAC direct, MKV/H264/AAC remux copié, MKV/H264/AC3 remux avec audio AAC,
  MPEG-2 refusé, durées 4 à 240 s et résolutions 320×180 à 640×360 correctement
  renvoyées.
- Une requête Range réelle a renvoyé `206`, 100 octets et le bon
  `Content-Range`. Le remux AC3 sélectionné a renvoyé `200`, une seule piste AAC
  et une vidéo H264 copiée.
- Dans le navigateur embarqué, l'accueil v1 a chargé la bibliothèque
  consolidée, la fiche du film multi-fichiers s'est ouverte avec les champs
  historiques du principal, puis le lecteur v1 a lu ce MP4 jusqu'à
  `readyState = 4`, sans erreur console. M1-FE reste non implémenté, par design.
- Les 277 gros fichiers factices de la bibliothèque commencent par des zéros et
  ne sont pas des médias décodables ; ils ont servi au test de migration et de
  regroupement, pas à prétendre valider ffmpeg. Deux clips éphémères valides,
  dont un MKV à deux pistes `eng`/`fra`, ont donc vérifié le choix de piste 2 et
  produit un MP4 avec une seule piste française.
- Les tests de régression couvrent aussi le renommage avec invalidation des
  anciennes métadonnées et l'interdiction de purger après une écriture SQLite
  volontairement rejetée pendant le scan.

### M2-BE — Profils, nouvelle mouture

**Statut : bloqué par les screenshots et croquis du mainteneur.**

Le chantier repart de zéro. Il ne restaure ni l'ancien package, ni l'ancienne
migration, ni `X-Theia-Profile`. Les références visuelles permettront d'abord
de figer les actions réelles — créer, choisir, modifier, supprimer — puis le
modèle et l'API seront conçus pour ces actions. Tant que la décision
zéro-authentification tient, les profils séparent l'expérience et la progression
mais ne deviennent ni comptes ni permissions.

### M3-BE — Séries

**Statut : implémenté et vérifié dans
[`5b2615e`](https://github.com/Benitoow/theia-media/commit/5b2615e77655e41567f339e68de3cf7c8e0a05d7),
livré par la [PR #5](https://github.com/Benitoow/theia-media/pull/5). Le backend
M3 est prêt pour M3-FE ; aucun frontend série n'est inclus dans ce commit.**

M3-A, M3-B et M3-C ont été livrés ensemble après décision du mainteneur. La
progression est volontairement **single-viewer**, comme le produit actuel : M3
ne ressuscite ni l'ancien modèle profils, ni `X-Theia-Profile`. M2 migrera cette
progression lorsqu'un nouveau contrat profils existera.

#### Migration et modèle local

`0007_tv_series.sql` est entièrement additive. Elle crée :

- `series`, identité locale et cache TMDB TV ;
- `seasons`, unique sur `(series_id, season_number)` ;
- `episodes`, membres TMDB uniques par saison et numéro ;
- `episode_items`, unités réellement jouables et reprenables ;
- `episode_item_members`, jointure ordonnée d'un item vers un ou plusieurs
  épisodes ;
- `episode_files`, chemins, générations de scan et mesures ffmpeg ;
- `episode_file_audio_tracks`, pistes mesurées avec IDs SQLite stables.

Un fichier `S01E01E02` produit un item avec `episode_numbers: [1, 2]`, deux
membres et une seule timeline de progression. Deux encodages ne rejoignent le
même item que si leur liste canonique de numéros est strictement identique. La
migration ne lit, ne copie et ne modifie aucune ligne film ; elle est testée
avec deux passes successives de `Migrate`.

La progression se trouve sur `episode_items`. Elle reprend exactement les
règles films : moins de 30 secondes n'est pas mémorisé ; la fin est calculée à
la plus proche des deux bornes, 95 % ou deux minutes restantes ; un reset garde
la durée mesurée.

#### Classification et réconciliation

Le scanner transmet maintenant la racine et le chemin relatif. Le classifieur
reconnaît, avec de vraies frontières :

- `S01E02`, `S1E2` et variantes de casse ;
- `1x02` ;
- `S01E01E02` et `S01E01-E02` ;
- `S00E01`, rangé dans la saison spéciale 0 ;
- un titre/une année avant le marqueur, ou le premier parent qui n'est pas un
  dossier de saison.

`101`, les dates, les numéros absolus d'anime et « Episode 2 » sans saison ne
sont pas devinés. Un marqueur fiable sans titre de série renvoie le problème
stable `episode_series_unknown` et n'est pas transformé en film. Ce problème
n'empêche pas la purge ; toute erreur de marche ou d'écriture SQLite l'empêche.
Le dossier `shorts` n'est plus sauté globalement, car il peut contenir une vraie
série courte.

La réconciliation suit cet ordre : chemin connu, déplacement prouvé par taille
et date, puis association locale titre normalisé + année. Plusieurs candidats
de déplacement sont départagés seulement par un meilleur contexte unique —
même item, saison, puis série. Une égalité reste non fusionnée. Deux identités
TMDB contradictoires bloquent l'association locale ; un même `tmdb_id` non nul
est au contraire la preuve qui consolide des titres localisés. Une bascule
film ↔ épisode retire l'ancien propriétaire dans la même transaction.

Les suppressions retirent dans l'ordre fichiers, items orphelins, épisodes,
saisons puis séries ; un nouveau fichier principal est promu si nécessaire.
Un renommage logique crée le nouvel `episode_item`, mais conserve le
`episode_file.id`, les mesures média et la progression la plus récente lorsque
le déplacement est prouvé.

#### TMDB TV

Le client utilise :

- `GET /3/search/tv`, avec `first_air_date_year` quand il existe ;
- `GET /3/tv/{id}?append_to_response=credits` ;
- `GET /3/tv/{id}/season/{number}` uniquement pour les saisons locales.

Le nom ou `original_name` exact gagne, puis la popularité. Les métadonnées
restent en `fr-FR`, avec 90 jours de cache après succès et 7 jours après absence
ou erreur. Une saison fraîche est relue si un nouvel épisode local apparaît :
le cache surveille série, saison **et épisode**, ce qui évite de laisser une
nouvelle acquisition `pending` pendant trois mois. TMDB ne crée jamais de
saison ou d'épisode absent du disque.

#### Catalogue et reprise

| Méthode et route | Réponse / rôle |
|---|---|
| `GET /api/library/series?limit=&offset=` | Catalogue paginé, `kind: "series"` |
| `GET /api/library/series/{series_id}` | Fiche série et saisons possédées |
| `GET /api/library/series/{series_id}/seasons/{number}` | Items locaux compacts de la saison |
| `GET /api/library/episodes/{episode_id}` | Item complet, membres, fichiers, progression et voisinage |
| `GET /api/library/series/home?limit=` | `continue_watching` + `recent_series` |
| `PUT /api/library/episodes/{episode_id}/progress` | Sauve `position_seconds` et `duration_seconds` |
| `DELETE /api/library/episodes/{episode_id}/progress` | Repart de zéro, réponse 204 |

L'`episode_id` est toujours l'ID de `episode_items`, jamais un ID TMDB. Une
réponse de saison reste compacte : elle expose membres et progression, mais
pas `files`. Le frontend ouvre ensuite la fiche de l'item pour obtenir les
fichiers. Aucun payload ne contient de chemin absolu.

Fixture capturée contre le serveur de validation et TMDB réel ; les mesures
restent `pending` tant que l'inspection explicite n'a pas eu lieu :

```json
{
  "kind": "episode",
  "id": 1,
  "series_id": 1,
  "series_title": "Severance",
  "season_id": 1,
  "season_number": 1,
  "episode_numbers": [1],
  "episode_metadata": [
    {
      "id": 1,
      "episode_number": 1,
      "metadata": {
        "tmdb_id": 1982925,
        "name": "Le bon côté de l'enfer",
        "runtime_minutes": 57,
        "status": "ok"
      }
    }
  ],
  "files": [
    {
      "id": 1,
      "file_name": "S01E01.1080p.mkv",
      "size_bytes": 2415845,
      "extension": "mkv",
      "is_primary": true,
      "media": { "status": "pending", "audio_tracks": [] }
    },
    {
      "id": 2,
      "file_name": "S01E01.720p.mp4",
      "size_bytes": 124512,
      "extension": "mp4",
      "is_primary": false,
      "media": { "status": "pending", "audio_tracks": [] }
    }
  ],
  "progress": { "position_seconds": 0, "finished": false },
  "next_episode_id": 2
}
```

Le prochain item est le prochain possédé localement. `next_has_gap: true`
signale un saut de numéro sans bloquer la lecture. Passer de la fin d'une saison
à l'épisode 1 de la suivante n'est pas un trou. `S00` ne reçoit jamais de
`next_episode_id`, et un item combiné avance depuis son numéro maximal.

`/api/library/stats` ajoute `series` et `episodes`. Le rapport de scan garde ses
compteurs historiques de fichiers et ajoute `movie_files`, `episode_files`,
`series` et `episodes`. `enriched`, `not_found` et `metadata_errors` agrègent
désormais films et séries/saisons.

#### Inspection, fichiers, audio et lecture

| Méthode et route | Rôle |
|---|---|
| `POST /api/library/episodes/{episode_id}/files/{file_id}/inspect` | Mesure conteneur, durée, vidéo et pistes |
| `GET /api/library/episodes/{episode_id}/files/{file_id}/stream/info` | Décide direct/remux/refus sans lancer ffmpeg |
| `GET /api/library/episodes/{episode_id}/files/{file_id}/stream` | Direct play avec HTTP Range |
| `GET /api/library/episodes/{episode_id}/files/{file_id}/stream/remux` | Remux, seek `t` et sélection `audio` |

`audio={audio_track_id}` utilise l'ID stable exposé après inspection, jamais
`stream_index`. Une sélection force le remux même sur MP4. `t={secondes}` garde
le seek corrigé. La durée vient, dans l'ordre, de la mesure du fichier, de la
progression connue, puis de la somme des durées TMDB des membres. Les routes
sont placées sous la ressource épisode : la forme initialement proposée
`/api/stream/episodes/...` chevauche réellement les wildcards films existants
dans `http.ServeMux`.

Le navigateur ne transmet ni chemin ni argument ffmpeg. Le serveur vérifie
`episode_item → episode_file → audio_track`, résout les jonctions/liens, puis
refuse tout fichier hors des racines configurées. Une inspection échouée est
retentable ; seule une vraie erreur média est mise en cache comme `error`.

#### Codes d'erreur stables pour M3-FE

| HTTP | `error` |
|---|---|
| 400 | `invalid_series_id`, `invalid_season_number`, `invalid_episode_id`, `invalid_file_id` |
| 400 | `invalid_audio_track_id`, `invalid_progress_payload`, `audio_selection_requires_remux` |
| 403 | `file_outside_library` |
| 404 | `series_not_found`, `season_not_found`, `episode_not_found`, `file_not_found` |
| 404 | `audio_track_not_found`, `media_file_unavailable` |
| 409 | `media_not_inspected` |
| 415 | `media_unreadable`, `video_transcode_required` |
| 500 | `series_unavailable`, `season_unavailable`, `episode_unavailable`, `file_unavailable` |
| 500 | `audio_track_unavailable`, `progress_not_saved`, `progress_not_reset` |
| 500 | `media_file_unreadable`, `media_inspection_not_saved`, `stream_start_failed` |
| 501 | `ffmpeg_unsupported` |
| 503 | `ffmpeg_unavailable` |

#### Vérification effectuée

- `go test ./... -count=1` et `go vet ./...` passent ;
- six builds passent avec `CGO_ENABLED=0` : Windows, Linux et macOS en
  `amd64`/`arm64` ;
- la bibliothèque réelle a été scannée dans une base et un serveur isolés sur
  8395 : **274 fichiers**, **254 films**, **0 fichier épisode**, **0 série** et
  aucun faux positif ; la base utilisateur n'a pas été ouverte en écriture ;
- un corpus positif décodable séparé contient saison 1, `S00`, un item
  `S01E02E03` et deux encodages de `S01E01` ; il produit 4 fichiers, 3 items et
  1 série ;
- la vraie API TMDB a reconnu *Severance* (`tmdb_id: 95396`) et rempli uniquement
  les saisons 0 et 1 locales ; le scan suivant a fait 0 enrichissement ;
- les deux fichiers ont été inspectés en `1280×720` et `640×360`; le MP4 a
  exposé deux pistes `eng`/`fra` et la sélection française a renvoyé
  `audio_track_selected` ;
- le direct play a renvoyé 206 et exactement 20 octets pour une Range de 20
  octets ; le remux français a renvoyé 200, produit 96 095 octets et a été
  redécodé sans erreur par ffmpeg ;
- un renommage ambigu face à trois fichiers de taille/date identiques a gardé
  `episode_file.id = 3`, déplacé `[2,3]` vers `[4,5]`, signalé le trou, puis
  conservé 60 secondes de progression au retour ;
- aucun corpus **utilisateur** positif n'existe encore. Le positif ci-dessus est
  un corpus de validation généré et réellement lu, pas une série de la
  bibliothèque du mainteneur. Ce manque reste à revalider pendant M3-FE avec
  les premiers fichiers réels.

Hors contrat : sous-titres, transcodage vidéo, ordre DVD/absolu, profils,
accès distant et choix automatique de « meilleure » qualité. Ils ne se sont pas
glissés dans M3 par la fenêtre pendant que la porte était ouverte.

### M4-BE — Accès distant

**Statut : bloqué par une décision de sécurité.**

Le réseau local sans authentification ne peut pas être simplement exposé sur
internet. Aucun tunnel, compte, jeton ou HTTPS ne sera choisi avant une décision
d'architecture explicite sur le modèle de menace et la récupération d'accès.

### M5-BE — Logo et navigation

**Statut : aucun chantier backend.** Ce jalon appartient entièrement à la piste
frontend et à la direction artistique.

### M6-BE — Optimisation matérielle

**Statut : différé.**

Détection des capacités, transcodage vidéo à la volée, choix CPU/iGPU/GPU,
limites de concurrence et observabilité locale. Ce travail ne commence pas
avant stabilisation des jalons précédents et conserve zéro CGO ainsi que les six
cibles de compilation existantes.

## 3. Journal des handoffs backend

| Jalon | Statut | Commit fusionné | Contrat frontend |
|---|---|---|---|
| M1-BE | Implémenté et vérifié | [`8518bab`](https://github.com/Benitoow/theia-media/commit/8518bab69a84a0f1a5073a16694e4efd52b0a02e), [PR #4](https://github.com/Benitoow/theia-media/pull/4) | Contrat ci-dessus ; M1-FE part de ce commit |
| M2-BE | Bloqué par les références | — | — |
| M3-BE | Implémenté et vérifié | [`5b2615e`](https://github.com/Benitoow/theia-media/commit/5b2615e77655e41567f339e68de3cf7c8e0a05d7), [PR #5](https://github.com/Benitoow/theia-media/pull/5) | Contrat ci-dessus ; M3-FE part de ce commit |
| M4-BE | Bloqué par la sécurité | — | — |
| M5-BE | Sans backend | — | Aucun |
| M6-BE | Différé | — | — |
