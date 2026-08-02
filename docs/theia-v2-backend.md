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

**Statut : découverte M3.0 terminée, verdict `PARTIAL`. Production non
commencée.**

Le [rapport de découverte](theia-v2-m3-discovery.md) contient l'inventaire réel,
le spike de parsing, la cartographie du backend, le schéma et les routes
proposés, ainsi que les décisions encore ouvertes. Le corpus actif compte 283
vidéos mais aucun motif d'épisode : zéro faux positif a été mesuré, aucun import
positif réel ne l'a été. C'est pourquoi il n'existe encore ni migration, ni
endpoint, ni contrat frontend M3.

Le prochain lot recommandé est M3-A — catalogue local additif — puis M3-B —
TMDB TV et lecture. M3-C — progression, épisode suivant et accueil — attend le
contrat de profils M2 afin de ne pas inventer une seconde fois le propriétaire
de la progression.

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
| M3-BE | Découverte `PARTIAL` ; production non commencée | [rapport M3.0](theia-v2-m3-discovery.md) | Aucun contrat ; attend M3-A/M3-B vérifiés |
| M4-BE | Bloqué par la sécurité | — | — |
| M5-BE | Sans backend | — | Aucun |
| M6-BE | Différé | — | — |
