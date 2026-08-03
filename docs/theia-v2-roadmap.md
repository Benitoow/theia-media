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

### Deux pistes, un seul contrat produit

La V2 est livrée dans deux pistes explicites :

- [`theia-v2-backend.md`](theia-v2-backend.md) est la file de travail backend,
  actuellement prise en charge avec Codex.
- [`theia-v2-frontend.md`](theia-v2-frontend.md) est la file de travail frontend,
  destinée à Claude une fois le backend correspondant fusionné.

On avance **jalon par jalon et backend d'abord**. Finir `M1-BE` ne termine pas
V2-M1 : cela publie un contrat vérifié que `M1-FE` peut consommer plus tard.
Chaque handoff backend contient les endpoints et payloads réels, les migrations,
les erreurs attendues, le hash du commit et ce qui a été vérifié sur la vraie
bibliothèque. Si ce contrat change, les deux pistes et `DECISIONS.md` changent
dans le même commit. Claude ne doit jamais reconstruire l'état du backend à
partir d'une ancienne conversation.

| Jalon | Backend | Frontend | Blocage actuel |
|---|---|---|---|
| V2-M1 — fichiers et qualités | [`8518bab`](https://github.com/Benitoow/theia-media/commit/8518bab69a84a0f1a5073a16694e4efd52b0a02e), [PR #4](https://github.com/Benitoow/theia-media/pull/4) | Implémenté et vérifié (décision 47) | Reste la validation sur un vrai film décodable |
| V2-M2 — profils | Implémenté et vérifié (décision 48) | Contrat visuel figé (décision 48) | Reste `M2-FE` |
| V2-M3 — séries | [`5b2615e`](https://github.com/Benitoow/theia-media/commit/5b2615e77655e41567f339e68de3cf7c8e0a05d7), [PR #5](https://github.com/Benitoow/theia-media/pull/5) | Prêt sur le contrat M3-BE | Reste M3-FE + validation sur les premiers fichiers série utilisateur |
| V2-M4 — accès distant | [`a547528`](https://github.com/Benitoow/theia-media/commit/a547528ddb0606a3dbe21c44015ced5088c78d2a) | Prêt sur le contrat M4-BE après fusion | Reste M4-FE + validation via un vrai endpoint hors LAN |
| V2-M5 — logo/navigation | Aucun chantier backend | Backlog | Direction artistique |
| V2-M6 — optimisation matérielle | Différé | Attend `M6-BE` | À ouvrir après stabilisation du reste |

### V2-M1 — Dédoublonnage de fichiers + sélection de qualité
Le chantier resté en suspens depuis plusieurs sessions. Le flux visible est
maintenant tranché ; la règle technique d'association appartient à `M1-BE` :
- Un film = une fiche, plusieurs fichiers possibles dessous, sélection sur
  la page du film.
- Qualité audio sélectionnable dans les pistes réellement mesurées du fichier ;
  une sélection explicite passe par le remux pour garantir la piste.
- Qualité vidéo à la demande sur un fichier unique (ex : lire un REMUX
  2160p en 720p60) implique un vrai transcodage à la volée — ce n'est pas
  gratuit, ça rouvre le chantier "optimisation CPU/GPU" mis de côté
  jusqu'ici. À trancher explicitement : combien de ce scope maintenant, et
  combien attend la vraie passe d'optimisation matérielle.

Interview tranchée le 31/07/2026 — le jalon est promptable tel quel :
- **Regroupement par nom de base identique** : deux fichiers portant le
  même nom sans extension (hors casse et hors extension) sont une seule
  fiche avec plusieurs fichiers dessous.
- **Une fiche, puis choix manuel** : le catalogue garde une seule carte par
  film. L'utilisateur ouvre sa fiche, voit les fichiers disponibles et choisit
  celui à lire. Aucun sélecteur avant la fiche et aucune sélection automatique
  par qualité au lancement.
- **Qualité vidéo à la demande éjectée vers V2-M6** : le transcodage live
  rouvre le chantier optimisation matérielle, pas de demi-mesure dans M1.
  Le multi-fichier + pistes audio du même fichier couvrent le besoin.

Point d'attention relevé pendant l'interview : plusieurs paires du même film
n'ont pas le nom de base brut
identique (`2001 A Space Odyssey 1968.mkv` / `2001_A_Space_Odyssey_1968_720p.mp4`,
`1917 2019 REMASTERED 1080p BluRay.mkv` / `1917.mkv`). La règle de
comparaison est maintenant codée et vérifiée dans M1-BE : parser `titre + année`
quand l'année existe, nom de base exact quand elle manque, conflit TMDB bloquant
et `tmdb_id` identique comme preuve finale. La copie actuelle de la base passe
de 274 lignes fichier à 248 films / 274 fichiers, dont 25 films multi-fichiers,
sans perdre métadonnées ni progression. Le contrat complet est dans
`theia-v2-backend.md`.

### V2-M2 — Profils, nouvelle mouture
Les profils reviennent en V2 pour séparer l'expérience et la progression des
membres du foyer, mais l'ancienne implémentation ne sert pas de point de départ.
Le chantier repart d'une base vierge : pas de résurrection de l'écran, de l'API
ou du modèle visuel retirés après v1.5.0.

Les références sont arrivées le 03/08/2026 et le contrat est figé par la
**décision 48** : sélecteur en écran plein, entrée de nav qui ouvre cet écran
plutôt qu'un menu déroulant, fiche de profil à deux panneaux ne portant que des
faits locaux, avatar fourni par l'utilisateur, et écran d'accueil tant qu'aucun
profil n'est actif dans ce navigateur.

Les références montrent des écrans Netflix, donnés pour la **disposition, pas le
style**. Tout ce qu'ils portent de compte — déconnexion, email, rôle, statut,
badge d'abonnement, transfert de profil, centre d'aide, notifications — est
refusé, pas traduit. Leurs illustrations ne peuvent pas entrer dans un dépôt
public GPL-3.0.

Il reste à concevoir `M2-BE` : aucune route profils n'existe aujourd'hui. Tant
que la décision zéro-authentification reste en vigueur, un profil n'est ni un
compte, ni une permission.

### V2-M3 — Séries
Le plus gros morceau du backlog, mérite son propre cycle multi-session
complet façon M0-M8, pas une phase parmi d'autres. Modèle série/saison/
épisode, parsing `SxxExx`, endpoints TMDB TV, enchaînement automatique
d'épisode, "continuer à regarder" adapté aux séries.

M3-BE est implémenté dans
[`5b2615e`](https://github.com/Benitoow/theia-media/commit/5b2615e77655e41567f339e68de3cf7c8e0a05d7)
et livré par la [PR #5](https://github.com/Benitoow/theia-media/pull/5). Le modèle
additif série/saison/épisode, les items multi-épisodes, les fichiers et pistes
audio, TMDB TV, le streaming, la progression single-viewer, le prochain épisode
et l'accueil séries sont couverts par le contrat de `theia-v2-backend.md`.

Les choix sont désormais figés : `SxxExx`/`1x02` seulement, un item combiné
pour plusieurs numéros, `S00` séparé et hors autoplay, prochain item possédé
avec indicateur de trou. M2 migrera la progression vers les futurs profils ; M3
n'a pas restauré l'ancien modèle supprimé.

La bibliothèque active actuelle contient 274 vidéos et toujours aucune série :
le scan isolé confirme zéro faux positif, pas un import positif utilisateur. Le
positif a donc été vérifié sur un corpus décodable séparé avec TMDB réel, saison
spéciale, multi-épisode, deux qualités, deux pistes, Range et remux. M3-FE est
débloqué, mais devra refaire la validation dès que les premiers fichiers série
réels seront disponibles.

### V2-M4 — Accès distant hors réseau local

Le backend est implémenté dans
[`a547528`](https://github.com/Benitoow/theia-media/commit/a547528ddb0606a3dbe21c44015ced5088c78d2a).
La décision zéro-authentification reste vraie sur le LAN ; hors LAN, chaque
appareil prouve une clé WireGuard créée localement. Ce n'est ni un compte, ni un
profil, ni une permission par personne.

Le tunnel est entièrement embarqué en Go avec une netstack userspace : pas de
TUN système, de CGO, d'installation VPN sur le serveur, de compte cloud ou de
control plane. Le propriétaire fournit l'endpoint et la redirection UDP. Theia
ne traverse pas le CGNAT, ne contacte aucun relais et ne publie jamais le port
HTTP 8383.

Le distant reçoit une capacité de lecture : catalogue, images, streams,
inspection et progression. Réglages, scans, onboarding, updater et gestion des
appareils restent LAN-only. M4-FE doit maintenant construire le panneau local de
configuration/provisioning et adapter la navigation distante à partir du
contrat exact de `theia-v2-backend.md`. Le jalon produit ne sera complet qu'après
un test navigateur via un vrai endpoint extérieur ; le backend a été vérifié
avec un vrai tunnel et un client séparé sur UDP loopback, ce qui prouve le code,
pas la box internet du mainteneur.

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
