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

**Statut : implémenté sur le contrat backend
[`8518bab`](https://github.com/Benitoow/theia-media/commit/8518bab69a84a0f1a5073a16694e4efd52b0a02e)
(PR #4), vérifié contre le vrai serveur et la vraie bibliothèque. **Seul jalon
que le mainteneur n'a pas encore validé** : il reste à ouvrir un vrai film
décodable multi-pistes, les 274 fichiers actuels étant remplis de zéros et ne
prouvant que le chemin `error`.**

Livré : `web/src/lib/components/FileChoice.svelte`, consommé par
`web/src/routes/film/[id]/+page.svelte`, plus les routes `file_id` dans
`Player.svelte` et la table de codes `player.codes` dans les deux catalogues.
Le raisonnement, les mesures D-pad et les limites sont dans la décision 47.

Trois points qu'un agent suivant ne doit pas défaire sans lire la décision 47 :

- le sélecteur est **au-dessus** du synopsis, pas en bas de fiche ;
- les options partagent une largeur plafonnée à `26rem` ;
- la section gère elle-même Haut/Bas ; les bords ne sont pas consommés.

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

**Statut : implémenté, vérifié à l'écran et confirmé par le mainteneur
(décisions 48 et 50).**

Livré : `web/src/routes/profils/+page.svelte`, `ProfileMark.svelte`,
`profiles.svelte.js`, l'entrée de nav et le `?profile=` sur les requêtes.
La décision 50 note le seul écart assumé par rapport à la décision 48 : un
profil unique est adopté en silence, le sélecteur n'apparaissant que lorsqu'il y
a un vrai choix. Ne pas recycler l'ancien écran `/profils`.

Le profil actif vit en `localStorage`, comme la langue, et voyage en
`?profile={id}` sur les routes de catalogue et de progression. Un id inconnu
renvoie `profile_not_found` : c'est le cas à traiter quand une télévision garde
un profil supprimé ailleurs.

Les références du mainteneur donnent la **disposition, pas le style**. Le style
reste celui du design system : fond `--ink`, registre display pour le titre,
label tracké pour les métadonnées, un seul accent, cibles de 52 à 56 px.

#### Trois surfaces

**1. Le sélecteur — écran plein, nav supprimée pour cette route.**
Marque en haut à gauche, titre en question au registre display, une rangée
horizontale de cartes (image carrée, nom centré dessous en registre UI), puis,
nettement détaché, un unique bouton contourné vers la gestion. Beaucoup de vide :
c'est du chrome, pas la grille (§6 du design system ne s'applique pas ici, mais
son esprit de densité non plus).

Le premier focus D-pad est une carte de profil, jamais la nav ni le bouton de
gestion : c'est la question que l'écran pose. La rangée suit la règle §9 des
listes d'options — largeur partagée, axe géré, bords non consommés.

**2. La fiche de profil — deux panneaux empilés.**
Identité en haut : image circulaire cerclée, nom, action d'édition discrète mais
jamais réservée au survol. Puis une liste libellé-gauche / valeur-droite séparée
par des filets : créé le, films commencés, films terminés, dernière lecture.
Aucune ligne email, rôle, statut ou abonnement — elles n'existent pas ici.
En pied, isolée et pleine largeur, la suppression du profil, en `--error`, avec
confirmation. Aucun bouton de déconnexion : il n'y a pas de session.

**3. L'entrée dans la nav — un raccourci, pas un menu.**
L'avatar actif dans la nav **ouvre le sélecteur**. Pas de liste déroulante :
la décision 35 avait mesuré pourquoi (illisible à trois mètres, débordement au
plancher de 320 px, premier focus D-pad détourné vers la navigation).

#### Démarrage

Tant qu'aucun profil n'est choisi dans ce navigateur, le sélecteur s'affiche à
l'arrivée. Le choix vit en `localStorage`, comme la langue : une télévision et
un portable n'imposent pas leur profil l'un à l'autre. Un retour n'est proposé
que si un profil est déjà actif — arriver ici parce que l'application a besoin
d'une réponse ne doit pas offrir de partir sans la donner.

#### États obligatoires à maquetter et tester au D-pad

Aucun profil, un seul, la limite haute retenue par M2-BE, nom très long, image
absente, image en cours d'envoi, envoi refusé, création, renommage, suppression
confirmée et annulée, profil actif supprimé depuis un autre appareil, et le
comportement quand le serveur répond une erreur pendant la sélection.

L'image d'avatar est fournie par l'utilisateur : aucune illustration ne peut
entrer dans le dépôt, et les artworks des références sont exclus (dépôt public
GPL-3.0). Un profil sans image retombe sur une marque CSS, jamais sur une icône
d'image cassée.

### M3-FE — Séries

**Statut : implémenté, vérifié à l'écran et confirmé par le mainteneur
(décision 52).**

Livré : `/series`, `/serie/[id]`, `/episode/[id]`, `EpisodeRow.svelte`, et deux
rangées séries ajoutées après les rangées films sur l'accueil.

Le sélecteur de fichiers et le lecteur sont **ceux de M1**, pas des copies : ils
prennent désormais la ressource sur laquelle ils agissent (`basePath`,
`streamBase`, `progressPath`). Un épisode est une autre ressource, pas une autre
interaction. `PosterCard` a gagné une image et un titre optionnels pour qu'un
épisode l'utilise sans mentir dans ses données.

Trois points qu'un agent suivant ne doit pas défaire sans lire la décision 52 :

- un item combiné s'affiche **une fois**, avec ses deux titres et une seule
  reprise ; chaque membre garde son synopsis ;
- `next_has_gap` s'affiche et ne bloque rien ; `S00` ne mène nulle part ;
- les saisons sont des options sur la fiche, pas des pages.

Le corpus de validation a été généré : *Severance* avec `S01E01` en deux
encodages, un `S01E02E03` combiné, un `S01E05` créant un trou, et un `S00E01`.
Le corpus **utilisateur** ne contient toujours aucune série ; c'est la validation
qui reste.

### M4-FE — Accès distant

**Statut : implémenté, vérifié sur le LAN et confirmé par le mainteneur en
mode distant réel (décision 53).**

Livré : `RemoteAccess.svelte` en section de Réglages, `ProvisionDialog.svelte`,
`remote.svelte.js` pour la détection de contexte, la navigation distante et le
saut de la requête d'onboarding hors LAN.

Ce qui a été vérifié ici : activation sur un port UDP libre, `state: running`,
`reachability: unverified` présenté comme un fait et non comme une panne,
création d'appareil avec QR et configuration, adresse `10.77.0.2`,
`AllowedIPs = 10.77.0.1/32` dans le client, révocation, désactivation, et les
refus `invalid_remote_listen_port` / `invalid_remote_endpoint` traduits.

**La garantie de la décision 45 a été mesurée dans le navigateur** : après
création, `PrivateKey` n'apparaît ni dans `localStorage`, ni dans
`sessionStorage`, ni dans IndexedDB, ni dans l'URL, ni dans le document. Fermer
sans conserver demande confirmation, puis efface tout.

Ce qui **n'a pas** été vérifié : le mode distant lui-même. La navigation sans
réglages et le saut d'onboarding sont implémentés selon le contrat, mais ils ne
peuvent être observés que depuis une vraie session derrière WireGuard.

### M5-FE — Logo et identité de navigation

**Statut : implémenté (décision 54).** Quatre pistes ont été dessinées, rendues
à 16, 28 et 96 px puis en verrou de navigation, et soumises au mainteneur avant
la première ligne de code, comme le jalon l'exigeait.

Retenu : **le mot et le filet**. La marque réduite est l'initiale posée sur le
même filet — un recadrage du verrou, pas une seconde marque à côté. Elle sert de
favicon et, rendue depuis le même SVG, d'icône d'application. `icon-512.png` et
`theia-wordmark.webp` ont quitté le binaire.

Le trait est dessiné en chemins et non en `<text>` : un favicon est chargé comme
une image, donc sans police web. Le fichier est vérifié en le décodant dans un
`Image()`, pas en constatant qu'il existe.

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
