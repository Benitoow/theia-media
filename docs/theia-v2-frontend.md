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

**Statut : prêt à démarrer dès que le commit M1-BE indiqué dans le journal
backend est fusionné sur `main`.**

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

**Statut : attend M3-BE.**

Écrans série, saison et épisode, reprise, épisode suivant et intégration aux
rangées d'accueil. La composition exacte sera décidée lorsque le contrat backend
et les cas réels de bibliothèque existeront.

### M4-FE — Accès distant

**Statut : bloqué avec M4-BE.**

Onboarding, état de connexion, erreurs et récupération d'accès dépendront du
modèle de sécurité choisi. Aucun faux écran de connexion avant cette décision.

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
