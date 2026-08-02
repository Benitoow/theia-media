# THEIA — Piste frontend V2

> File de travail frontend destinée à Claude. À lire après `CLAUDE.md`,
> `spec-fondatrice.md`, `DECISIONS.md`, `design-system.md`,
> `theia-v2-roadmap.md` et le handoff correspondant dans
> `theia-v2-backend.md`.

## 1. Règle de reprise

Le frontend d'un jalon ne commence que lorsque son backend est fusionné sur
`main` et marqué « prêt pour le frontend ». Le payload documenté et l'API réelle
font foi. Une ancienne conversation, une ancienne branche ou le code supprimé
des profils ne sont pas des spécifications.

Si le frontend a besoin d'un champ absent, il le signale comme changement de
contrat ; il ne le devine pas et ne reprogramme pas une logique backend dans
Svelte. Toute modification acceptée revient dans les deux pistes et dans
`DECISIONS.md` avant de continuer.

Contraintes communes à tous les écrans :

- navigation D-pad complète et focus visible ;
- texte et cibles lisibles à trois mètres ;
- français et anglais maintenus à parité ;
- aucune interaction importante réservée au survol ;
- vérification dans un vrai navigateur contre le vrai serveur et la vraie
  bibliothèque, pas seulement avec des données factices.

## 2. État des jalons frontend

### M1-FE — Choix du fichier sur la fiche film

**Statut : prêt à démarrer à partir du commit backend
[`8518bab`](https://github.com/Benitoow/theia-media/commit/8518bab69a84a0f1a5073a16694e4efd52b0a02e),
publié par la [PR #4](https://github.com/Benitoow/theia-media/pull/4), dès qu'il
est présent sur `main`.**

Une seule carte représente le film dans le catalogue. Après ouverture de la
fiche, l'utilisateur voit les fichiers réellement renvoyés par l'API et choisit
manuellement celui qu'il veut lire. Le sélecteur n'apparaît ni avant la fiche,
ni uniquement au survol. Le lecteur transmet les identifiants documentés du
fichier et, lorsqu'elle existe, de la piste audio choisie.

À vérifier : états avec un seul fichier, plusieurs fichiers, caractéristiques
partiellement inconnues, fichier disparu et lecture refusée. Les libellés de
qualité ne doivent pas inventer une résolution ou une piste que le backend n'a
pas détectée.

Contrat à consommer, sans le réinventer :

1. charger `GET /api/library/movies/{movie_id}` ; `files` n'existe que sur cette
   fiche, pas dans les cartes de `/films` ou de l'accueil ;
2. afficher `file_name`, taille, `extension` et uniquement les caractéristiques
   dont `media.status == "ok"` ; aucun chemin serveur n'est renvoyé ;
3. pour un fichier `pending`, appeler explicitement
   `POST /api/library/movies/{movie_id}/files/{file_id}/inspect`, avec un état de
   préparation visible ; un résultat `error` reste retentable ;
4. laisser l'utilisateur choisir le fichier, puis éventuellement une entrée de
   `media.audio_tracks` par son `id` ; ne jamais transmettre `stream_index` ;
5. appeler `GET /api/stream/{movie_id}/files/{file_id}/info`, avec
   `?audio={audio_track_id}` lorsqu'une piste a été choisie ;
6. si `mode == "direct"`, lire
   `/api/stream/{movie_id}/files/{file_id}` ; si `mode == "remux"`, lire
   `/api/stream/{movie_id}/files/{file_id}/remux`, en conservant `audio` et `t` ;
   si `mode == "unsupported"`, traduire `reason_code` et ne pas lancer le
   lecteur.

Une piste explicitement choisie force le mode remux, y compris sur un MP4 : la
route directe ne peut pas garantir quelle piste le navigateur activera. Les
routes historiques sans `file_id` restent uniquement le filet de compatibilité
du frontend actuel ; M1-FE ne doit pas construire dessus.

États obligatoires à maqueter et tester au D-pad :

- un seul fichier `pending`, puis `ok` ;
- deux fichiers de résolutions mesurées différentes ;
- plusieurs pistes audio, avec langue, titre, codec et défaut éventuellement
  absents ;
- `media_unreadable`, `media_not_inspected`, `file_not_found`,
  `audio_track_not_found`, `video_transcode_required` et `ffmpeg_unavailable` ;
- progression commune conservée quand l'utilisateur change de fichier.

La fixture complète, les payloads `info`, les codes HTTP et les limites sont
dans la section M1-BE de `theia-v2-backend.md`. Le frontend traduit les codes ;
il n'affiche aucune prose technique du serveur.

### M2-FE — Profils, nouvelle mouture

**Statut : bloqué par les screenshots et croquis du mainteneur.**

Ces références seront le contrat visuel : composition, hiérarchie, transitions,
focus, navigation télécommande et comportement de gestion. Ne pas recycler
l'ancien écran `/profils`, même comme raccourci temporaire. L'implémentation
commence seulement après validation des références et handoff M2-BE.

### M3-FE — Séries

**Statut : prêt à démarrer depuis
[`5b2615e`](https://github.com/Benitoow/theia-media/commit/5b2615e77655e41567f339e68de3cf7c8e0a05d7),
livré par la [PR #5](https://github.com/Benitoow/theia-media/pull/5). Aucun écran
série n'a été codé par le chantier backend.**

Le contrat complet, la fixture et les erreurs se trouvent dans la section
M3-BE de `theia-v2-backend.md`. Le frontend ne repart pas du rapport de spike et
ne devine pas une route à partir d'un ancien message.

Flux à consommer :

1. `GET /api/library/series` construit le catalogue séries ;
2. `GET /api/library/series/{id}` construit la fiche et les saisons locales ;
3. `GET /api/library/series/{id}/seasons/{number}` fournit les items compacts ;
4. `GET /api/library/episodes/{id}` fournit membres, fichiers, progression,
   `next_episode_id` et `next_has_gap` ;
5. un fichier `pending` se mesure uniquement par le `POST .../inspect` explicite ;
6. le lecteur demande `.../stream/info`, puis utilise `.../stream` ou
   `.../stream/remux`, avec les mêmes IDs de piste et règles que M1-FE ;
7. la progression utilise `PUT`/`DELETE /api/library/episodes/{id}/progress` ;
8. `GET /api/library/series/home` fournit la reprise série et les séries
   récentes à composer avec l'accueil films existant.

Un item peut porter `[1, 2]` : il doit être présenté comme un épisode combiné,
pas dupliqué en deux cartes qui lancent le même fichier. `next_has_gap` mérite un
état visible mais ne bloque pas l'action. Les spéciaux sont la saison 0 et ne
doivent jamais être injectés dans l'enchaînement automatique principal.

Le choix de fichier reste manuel sur la fiche, exactement comme M1-FE. Une
piste audio choisie force le remux. Aucun label de résolution, langue ou codec
n'est inventé quand `media.status != "ok"`, et aucun chemin serveur n'existe à
afficher.

États obligatoires à maquetter au D-pad et à trois mètres : catalogue vide,
série sans TMDB, saison 0, item simple, item combiné, trou, dernier épisode,
plusieurs fichiers, inspection `pending/error/ok`, plusieurs pistes, reprise et
erreurs stables du tableau M3-BE. Le corpus utilisateur actuel ne contient
encore aucune série ; utiliser d'abord la fixture backend, puis refaire la
validation sur les premiers vrais fichiers au lieu de transformer ce manque en
fausse preuve visuelle.

### M4-FE — Accès distant

**Statut : prêt à démarrer depuis
[`a547528`](https://github.com/Benitoow/theia-media/commit/a547528ddb0606a3dbe21c44015ced5088c78d2a),
après sa fusion sur `main`. Le backend est complet ; aucun écran M4 n'a été
codé.**

M4 n'ajoute ni compte, ni mot de passe, ni profil. Le LAN reste zéro-login ; un
appareil distant possède une clé WireGuard. Ne dessine donc pas un écran de
connexion façon SaaS : ce serait un mensonge très bien aligné.

#### Détection de contexte

Le layout appelle tôt `GET /api/remote-access/session` :

- `mode: "lan"` conserve toute la navigation actuelle ;
- `mode: "remote"` permet catalogue, fiches et lecteur, mais retire de la
  navigation les réglages, le scan, l'onboarding et l'updater ; le nom du pair
  peut être affiché discrètement comme contexte, jamais comme profil humain.

Depuis le tunnel, `GET /api/remote-access` et `GET /api/settings` renvoient 403.
Ne les lance pas puis ne transforme pas le refus en écran rouge : ne les lance
pas. La home actuelle tolère déjà l'échec de `/api/onboarding` via
`Promise.allSettled`, mais M4-FE doit sauter cette requête en mode distant pour
ne pas demander volontairement une donnée interdite.

#### Écran local Réglages > Accès distant

Uniquement sur le LAN :

1. charger `GET /api/remote-access` ;
2. afficher `disabled`, `running` ou `error`, le port UDP, l'endpoint public et
   `reachability` ;
3. sauvegarder par `PUT` avec les champs réellement modifiés ;
4. expliquer que le routeur redirige **UDP**, jamais TCP 8383, et que CGNAT
   n'est pas résolu par Theia ;
5. créer un appareil par nom seulement quand `state == "running"` ;
6. lister les pairs actifs, leur dernière poignée de main et leurs compteurs,
   puis confirmer la révocation avant `DELETE`.

`unverified` ne signifie pas « erreur » : aucun appareil n'a encore prouvé le
chemin. `confirmed` signifie au moins un handshake observé depuis ce démarrage,
pas une garantie éternelle. Les octets sont des compteurs réseau techniques,
pas une statistique de visionnage.

L'endpoint est saisi comme `hôte:port` sans `http://`. Donner des exemples
distincts : `media.example.net:51820`, `203.0.113.10:51820` et
`[2001:db8::10]:51820`. Le port public peut différer du port d'écoute. Changer
l'endpoint ne met pas à jour les appareils existants : afficher ce fait avant
la sauvegarde et proposer de modifier WireGuard ou de reprovisionner. Changer
le port peut couper momentanément les appareils.

#### Provisioning affiché une fois

Après le 201 de `POST /api/remote-access/peers` :

- ouvrir un dialogue D-pad complet avec le QR, le bouton de copie et le
  téléchargement d'un `.conf` standard ;
- annoncer clairement que le QR et le fichier contiennent une clé privée ;
- garder `client_config` et `qr_svg` uniquement dans l'état mémoire du dialogue,
  jamais dans `localStorage`, IndexedDB, logs, analytics ou URL ;
- vider cet état à la fermeture/navigation ;
- demander confirmation avant de fermer sans copie, car le backend ne les
  renverra pas ;
- si l'utilisateur les perd, proposer « Révoquer et recréer », pas « Afficher
  à nouveau ».

Le QR SVG vient du serveur local et doit rester dans le dialogue. Ne pas le
mettre en cache par un service worker et ne pas en faire une miniature de carte.
Le secret d'une télévision n'est pas un badge décoratif.

#### Erreurs et récupération à traduire

Formulaire : `invalid_remote_access_payload`, `invalid_remote_listen_port`,
`invalid_remote_endpoint`, `invalid_remote_peer_payload`,
`invalid_remote_peer_name`, `invalid_remote_peer_id`,
`remote_peer_limit_reached`, `remote_peer_not_found`,
`remote_access_disabled`, `remote_access_not_ready` et
`remote_access_unavailable`.

État serveur : `remote_config_invalid`, `remote_key_unavailable`,
`remote_listen_failed`, `remote_listener_stopped`,
`remote_peer_reload_failed`, `remote_restore_failed`. Chaque état d'erreur doit
garder l'action **Désactiver** disponible. La récupération documentée est locale
: désactiver, corriger le port/endpoint ou, pour une clé devenue illisible après
copie du data-dir Windows, retirer `remote-access.key`, réactiver puis
reprovisionner.

Les gardes `lan_access_required`, `remote_peer_unknown`,
`remote_host_invalid`, `remote_access_forbidden` et
`remote_origin_forbidden` ont leur contrat dans M4-BE. Une session distante qui
les rencontre doit revenir vers une page de récupération concise, sans proposer
un mot de passe qui n'existe pas.

#### Acceptation visuelle et réelle

- écran lisible à trois mètres, focus toujours visible, clavier et D-pad ;
- aucun QR ou bouton essentiel réservé au survol ;
- états : disabled, enable invalide, running/unverified, running/confirmed,
  aucun pair, 32 pairs, erreur listener, provisioning, fermeture sans copie,
  révocation et session distante ;
- français/anglais ajoutés ensemble et contrôle des 224 clés existantes étendu ;
- test dans un vrai navigateur LAN pour la gestion, puis dans un vrai navigateur
  derrière un client WireGuard pour home, fiches, lecture, progression et nav
  sans réglages ;
- validation avec un vrai endpoint hors du LAN indispensable avant de déclarer
  **V2-M4 produit** terminé. Le backend a prouvé le tunnel en UDP loopback ; il
  n'a pas prétendu posséder un routeur extérieur de poche.

Le payload exact, la fixture, les codes HTTP, les limites et la récupération
sont dans M4-BE. Le rapport de découverte
`theia-v2-m4-discovery.md` explique les alternatives ; il ne remplace pas ce
contrat.

### M5-FE — Logo et identité de navigation

**Statut : backlog frontend, sans dépendance backend.**

Travail de direction artistique à partir de références validées. Il ne doit pas
être glissé dans un autre jalon comme une retouche cosmétique opportuniste.

### M6-FE — Contrôles de qualité et capacités matérielles

**Statut : attend M6-BE.**

Expose uniquement les modes réellement annoncés par le serveur, avec leurs
coûts et indisponibilités. Aucun bouton décoratif pour une qualité que la machine
ne sait pas produire.

## 3. Checklist de démarrage pour Claude

Avant la première ligne de frontend d'un jalon :

1. partir du dernier `main` fusionné ;
2. lire les six documents indiqués en tête de fichier ;
3. vérifier que le tableau de handoff backend contient un commit et un contrat ;
4. lancer le serveur et observer les réponses réelles ;
5. confirmer les références visuelles avec le mainteneur lorsqu'elles sont
   requises ;
6. annoncer tout écart entre documentation et comportement avant de le contourner.
