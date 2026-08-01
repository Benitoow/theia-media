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

**Statut : attend le handoff M1-BE.**

Une seule carte représente le film dans le catalogue. Après ouverture de la
fiche, l'utilisateur voit les fichiers réellement renvoyés par l'API et choisit
manuellement celui qu'il veut lire. Le sélecteur n'apparaît ni avant la fiche,
ni uniquement au survol. Le lecteur transmet les identifiants documentés du
fichier et, lorsqu'elle existe, de la piste audio choisie.

À vérifier : états avec un seul fichier, plusieurs fichiers, caractéristiques
partiellement inconnues, fichier disparu et lecture refusée. Les libellés de
qualité ne doivent pas inventer une résolution ou une piste que le backend n'a
pas détectée.

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
