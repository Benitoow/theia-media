# THEIA — Feuille de route v2

> Document de consolidation. Sert de point de départ pour la prochaine phase
> de développement. Complète, sans les remplacer, docs/spec-fondatrice.md et
> docs/DECISIONS.md qui restent l'historique détaillé des décisions v1.

---

## 1. État actuel : v1.5.0

Scope v1 (M0-M8) entièrement livré et vérifié en conditions réelles :
scan de bibliothèque tolérant, métadonnées TMDB, lecture directe + remux/
transcode audio via ffmpeg auto-téléchargé, suivi de progression, onboarding
QR code + mDNS, auto-update testé contre la vraie API GitHub.

Ajouts post-v1 livrés et publiés :
- Interface D-pad complète pour usage télécommande à trois mètres.
- Page `/films` : recherche, tri, filtres sur toute la bibliothèque.
- Lecteur vidéo redessiné (icônes custom, barre avec plage tamponnée,
  masquage auto des contrôles).
- Accueil recentré sur une surface personnelle (reprise, ajouts récents,
  sélection "au hasard ce soir") plutôt qu'un second catalogue redondant
  avec `/films`.
- Format de carte 16/9 sur backdrop (remplace le 2/3 verrouillé depuis M3,
  déverrouillage assumé — voir décisions 33/34).
- Internationalisation FR/EN.
- README technique : légèreté mesurée sur le vrai binaire, tableau
  comparatif sourcé face à Plex/Jellyfin/Emby.

---

## 2. Leçons retenues (pour tout futur agent)

Ces bugs ont un point commun : chacun n'a été trouvé qu'en vérifiant à
l'écran ou en conditions réelles, jamais sur simple lecture de code ou
supposition. C'est la discipline à ne jamais relâcher.

- **Le favicon n'a jamais fonctionné, de M2 à v1.4.0.** Un commentaire XML
  contenant un double tiret (`--`) est invalide selon la spec XML. Toléré
  par le navigateur en HTML inline, rejeté quand le même SVG est chargé
  comme ressource image externe (ce que fait un onglet). Aucun test
  automatisé ne l'attrapait ; seul un test manuel de décodage l'a révélé.
- **TMDB ne trie pas par popularité.** `search/movie` classe par pertinence
  textuelle. Prendre le premier résultat pouvait renvoyer un making-of
  (popularité 0,6) au lieu du vrai film (popularité 19,1). Corrigé en
  comparant aussi `original_title` et en retenant le résultat le plus
  populaire, jamais le premier.
- **Le décalage de lecture en mode remux** pouvait sauvegarder une position
  de reprise fausse après un repositionnement — corruption silencieuse de
  données sans aucune erreur visible.
- **La réconciliation du scan basée sur des horodatages** confondait mise à
  jour et ajout si deux scans avaient lieu dans la même seconde. Remplacé
  par un compteur de génération entier, atomique.
- **Une sélection "aléatoire" à base de `(id * k) mod p`** n'était pas
  aléatoire du tout — progression arithmétique à pas fixe, découverte
  seulement en imprimant les identifiants bruts et en les regardant, jamais
  via les tests automatisés qui passaient tous.

Règle générale qui en découle : un test qui passe ne prouve que ce qu'on a
pensé à lui demander de vérifier. Le doute doit porter aussi sur l'outil de
vérification lui-même (cf. le cas du panneau d'aperçu qui ne recalculait
pas le style, révélé par un test de contrôle avant de conclure à un bug CSS
inexistant).

---

## 3. Points ouverts, décision utilisateur requise avant de prompter

- **Logo dans la nav** : le wordmark photographique recadré est jugé mal
  intégré (rectangulaire, mauvaise couleur perçue). Retour à un texte
  simple "THEIA" en attendant un vrai travail de direction artistique dédié
  au logo de nav — ne pas retenter un câblage rapide sans ça.
- **`web/static/icon-512.png`** (208 Ko) : embarqué dans le binaire,
  référencé nulle part, aucun manifeste PWA dans le projet. À trancher :
  retirer, ou construire le manifeste qui le justifie.

---

## 4. Backlog v2, organisé en jalons

Même discipline que v1 : un jalon à la fois, checkpoint avant de continuer,
jamais deux agents en parallèle sur le même territoire frontend sans
synchronisation explicite (séquencer, pas paralléliser).

### V2-M1 — Dédoublonnage de fichiers + sélection de qualité
Le chantier resté en suspens depuis plusieurs sessions. Réel flou de scope
qui justifie la technique d'interview (Claude Code pose les questions via
son outil dédié plutôt que d'exécuter une spec devinée) :
- Un film = une fiche, plusieurs fichiers possibles dessous, sélection sur
  la page du film.
- Qualité audio sélectionnable (a minima) — probablement gratuit une fois
  le multi-fichier en place, si plusieurs pistes existent déjà dans un même
  fichier.
- Qualité vidéo à la demande sur un fichier unique (ex : lire un REMUX
  2160p en 720p60) implique un vrai transcodage à la volée — ce n'est pas
  gratuit, ça rouvre le chantier "optimisation CPU/GPU" mis de côté
  jusqu'ici. À trancher explicitement : combien de ce scope maintenant, et
  combien attend la vraie passe d'optimisation matérielle.

Interview tranchée le 31/07/2026 — le jalon est promptable tel quel :
- **Regroupement par nom de base identique** : deux fichiers portant le
  même nom sans extension (hors casse et hors extension) sont une seule
  fiche avec plusieurs fichiers dessous.
- **Sélection manuelle** : le choix du fichier se fait sur la page du film,
  pas d'auto-sélection par qualité au lancement.
- **Qualité vidéo à la demande éjectée vers V2-M6** : le transcodage live
  rouvre le chantier optimisation matérielle, pas de demi-mesure dans M1.
  Le multi-fichier + pistes audio du même fichier couvrent le besoin.

Point d'attention relevé sur la vraie bibliothèque (282 fichiers) pendant
l'interview : plusieurs paires du même film n'ont pas le nom de base brut
identique (`2001 A Space Odyssey 1968.mkv` / `2001_A_Space_Odyssey_1968_720p.mp4`,
`1917 2019 REMASTERED 1080p BluRay.mkv` / `1917.mkv`). La règle de
comparaison exacte (normalisation des séparateurs, tolérance sur l'année)
est à fixer au moment de l'implémentation — le parser existant
`internal/library/parse.go` est le candidat naturel.

### V2-M2 — Profils, nouvelle mouture
Les profils reviennent en V2 pour séparer l'expérience et la progression des
membres du foyer, mais l'ancienne implémentation ne sert pas de point de départ.
Le chantier repart d'une base vierge : pas de résurrection de l'écran, de l'API
ou du modèle visuel retirés après v1.5.0.

Le frontend est bloqué jusqu'à réception des screenshots et croquis fournis par
le mainteneur. Ces références deviennent le contrat visuel du jalon : structure,
hiérarchie, interactions D-pad et rendu à trois mètres doivent les respecter,
pas être improvisés par l'agent. Les comportements précis et le nouveau modèle
de données seront cadrés avec ces références avant la première ligne de code.
Tant que la décision zéro-authentification reste en vigueur, un profil n'est ni
un compte, ni une permission.

### V2-M3 — Séries
Le plus gros morceau du backlog, mérite son propre cycle multi-session
complet façon M0-M8, pas une phase parmi d'autres. Modèle série/saison/
épisode, parsing `SxxExx`, endpoints TMDB TV, enchaînement automatique
d'épisode, "continuer à regarder" adapté aux séries.

### V2-M4 — Accès distant hors réseau local
Identifié dès le tout premier message comme LE différentiateur manquant
face à Plex. Jamais commencé. Implique de vraies décisions de sécurité
(la décision #6 "zéro authentification" ne peut plus tenir telle quelle
si Theia devient joignable depuis internet) — probablement le chantier qui
demande le plus de réflexion en amont avant tout prompt.

### V2-M5 — Logo et identité de nav
Vrai travail de direction artistique, pas un câblage technique. À traiter
comme la conception du logo initial (plusieurs pistes, validation avant
implémentation), pas comme une correction de bug.

### V2-M6 — Optimisation matérielle
CPU/GPU, iGPU inclus. Explicitement "plus tard" depuis le premier message
de ce projet. Ne pas commencer avant que tout le reste soit stable, et
probablement à coupler avec la partie qualité vidéo de V2-M1 si elle a été
partiellement ouverte.

### Housekeeping continu
- Nettoyage code mort/dupliqué : à refaire en amont de chaque gros jalon,
  pas une seule fois pour toutes.
- `docs/design-system.md` et `docs/DECISIONS.md` : continuer à les traiter
  comme l'autorité vivante, mise à jour à chaque changement de composition,
  pas seulement les tokens numériques.

### Explicitement écarté, pas en backlog
- Rotten Tomatoes / Letterboxd comme sources alternatives : RT exige une
  licence commerciale payante avec candidature et délai d'approbation,
  Letterboxd n'accorde l'accès à son API que sur demande sans garantie —
  aucune des deux n'offre l'équivalent d'une clé TMDB auto-servie et
  gratuite. Repris uniquement si l'un des deux change fondamentalement ses
  conditions d'accès.

---

## 5. Rappel de discipline (celle qui a fait tenir tout le projet)

- Un jalon, une vérification à l'écran, un feu vert — jamais un agent qui
  enchaîne plusieurs chantiers sans checkpoint humain.
- "Vérifié" veut dire testé en conditions réelles, jamais supposé à partir
  de la lecture du code seule.
- Toute modification d'une règle de composition du design system (pas les
  simples valeurs numériques) doit être signalée et validée par toi avant
  d'être appliquée, même si le raisonnement de l'agent est bon.
- Licence de tout asset (image, police, logo) vérifiée par toi avant qu'il
  entre dans le repo public GPL-3.0 — jamais par l'agent seul.
- Ne jamais faire tourner deux agents en parallèle sur le même territoire
  frontend sans synchronisation explicite entre les deux sessions.
