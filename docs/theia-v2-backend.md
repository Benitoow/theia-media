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

**Statut : prochain chantier, non commencé.**

Contrat produit déjà figé : le catalogue expose une seule fiche par film. La
fiche contient les fichiers disponibles et l'utilisateur choisit manuellement
celui qu'il veut lire. Le serveur ne choisit pas une « meilleure qualité » à sa
place.

Périmètre backend :

- faire évoluer le domaine et SQLite d'un fichier par film vers un film avec
  plusieurs fichiers jouables, sans perdre métadonnées ni progression ;
- réconcilier ajouts, suppressions, déplacements et renommages pendant le scan
  sans créer de faux doublons ;
- figer et tester une règle d'association conservatrice. En cas de doute, deux
  fiches séparées valent mieux qu'une fusion silencieusement fausse ;
- exposer pour chaque fichier un identifiant stable et les informations utiles
  au choix humain : nom, taille, conteneur et caractéristiques média réellement
  détectées ;
- exposer les pistes audio sélectionnables avec des identifiants stables ;
- faire sélectionner le fichier et la piste par identifiant dans les endpoints
  de lecture. Aucun chemin arbitraire ne vient du navigateur ;
- conserver les garde-fous direct-play/remux, la reprise, le seek corrigé et le
  verrou qui empêche l'updater de redémarrer pendant une lecture ;
- laisser le transcodage vidéo à la demande hors de M1-BE, réservé à M6-BE.

Handoff attendu pour M1-FE : schéma JSON exact d'une fiche avec ses fichiers,
routes de lecture, choix de piste audio, réponses d'erreur et fixture couvrant
au moins deux fichiers du même film. Les noms sont provisoires tant que le code
n'existe pas ; ils ne doivent pas être copiés comme une API déjà décidée.

### M2-BE — Profils, nouvelle mouture

**Statut : bloqué par les screenshots et croquis du mainteneur.**

Le chantier repart de zéro. Il ne restaure ni l'ancien package, ni l'ancienne
migration, ni `X-Theia-Profile`. Les références visuelles permettront d'abord
de figer les actions réelles — créer, choisir, modifier, supprimer — puis le
modèle et l'API seront conçus pour ces actions. Tant que la décision
zéro-authentification tient, les profils séparent l'expérience et la progression
mais ne deviennent ni comptes ni permissions.

### M3-BE — Séries

**Statut : backlog, à cadrer après M2.**

Modèle série/saison/épisode, parsing `SxxExx`, métadonnées TMDB TV, scan et
réconciliation, progression par épisode, épisode suivant et API consommable par
les écrans de série. Le contrat frontend sera écrit à partir du code fusionné,
pas anticipé ici.

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
| M1-BE | Non commencé | — | À publier après vérification |
| M2-BE | Bloqué par les références | — | — |
| M3-BE | Backlog | — | — |
| M4-BE | Bloqué par la sécurité | — | — |
| M5-BE | Sans backend | — | Aucun |
| M6-BE | Différé | — | — |
