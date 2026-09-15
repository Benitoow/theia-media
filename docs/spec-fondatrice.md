# THEIA - Spec Fondatrice v1.0

> Document de référence unique. Tout agent IA (Claude Code, Codex, Opus, Fable)
> travaillant sur ce projet doit lire ce fichier en premier. Toute feature hors
> de ce document nécessite une validation explicite avant implémentation.

---

## 1. Vision

Theia est un serveur média personnel, open source, 100% gratuit, pensé comme
l'anti-Plex : zéro configuration, zéro compte, zéro paywall. On branche la
machine, on scanne le réseau, ça marche.

> **Amendé en V3.3 (§14)** : « un seul binaire » devient **trois exécutables**
> (`theia-server`, `theia-player`, `theia-setup`). Le zéro-configuration, lui,
> ne bouge pas.

**Philosophie en trois règles :**
1. Si une feature demande un réglage, elle est mal conçue - sauf le réglage lui-même.
2. Si une dépendance casse la compilation croisée (Windows/Linux/macOS), elle est bannie.
3. Si Plex ou Jellyfin le font déjà bien, on ne le refait pas en v1 - on scope plus étroit, plus léger.

**Pitch en une phrase :** Navidrome a prouvé qu'un serveur média pouvait être
un seul binaire Go de 50 Mo de RAM avec une UI magnifique. Theia fait pareil,
pour la vidéo.

> **Amendé en V3.3 (§14)** : le serveur reste ce binaire Go léger et sans
> dépendance graphique. C'est la **restitution** qui quitte le navigateur, pas
> le serveur qui grossit.

---

## 2. Scénario utilisateur cible (v1)

Usage personnel, réseau local (LAN) uniquement. Un utilisateur unique, pas de
gestion de comptes multiples en v1.

1. L'utilisateur allume/branche le PC qui héberge Theia.
2. Le binaire démarre automatiquement (service au boot), scanne les dossiers médias configurés.
3. Le serveur s'annonce sur le réseau local via mDNS (`theia.local`).
4. Depuis n'importe quel appareil du réseau (TV via navigateur, autre PC), l'utilisateur ouvre `theia.local` - ou scanne un QR code affiché au premier lancement.
5. Interface façon Netflix : héros en haut, rangées par genre/catégorie, continuer à regarder.
6. Lecture directe (direct play) ou remux HLS à la volée si le format n'est pas compatible nativement.

Mobile natif : **hors scope v1**, prévu plus tard (PWA suffira pour l'instant en web responsive).

---

## 3. Stack technique (décisions arrêtées)

| Composant | Choix | Justification |
|---|---|---|
| Langage backend | **Go 1.23+** | Compilation croisée triviale, un seul binaire, excellent pour agents IA, écosystème mature *(§14 : « un seul binaire » vaut désormais pour `theia-server` ; le player est un artefact Rust séparé)* |
| Driver SQLite | **`modernc.org/sqlite`** (pure Go, sans CGO) | CGO casse la compilation croisée multi-OS - piège n°1 à éviter |
| Frontend | **SvelteKit**, build statique (`adapter-static`) | Compilé en HTML/JS/CSS statique, embarqué dans le binaire via `go:embed` |
| Style | Tailwind CSS | Rapide à itérer pour un agent IA, cohérent |
| Base de données | SQLite embarquée | Zéro serveur DB à installer, fichier unique |
| Métadonnées | API TMDB | Standard de l'industrie, gratuit, bien documenté |
| Découverte réseau | mDNS/Bonjour (ex: `github.com/hashicorp/mdns`) | Zéro-config, fonctionne nativement sur la plupart des OS/TV |
| Transcodage | `ffmpeg` en binaire externe, auto-téléchargé au premier lancement dans le dossier de données de l'app | Ne PAS embarquer ffmpeg dans le binaire (trop lourd) ; ne PAS exiger une install manuelle |
| Mise à jour | Auto-update via GitHub Releases (méthode Hermes) | Check version → téléchargement → remplacement atomique → redémarrage |
| Distribution | Un binaire par couple OS/arch (win/linux/darwin × amd64/arm64) via GitHub Actions | Releases GitHub, pas d'installeur complexe requis |

**Interdits techniques (guardrails pour les agents IA) :**
- Pas de CGO, jamais, sous aucun prétexte. *(§14 : cette règle régit le code Go ;
  `theia-player` est un artefact Rust distinct, avec sa propre chaîne d'outils.)*
- Pas de dépendance runtime au-delà de ffmpeg (auto-géré). *(§14 : reste vrai
  pour `theia-server`. `theia-player` ajoute libmpv, épinglé, SHA-256 vérifié et
  licence contrôlée - c'est la seule exception, et elle est écrite.)*
- Pas de Docker requis pour l'usage basique (Docker peut être une option de distribution *en plus*, jamais la seule voie).
- Pas de télémétrie, pas de compte cloud, pas d'appel réseau externe non essentiel (seul TMDB et GitHub Releases sont autorisés). *(inchangé en V3.3.)*
- Aucun binaire tiers distribué sans source épinglée, SHA-256 et licence vérifiée. *(§14 : la règle qui régissait FFmpeg régit désormais libmpv.)*

---

## 4. Scope v1 - ce qui EST inclus

1. **Auto-découverte** : annonce mDNS + QR code affiché au premier lancement pour connexion instantanée depuis un autre appareil.
2. **Scan de bibliothèque** : parsing de dossiers, détection de fichiers vidéo, extraction du titre/année depuis le nom de fichier.
3. **Métadonnées automatiques** : requête TMDB, récupération poster/backdrop/synopsis/casting, mise en cache locale.
4. **Interface web type Netflix** : hero carousel, rangées horizontales par genre, page détail par film/série.
5. **Lecture** : direct play (HTTP range requests) en priorité, fallback remux HLS via ffmpeg si le codec n'est pas lisible nativement par le navigateur.
6. **Suivi de progression** : reprise de lecture, rangée "Continuer à regarder".
7. **Page de réglages minimale** : dossiers surveillés, port, clé API TMDB. Rien d'autre.
8. **Auto-mise à jour** : vérification et application automatique des nouvelles releases GitHub.

## 5. Scope v1 - ce qui N'EST PAS inclus (roadmap v2+)

- Transcodage matériel GPU (VAAPI / NVENC / QSV) - v2, seulement si direct play + remux CPU s'avèrent insuffisants à l'usage réel.
- Applications natives (TV, mobile, desktop) - le web/PWA suffit pour l'instant.
  > **Amendé en V3.3 (§14)** : les applications natives **de bureau** sont
  > désormais dans le périmètre (`theia-player`). Les applications TV et mobiles
  > restent hors périmètre et sont renvoyées à une V3.4/V5.
- Gestion multi-utilisateurs avec permissions.
- Assistant IA / recherche sémantique (ex: modèle type Gemma local) - idée valable, réservée à un module optionnel téléchargeable séparément, jamais dans le binaire de base.
- Live TV / DVR.
- Accès distant hors réseau local (tunnel Tailscale/WireGuard) - candidat sérieux pour v2, c'est un vrai différenciateur face à Jellyfin.
- Système de plugins.

**Règle d'or :** toute feature listée ici ne doit PAS être implémentée avant que
tout le scope v1 soit stable et utilisé en usage réel pendant au moins quelques semaines.

---

## 6. Architecture du projet

```
theia/
├── cmd/
│   └── theia/
│       └── main.go              # point d'entrée, orchestration au démarrage
├── internal/
│   ├── scanner/                 # scan des dossiers, détection fichiers
│   ├── metadata/                # client TMDB, cache local
│   ├── library/                 # modèle de données bibliothèque (films/séries)
│   ├── stream/                  # direct play + remux HLS via ffmpeg
│   ├── discovery/                # mDNS, QR code premier lancement
│   ├── updater/                  # auto-update via GitHub Releases
│   ├── db/                       # SQLite, migrations, requêtes
│   └── api/                      # routes HTTP/REST consommées par le frontend
├── web/                          # source SvelteKit
│   ├── src/
│   └── static/
├── web-dist/                     # build SvelteKit compilé (généré, embarqué via go:embed)
├── go.mod
└── README.md
```

> **Amendé en V3.3 (§14)** : cette arborescence décrit le serveur. Deux
> répertoires s'y ajoutent - `player/` (workspace Rust : `theia-player`, FFI
> libmpv, OSD Svelte) et `setup/` (`theia-setup`, TUI Charm). `theia-server`
> reste le module Go de ce document, et `web/` reste son frontend, gelé en
> lecture de secours.

---

## 7. Jalons de développement (ordre de prompt)

Chaque jalon = une session de travail avec un agent IA. Ne pas paralléliser
M0-M2, ils sont séquentiels. M3 (frontend) peut être travaillé en parallèle
une fois l'API de M1/M2 stabilisée.

| Jalon | Contenu | Sortie attendue |
|---|---|---|
| **M0** | Scaffold du repo, module Go, serveur HTTP minimal qui sert une page statique, annonce mDNS de base | `theia` démarre et répond sur `theia.local:PORT` |
| **M1** | Schéma SQLite, scan de dossiers configurés, parsing basique de noms de fichiers | Bibliothèque de fichiers détectés visible en DB |
| **M2** | Intégration TMDB, récupération et cache des métadonnées (poster, synopsis, cast) | Chaque entrée de bibliothèque a ses métadonnées |
| **M3** | Interface web Netflix-like : hero, rangées, page détail | Navigation visuelle complète de la bibliothèque |
| **M4** | Lecture directe (range requests) + fallback remux HLS via ffmpeg | Un film se lance et se lit dans le navigateur |
| **M5** | Suivi de progression, rangée "Continuer à regarder" | Reprise de lecture fonctionnelle |
| **M6** | Onboarding premier lancement : QR code, découverte réseau | Un nouvel appareil se connecte en un scan |
| **M7** | Mécanisme d'auto-update via GitHub Releases | Le binaire se met à jour tout seul |
| **M8** | Polish : gestion d'erreurs, page de réglages minimale, packaging des releases | Version 1.0 publiable |

---

## 8. Répartition suggérée entre tes outils IA

Tu as Claude Code, Codex, Opus 5 et Fable 5 sous la main. Ne les utilise pas
au hasard - chacun a un terrain de jeu naturel :

- **Claude Code (Opus/Sonnet)** : backend Go - architecture, scanner, stream,
  updater, db. C'est là que la rigueur systémique compte le plus (compilation
  croisée, gestion de fichiers, concurrence). Donne-lui ce document en entier
  à chaque nouvelle session.
- **Codex / GPT** : frontend SvelteKit - composants UI, animations, responsive.
  Bon terrain pour lui, itération visuelle rapide, moins critique si un
  composant est refait deux fois.
- **Fable 5** : parfait pour du contenu - rédaction du README, de la doc
  utilisateur, des messages d'erreur clairs, éventuellement les visuels/assets
  de marque de Theia.
- Ne fais jamais travailler deux agents sur `internal/` en parallèle sans
  synchroniser - le risque de conflits d'architecture (deux façons différentes
  de structurer le scanner, par exemple) est réel.

---

## 9. Nom et identité

**Nom du projet : Theia** - Titanide grecque de la vue et de la lumière céleste,
mère d'Hélios (le soleil). Cohérent avec ton écosystème mythologique (Hermes).
Nom de binaire : `theia`. Repo GitHub suggéré : `theia-media`.

---

## 10. Critère de succès v1

Theia v1 est un succès si, dans les faits : tu branches le PC, tu ouvres un
navigateur sur ta TV sans taper une seule adresse IP manuelle, et un film se
lance en moins de 3 clics. Si ça prend plus que ça, on a raté la mission.

---

## 11. Addendum - Décisions de cadrage M0

Questions soulevées par l'agent au démarrage de M0, tranchées ici pour référence.

1. **Remux vidéo + transcode audio AAC autorisé** quand la piste audio n'est
   pas lisible nativement (AC3/DTS/TrueHD). Le transcode vidéo complet reste
   hors scope v1. "Remux" dans ce document ne doit pas être lu littéralement :
   l'objectif est "ça marche", pas "on ne touche qu'au conteneur".
2. **Séries hors scope v1, jalon dédié en v1.1** (juste après M8) : modèle
   série/saison/épisode, parsing `SxxExx`, endpoints TMDB TV, enchaînement
   automatique. v1 = films uniquement.
3. **Sous-titres v1** : fichiers `.srt`/`.vtt` externes + extraction de pistes
   texte (SRT/ASS) intégrées au conteneur via ffmpeg. PGS/VobSub (image)
   explicitement hors scope v1.
4. **Clé API TMDB embarquée dans le binaire par défaut**, surcharge possible
   dans les réglages pour qui veut sa propre clé. Zéro-config veut dire zéro-
   config, y compris pour les métadonnées.
5. **QR code au premier lancement = chemin de connexion principal**, mDNS/
   `.local` = confort additionnel pour les appareils compatibles, pas une
   garantie. Pas de PWA installable en v1 (nécessiterait HTTPS, hors scope) :
   web responsive simple.
6. **Zéro authentification, confirmé et assumé** : quiconque est sur le réseau
   local accède à tout, réglages compris. Avertissement obligatoire et visible
   dans le README : ne jamais exposer Theia directement sur internet.
7. **Binaire lancé manuellement en v1** ; `theia install-service` est une
   commande optionnelle explicite, pas un comportement par défaut. Pas
   d'élévation admin imposée au premier lancement.
8. **Logistique** : `git init` + repo GitHub public `theia-media` dès M0.
   Code, commentaires et messages d'erreur internes en anglais (convention
   open source). UI utilisateur en français pour v1, avec les chaînes
   isolées dans un fichier dédié pour permettre une i18n future sans
   réécriture. L'updater est conçu à partir du pattern documenté (§3, ligne
   "Mise à jour") - pas de code source Hermes disponible à réutiliser tel
   quel ; attention particulière à la contrainte Windows (un `.exe` en cours
   d'exécution ne peut pas se remplacer lui-même : prévoir un petit binaire
   ou script relais).
9. **Clé TMDB, nuance ajoutée après inscription** : la clé enregistrée est
   légitimement "personal use" au sens TMDB (projet gratuit, non-commercial,
   sans publicité). Reste embarquée par défaut dans le binaire (§11.4
   inchangé) pour le zéro-config, mais avec dégradation gracieuse prévue :
   si le rate-limit partagé est atteint à mesure que le projet grandit, un
   message dans les réglages invite l'utilisateur à ajouter sa propre clé
   gratuite plutôt que de casser silencieusement l'affichage des posters.
10. **Logo v1 retenu** : wordmark "THEIA" en serif capitales, lettres
    révélant une image de lever de soleil/horizon terrestre (thème "mère du
    soleil"). Licence de l'image source à vérifier avant tout usage public
    (README, favicon) - préférer une source domaine public (NASA) ou générée.
    Ce traitement photographique sert de pièce de prestige (README, écran de
    démarrage) ; une version simplifiée à plat (couleur unique) reste à
    produire pour favicon/icône d'app/nav bar, où le détail photographique
    ne survit pas à la réduction de taille.

---

## 12. Identité visuelle (DA)

Pivot assumé depuis "façon Netflix" (générique, rouge/noir) vers une direction
éditoriale/luxe, validée sur moodboard (Luxam, Rinascimento, Lavoza, Taste the
Notes) :

- **Palette** : fond quasi-noir dominant, texte écru/crème chaud, un seul
  accent (rouge oxblood ou or) utilisé avec parcimonie.
- **Typographie** : display serif dramatique (didone/blackletter) pour les
  titres et le hero, sans-serif fine et discrète pour la navigation et les
  labels UI. Contraste fort entre les deux.
- **Ambiance** : imagerie sombre et cinématique, sculpture/statuaire classique
  en référence visuelle pour les écrans d'accroche, beaucoup d'espace négatif.
- **Périmètre d'application** : le chrome de l'app (nav, hero, écrans vides,
  page de connexion QR, typographie globale) porte cette identité à fond. La
  **grille d'affiches reste dense et fonctionnelle** - les posters/backdrops
  TMDB portent déjà leur propre esthétique, pas besoin d'espace négatif façon
  galerie d'art à cet endroit précis.
- **Assets décoratifs** : jamais scrapés automatiquement par l'agent (risque
  de licence dans un repo public GPL-3.0). Sélectionnés à la main par
  l'utilisateur, licence CC0/libre vérifiée, déposés en assets fixes du repo.

---

## 13. Backlog v1.1+ - raffinement visuel (ne pas injecter avant la release v1)

Références glanées en cours de route, à évaluer une fois v1 livrée et
stable, jamais pendant un jalon de polish/release actif :
- Traitement de hero en portrait "par couches"/relief (réf. Apple TV+),
  alternative possible au backdrop plat actuel - à évaluer après usage réel.
- Le dashboard générique violet/pilule (réf. Netflix) est explicitement
  écarté - contredit le choix de l'or et l'identité éditoriale déjà actée.

---

## 14. Amendement V3.3 - architecture native (serveur / player / setup)

> Décision appliquée : [`DECISIONS.md`](DECISIONS.md) 117. **Cet amendement prime
> sur les clauses qu'il cite.** Les textes d'origine restent en place comme
> l'histoire du projet, pas comme des contraintes actives.

### 14.1 Le constat, et ce qu'il change

Le plafond de verre du navigateur n'est pas une impression : aucune API Web
n'expose le passthrough HDMI, donc le TrueHD/Atmos et le DTS-HD MA ne peuvent
pas atteindre un amplificateur autrement qu'en PCM réencodé ; le profil 7 du
Dolby Vision n'est pas géré ; le MKV n'est pas un conteneur natif du navigateur ;
et le rendu ASS complexe se paie en CPU. Ces limites sont des propriétés de la
plateforme, pas des défauts de Theia.

| Clause amendée | Sens en V3.3 |
|---|---|
| §1, §3 et §3 (pitch Navidrome) : « un seul binaire » | **trois exécutables** : `theia-server` (Go, headless), `theia-player` (Tauri 2 + Rust + libmpv), `theia-setup` (Go + Charm). Les **noms d'assets publiés** du serveur restent `theia-<os>-<arch>` : les installations v3.2 se mettent à jour avec ces noms, les renommer les abandonnerait. |
| §3 : « pas de dépendance runtime au-delà de ffmpeg » | reste vrai **pour `theia-server`**. `theia-player` ajoute **libmpv** : source épinglée, SHA-256 vérifié, licence contrôlée, téléchargée au premier besoin. Il utilise en plus le moteur web de la plateforme (WebView2, WKWebView, WebKitGTK), qui n'est ni téléchargé ni épinglé par Theia - exception nommée, pas oubli. |
| §5 : « applications natives hors périmètre » | les applications **de bureau** entrent dans le périmètre. Les applications TV et mobiles restent dehors (V3.4/V5). |
| §2 étape 4 et §10 : « depuis un navigateur, sur la TV, en moins de 3 clics » | le critère de succès V3.3 passe par `theia-player` pour la restitution ; le navigateur reste la voie d'administration et de secours. Le critère lui-même est réécrit dans [`v3.3.md`](v3.3.md). |
| §11.7 : « binaire lancé manuellement » *(reformulé, pas supprimé)* | `theia-setup` installe un service `systemd`, une tâche planifiée Windows ou un agent `launchd`, **sur demande explicite** uniquement. Aucune élévation imposée. |

Les trois premières lignes **supersèdent** ; la quatrième **reformule** ; tout ce
qui n'est pas cité ici reste en vigueur, y compris les §1-§13 non contredits.

### 14.2 Ce qui ne change pas

Ces règles restent des interdits, pas des préférences :

- **Pas de CGO dans le Go, jamais.** Cette règle régit le serveur ; le player
  est un artefact Rust distinct, avec sa propre chaîne d'outils.
- **Aucune télémétrie, aucun compte cloud.** Les seuls appels sortants restent
  TMDB et GitHub Releases ; l'accès distant reste du WireGuard entrant, sans
  relais, sans STUN, sans plan de contrôle.
- **Docker n'est jamais requis.**
- **Aucun binaire tiers distribué sans source épinglée, SHA-256 et licence
  vérifiée.** La règle qui régissait FFmpeg régit désormais libmpv.
- **Le serveur n'écrit jamais ce que l'utilisateur lit** (décision 25).
- **Interface en français et en anglais**, catalogues séparés, parité vérifiée.
- **Le design system reste la référence visuelle**, y compris pour l'OSD du
  player natif : une seule identité, pas deux.
- **On rapporte ce qui a été mesuré.** Windows est la seule plateforme de
  validation réelle de la V3.3. macOS, Linux, Android TV, Apple TV et iOS sont
  écrits mais **non vérifiés**, et le disent.

### 14.3 La machine ne devine pas son rôle

Un mini-PC sans écran dans un placard réseau et un HTPC branché à un téléviseur
partagent souvent la même empreinte matérielle. Aucune détection automatique ne
peut les distinguer de façon fiable. Le rôle est donc **déclaré** au moment de
l'installation, par `theia-setup` : serveur seul, lecteur seul, ou tout-en-un.
Le rôle par défaut est `tout-en-un`, parce qu'un utilisateur qui lance
l'installeur sur son propre PC veut voir un film, pas administrer un serveur.
