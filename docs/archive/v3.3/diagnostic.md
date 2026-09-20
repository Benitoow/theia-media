# Phase 0 — Diagnostic comparatif Theia V3.3

**Campagne du 15 septembre 2026, 23 h 15 → 00 h 05 (heure locale).**
Exécutant : DeepSeek V4.1 Flash, session DSH. Unité : phase 0 seule, aucun
correctif produit.

- Dépôt : `C:\Users\starx\Documents\CODE\Theia`, branche `main`,
  HEAD `249532e37770c4678fa321ee85de4f92808b7caf`, arbre propre avant et après
  la campagne (`git status --short` vide, `git diff --binary` vide).
- Répertoire de campagne :
  `C:\Users\starx\AppData\Local\Temp\theia-v33-polish-20260915-231530`
  (captures brutes, journaux, sorties, pilotes de mesure). Rien de tout cela
  n'est dans le dépôt, rien n'est commité.
- Machine : Windows 11 Famille 10.0.26200, AMD Ryzen AI 9 HX 370, Radeon 890M
  (pilote 32.0.31041.1004), 31,1 Go de RAM, écran 1440×900 **à 192 ppp, soit
  200 %**. Un pixel CSS vaut deux pixels physiques sur toute cette campagne.
- Aucun processus Theia ne tournait au départ. Les ports 8383, 8395, 8396 et
  8397 étaient libres ; 5199 a été ouvert par nous seuls (prévisualisation de
  l'OSD) et le serveur de test sur 8395 est le nôtre. Aucun processus arrêté.
- Aucune donnée personnelle lue : `%APPDATA%\Theia` et
  `%LOCALAPPDATA%\Programs\Theia` ont été constatés par `Test-Path` et par
  liste de noms, jamais ouverts ; la base personnelle n'a pas été touchée.
  Aucune installation, désinstallation ni modification du système.

**Statuts employés :** vérifié / échec / non vérifié / en attente de décision.
Le mot « simulation » marque une preuve obtenue dans Chromium avec
`window.__TAURI__` simulé, ou sur une bibliothèque synthétique.

---

## 1. Baseline (unité 0.3) — tout au vert, rien à corriger

Chaque commande a été lancée depuis la racine, `CGO_ENABLED=0`, sortie et code
conservés dans `logs/`. Les lettres renvoient au plan.

| # | Commande | Résultat | Durée | Statut |
|---|---|---|---|---|
| B | `go test ./...` | 0 échec sur tous les paquets | 19,3 s | vérifié |
| G | `go test ./internal/setup/ -v` (couvert par B) | `ok internal/setup 6.352s` | — | vérifié |
| W | `node scripts/contrast.mjs` | 11 ratios sur 11 conformes au document | < 1 s | vérifié |
| W | `node web/scripts/check-locales.mjs` | `fr ↔ en (684 valeurs, 54 fonctions)` | < 1 s | vérifié |
| W | `npm --prefix web run check` | 0 erreur, 0 avertissement | ~20 s | vérifié |
| B | `.\build.ps1` | `theia-server.exe` construit, `web-dist` écrit | ~45 s | vérifié |
| O | `npm --prefix player/ui run check:i18n` | `osd locales agree (fr 33, en 33)` | < 1 s | vérifié |
| O | `npm --prefix player/ui run build` | 113 modules ; CSS 10 741 o ; JS 57 890 o | 3,0 s | vérifié |
| R | `npm --prefix player/ui run check:render` | `render check passed`, 7 planches | ~9 s | vérifié |
| N | `.\build-player.ps1` | profil debug, 14,7 Mo | 48,7 s | vérifié |
| N | `cargo test --manifest-path player/Cargo.toml` | 6 tests sur 6 | 4,5 s | vérifié |
| L | `npm --prefix web test` (défaut 8396) | 81 réussis, 3 ignorés, 0 échec | 27,3 s | vérifié |
| — | `go build -trimpath ./cmd/theia-setup` | 12 970 496 octets | ~10 s | vérifié |
| — | `scripts/capture-tui.ps1` 100×30 | exit 0, sonde : 3 couleurs | ~6 s | vérifié |
| — | `scripts/capture-window.ps1` | pid 36040, fenêtre 1100×700 réels, DPI 192 | ~9 s | vérifié |
| P | `npm --prefix web run test:playback` | **non lancée** | — | en attente de D0 |

Aucune dépendance manquante : `node` 24.18.0, `npm` 12.0.2, `cargo` 1.97.1,
Go dans `C:\Users\starx\go-toolchain\go`. FFmpeg n'est pas sur `PATH`, ce qui
est le cas nominal : `theia-server` l'a téléchargé lui-même (87,9 Mo, SHA-256
`546580347aa7…`) dans le répertoire de campagne au moment de fabriquer la
bibliothèque jouable. `D:\` n'existe pas, comme le plan le supposait.

**La référence est donc verte.** Les écarts qui suivent ne sont pas des
régressions de la baseline : ils étaient déjà là, et la baseline ne les voit
pas parce qu'aucun harnais ne les regarde.

---

## 2. Protection des surfaces gelées (unité 0.2)

`before/frozen-surfaces.csv` : 313 fichiers suivis, avec SHA-256 et taille —
`web/**`, `cmd/theia-server/**`, `internal/**` sauf `internal/setup/`,
`player/**`, plus `go.mod`, `go.sum`, `web/package-lock.json`,
`player/Cargo.lock`, `player/libmpv.json`, `embed.go` et les trois scripts de
construction. Aucun de ces fichiers n'a été modifié pendant la campagne
(vérifié en fin de campagne, section 9).

Le libmpv épinglé a été vérifié plutôt que supposé : le `libmpv-2.dll` du
disque vaut `6f059354c5c45b41192cc52d867d94c0044edb48207c4efd2ea1244208c55359`
et 100 021 760 octets, exactement le manifeste `player/libmpv.json`. Les trois
copies présentes sur la machine (bundle joueur, archive 3.3.0, dossier de sonde)
donnent le même digest.

---

## 3. Les écarts

Chaque écart porte : reproduction, mesure, fichier probable, preuve, gravité,
phase de résolution proposée. Aucun correctif n'a été appliqué.

### E1 — L'OSD ne charge pas ses fontes : le lecteur natif n'est pas dans la bonne police

- **Reproduction.** Construire l'OSD (`O`) puis l'ouvrir : `document.fonts`
  est vide, aucune règle `@font-face` n'existe dans le CSS construit.
- **Mesure.** CSS construit de l'OSD : 10 741 octets, **0 règle `@font-face`**,
  0 fichier `.woff2` émis. CSS construit du web : 88 813 octets, **2 règles
  `@font-face`**, deux `.woff2` émis (`cinzel-latin-wght-normal`, `jost-latin-wght-normal`).
  Surfaces calculées : `.library-title` → `Georgia` (repli de `--font-display`),
  `.label` → `Segoe UI` (repli de `--font-label`), `.film-name` → `Arial`.
  La planche 1280×720 montre « Bibliothèque » en Georgia, pas en Cinzel.
- **Cause.** `player/ui/src/osd.css:8` importe `web/src/lib/tokens.css`, qui
  **nomme** les familles (`--font-display`, `--font-label`) mais ne les déclare
  pas. Les `@font-face` vivent dans `web/src/app.css:32-56`, que l'OSD n'importe
  pas — et ne peut pas importer tel quel, `app.css` commençant par
  `@import 'tailwindcss'` et 4 000 lignes d'utilitaires. `player/ui/package.json`
  ne dépend d'aucun `@fontsource-variable/*`, et
  `player/ui/node_modules/@fontsource-variable` **n'existe pas** après `npm ci` :
  le fichier n'est même pas téléchargé.
- **Ce que cela invalide.** Toute la comparaison typographique prévue en phase 3
  mesure aujourd'hui l'écart entre Georgia et Cinzel, pas un écart de taille.
  C'est la moitié de l'écart visuel perçu entre le web et l'OSD, et il est
  structurel, pas esthétique.
- **Piège à éviter.** `document.fonts.check('1rem "Cinzel Variable"')` renvoie
  **`true`** dans l'OSD alors qu'aucune face n'est déclarée. Le contrôle est
  donc inutilisable seul : la phase 1 a raison d'exiger « FontFace déclarée,
  fichier demandé avec succès, famille utilisée par le style calculé ».
- **Preuve.** `before/probe2/probe2-osd-1280x720.json` (`cssAudit`),
  `before/osd-native-viewport/measure-osd.json`, `before/osd-library-1280x720-fr.png`.
- **Gravité : haute** (identité visuelle du produit). **Phase 3.1.**

### E2 — Les raccourcis clavier globaux mangent la saisie dans le champ d'adresse

- **Reproduction.** Écrire `http://127.0.0.1:8395` dans le champ, touche par
  touche, en relevant la valeur du champ et le titre de l'écran.
- **Mesure (1280×720, français), `k` non inséré, `Espace` non inséré, `l` change
  la langue :**

  | Touche | Insérée dans le champ | Effet observé |
  |---|---|---|
  | `k` | **non** | rien de visible ; le gestionnaire global l'a consommée (`toggle()`) |
  | `f` | oui | bascule plein écran appelée en plus |
  | `m` | oui | son coupé en plus |
  | `c` | oui | menu des pistes basculé en plus |
  | `l` | oui | **« Choisir un film » devient « Choose a film »**, `html lang` fr→en |
  | `Espace` | **non** | lecture/pause au lieu d'une espace |

- **Cause.** `App.svelte:297` pose `onkeydown={onKey}` sur `<svelte:window>`,
  et `onKey` (`App.svelte:266-294`) ne teste nulle part si l'événement vient
  d'un champ de saisie. Une adresse contenant un `k`, un `l` ou une espace est
  donc soit tronquée, soit en train de changer la langue du lecteur pendant
  qu'on la tape.
- **Note d'honnêteté.** Le plan demandait de tester « les raccourcis globaux dans
  le champ d'adresse et le menu des pistes ». Le champ est mesuré ci-dessus. Le
  menu, lui, se comporte correctement : ouvert, une flèche ne le ferme pas et ne
  déclenche pas de seek, `Échap` le ferme et rend le focus au bouton (mesuré).
- **Preuve.** `before/probe2/probe2-osd-1280x720-noconnect.json`
  (`addressFieldKeyboard`).
- **Gravité : haute** (empêche de se connecter en tapant une adresse).
  **Phase 2.3.**

### E3 — À 320×180, la barre de titre et la barre de lecture interceptent le bouton « Se connecter »

- **Reproduction.** La fenêtre Tauri autorise `minWidth: 640`, `minHeight: 360`
  (pixels **physiques**). Sur cette machine à 200 %, cela fait **320×180 pixels
  CSS** : une taille que l'utilisateur peut réellement atteindre en redimensionnant.
  À cette taille, Playwright refuse le clic pendant 30 s :
  « `<header class="title-bar">` intercepts pointer events », puis
  « `<div class="scrub">` … intercepts pointer events ».
- **Mesure.** À 320×180 : le formulaire de connexion descend jusqu'à
  `bottom = 311` dans une fenêtre de 180 ; le champ et le bouton sont hors du
  cadre. En lecture, `scrollWidth = 366` dans une fenêtre de 320 : la barre de
  contrôle déborde et **le bouton de fermeture est coupé** (planche
  `before/osd-extremes/osd-playing-320x180-fr.png`).
- **À ne pas confondre.** À 375 et à 704, aucun débordement ; le harnais a donc
  raison sur les tailles qu'il dessine. Le trou est **entre** les tailles
  dessinées et la taille minimale que le produit autorise.
- **Fichier probable.** `player/theia-player/tauri.conf.json` (`minWidth`/`minHeight`)
  ou la mise en page à très faible hauteur ; l'arbitrage appartient à la phase 4.
- **Preuve.** `before/osd-extremes/measure-osd.json`, `logs/measure-osd-extremes.log`.
- **Gravité : moyenne** (aucune fonction perdue pour qui reste au-dessus de
  640×360 CSS ; mais la fenêtre peut y descendre). **Phase 4.1.**

### E4 — La barre de progression est la seule cible sous 44 px, et la charte se contredit

- **Mesure.** OSD : `.scrub` fait **502×24** à 550, **1177×24** à 1280,
  **1766×24** à 1920 — la seule cible sous le plancher, dans tous les états et à
  toutes les tailles. Toutes les autres cibles de l'OSD sont à 52×52 et le
  bouton principal à 52×52. Web, même élément : `.scrub` fait **3rem = 48 px**
  (`web/src/app.css:3128-3135`), au-dessus du plancher.
- **Cause.** `player/ui/src/osd.css:97-103` : `height: 24px`, avec un commentaire
  qui cite 6b. `docs/design-system.md` se contredit : §6b écrit « its hit area is
  24px », §9 écrit « Every interactive target is at least 44×44px ». La ligne
  peinte de 4 px est identique des deux côtés ; seule la zone sensible diffère.
- **Preuve.** `before/osd-native-viewport/measure-osd.json`,
  `before/web-measure/measure-web.json`.
- **Gravité : moyenne** (confort à la souris, vraie gêne au doigt et à la
  télécommande). **D3b, puis phase 2/3.**

### E5 — À 550×350, la connexion se replie et le panneau de bibliothèque défile

- **Mesure.** À la taille réelle de la fenêtre native (550×350) : le champ
  d'adresse fait 346×54 et le bouton « Se connecter » **147×54 à x = 378,94** —
  il ne tient plus dans la même ligne et passe dessous, à droite ; « Chercher un
  serveur » fait 204,67×52. Le panneau `.library` a `overflow-y: auto` et une
  boîte de 110 px de haut pour un formulaire de 135 px : la barre de défilement
  visible dans la capture native vient de là, pas du document
  (`scrollHeight == clientHeight`).
- **Preuve.** `before/osd-native-viewport/measure-osd.json`,
  `before/10-native-connect-1100x700-fr.png`.
- **Gravité : faible à moyenne** (esthétique et confort). **Phase 4.1.**

### E6 — Le web préservé déborde à 550×350, la taille de la fenêtre native

- **Mesure.** `/films` à 550×350 : `label.library-select` va jusqu'à
  `right = 696,39` et son `select` jusqu'à `682`, dans une fenêtre de 550. Aucun
  débordement à 1280×800 ni à 1920×1080. À 550, les cartes tombent bien à deux
  colonnes de 245,41 px, ratio 1,78.
- **Pourquoi cela compte.** 550×350 est exactement ce que donne la fenêtre
  native de 1100×700 sur un écran à 200 %, c'est-à-dire la configuration de
  M. Berthier. C'est la même classe de défaut que celui déjà corrigé dans l'OSD
  (les contrôles qui tombaient à 44rem), mais du côté du web.
- **Réserve de méthode.** Découvert par notre pilote ; `web/tests/layout.spec.js`
  ne dessine que 375, 1280 et 1920, donc ce n'est pas une régression, c'est un
  angle mort. Le web est gelé : le corriger est une décision, pas une évidence.
- **Preuve.** `before/web-measure/measure-web.json`,
  `before/web-measure/web-films-550x350-fr.png`.
- **Gravité : moyenne.** **Décision requise (voir questions).**

### E7 — Un clic sur un contrôle peut être avalé quand la zone sensible en recouvre un autre

- **Reproduction.** Le clic sur « Se connecter » échoue à 320×180 parce que
  `.scrub` (24 px de haut, `pointer-events` actif, `role="slider"`) recouvre la
  zone du bouton. Playwright nomme l'élément qui intercepte.
- **Portée honnête.** Observé à 320×180 ; **non reproduit** à 375, 550, 704,
  1280 ni 1920. C'est donc le même défaut que E3 vu par sa conséquence, et il
  n'est pas prouvé à une taille que quelqu'un utiliserait.
- **Preuve.** `logs/measure-osd-extremes.log`.
- **Gravité : faible** en l'état, **à revoir** après E3. **Phase 4.1.**

### E8 — La timeline 24 px de la charte contre les 44 px du §9, et le rectangle doré

Ce ne sont pas des mesures mais des contradictions de document, et elles
demandent un arbitrage avant toute correction :

1. **Cartes.** Le préambule V3.1 (« It does not draw a gold rectangle around the
   card ») et §6.2 (« Gold appears on hover and focus as a 1px `--accent`
   border ») disent la même chose ; l'ancien §6.2 cité par le plan disait
   l'inverse. L'OSD applique bien la bordure 1 px **et** l'anneau de focus 2 px
   (`osd.css:435-453`), donc il suit déjà le texte le plus récent.
2. **Timeline.** §6b dit 24 px, §9 dit 44 px, le web fait 48 px, l'OSD fait
   24 px. Deux surfaces du même produit ne peuvent pas continuer à répondre
   différemment à la même question.
- **Preuve.** `docs/design-system.md` §6.2, §6b, §9 ; `osd.css:97-103` ;
  `app.css:3128-3135` ; mesures M-01 à M-03.
- **Statut : en attente de décision (D3a, D3b).**

### E9 — Le rendu de l'OSD n'est pas reproductible pour un état sur sept

- **Mesure.** Deux exécutions identiques de `check:render` : six planches sur
  sept sont identiques au bit près ; `5-phone-library.png` fait **65 785** puis
  **57 267** octets (l'édition du dépôt en fait 62,4 Ko). L'état concerné est la
  bibliothèque à 390 px juste après connexion.
- **Conséquence.** Une comparaison avant/après qui s'appuierait sur cette planche
  conclurait à un changement qui n'en est pas un. Les six autres sont utilisables.
- **Hypothèse, non vérifiée.** Chargement paresseux des visuels de cartes à
  390 px, ou ordre d'arrivée des réponses. À confirmer en phase 1 avant d'en
  faire une assertion.
- **Preuve.** `before/osd/` contre `before/osd-run2/`, `logs/check-render-run2.log`.
- **Gravité : faible pour le produit, réelle pour la méthode.** **Phase 1.2.**

### E10 — La capture terminal ne photographie pas la fenêtre qu'elle prétend

- **Mesure.** `capture-tui.ps1` place la console à `40,40` pour `colonnes×10+30`
  de large. À 100 colonnes la fenêtre fait ~1030 px dans un écran de 1440 :
  la capture plein écran (1440×900) contient la console **et le navigateur
  derrière**. À 60 colonnes, la fenêtre calculée fait 630 px mais la capture
  montre la console décalée hors du bord gauche, texte coupé.
- **Ce qui reste utilisable.** Le texte du tampon, relu par la sonde, est
  complet : la mise en page est donc jugeable indépendamment du cadrage.
  À 60×24 le texte se replie proprement (« …puis c'est fini. Rien n'est /
  écrit avant la confirmation. ») et à 100×30 comme à 80×24 il tient sur deux
  lignes. Aucune valeur n'est masquée.
- **Preuve.** `before/00-setup-fr-100x30.png`, `80x24`, `60x24` ;
  `logs/tui-captures.log`.
- **Gravité : faible** (outil d'observation, pas produit). **Phase 5.3.**

### E11 — Le lecteur n'a pas encore été vu en train de jouer dans sa vraie fenêtre

- **Ce qui est vérifié.** La fenêtre s'ouvre, le moteur épinglé se charge, DPI
  192 détecté, capture de l'écran de connexion obtenue (`N-01`).
- **Ce qui ne l'est pas.** Un film joué dans cette fenêtre. La fixture disponible
  dure **45 s** et le plan demande une mesure de 60 s ; il n'y a pas de média
  réel autorisé. C'est la limite que la phase 4.3/4.4 doit lever.
- **Preuve.** `logs/baseline-NATIVE-CAPTURE.log`, `internal/testfixture/main.go:50`
  (`-t 45`).
- **Statut : non vérifié**, et la question D5 existe pour cela.

### E12 — Le curseur natif reste non observé

- **Ce qui est mesuré.** Le CSS : `cursor: none` est posé sans condition sur
  `html, body` (`osd.css:10-22`) ; les boutons, le scrub, les pistes et les
  cartes le rétablissent (`cursor: pointer`, `pointer` → 6 endroits) ; le champ
  d'adresse prend `text`. L'état `data-idle` ne change pas le curseur — il le
  laisse déjà à `none`.
- **Ce qui ne l'est pas.** Si le pointeur est visible ou non dans la vraie
  fenêtre. `capture-window.ps1` photographie l'écran : `CopyFromScreen` ne
  dessine pas le curseur, donc **son absence du PNG ne prouve rien**. Les dix
  cycles mouvement → repos → réveil de la phase 2 ne sont pas faits.
- **Preuve.** `before/10-native-connect-1100x700-fr.png`, `osd.css:21`.
- **Statut : non vérifié.** **Phase 2.1/2.2, avec D2.**

### E13 — Un média de plus d'une minute et un cache TMDB manquent pour les mesures prévues

- **Fixture.** `go run ./internal/testfixture` fabrique 4 films et 1 série,
  jouables, 45 s chacun ; c'est fait, la bibliothèque de test existe
  (`/api/library/stats` : 3 films, 1 série, 1 épisode, 0 problème).
- **Bench.** `scripts/bench -count 250` **exige**
  `bench-data/cache/images` non vide ; il n'existe pas sur cette machine. Un
  banc à 250 films avec visuels synthétiques est possible, mais les affiches
  réelles resteront non vérifiées. **Non lancé**, et il ne faut pas le présenter
  comme une recette à froid.
- **Média réel.** Aucun dossier autorisé **au moment des mesures de cette
  phase** ; `D:\` n'existe pas. **Réponse reçue après la campagne (D5 = B) :**
  `C:\Users\starx\Documents\Films`, vérifié en lecture seule et **jamais
  ouvert** — 11 fichiers `.mkv`, 118,2 Go, soit exactement la bibliothèque déjà
  décrite dans `docs/v3.3.md:392-426` (le remux 2160p Atmos de 53,79 Go et les
  10 épisodes de *Shogun*). La limite tombe pour la suite ; elle reste vraie
  pour tout ce qui a été mesuré avant cette réponse.
- **Gravité : méthode.** **Phases 4.4 et 6.4** — débloquées par D5 = B.

### E14 — Ce que la phase 0 ne pouvait structurellement pas comparer

À dire plutôt qu'à combler par une approximation :

- **Écran de lecture web contre OSD.** Le player web exige un fichier réel à
  ouvrir ; l'OSD se dessine à partir d'une trame de statut simulée. Les deux
  n'ont pas été mis dans le même état de lecture. Ce qui **a** été comparé à
  contenu identique : les grilles de cartes, la typographie calculée, les cibles,
  les couleurs et les débordements, sur la même bibliothèque issue du même
  serveur (3 films + 1 série), aux mêmes largeurs.
- **Natif contre web.** Chromium ne prouve ni WebView2, ni libmpv, ni le HDMI.
  La composition réelle des pixels au-dessus du film reste la validation de
  M. Berthier (`docs/v3.3.md`, risques ouverts 1).
- **TUI contre web.** Aucun équivalent : ce sont deux surfaces différentes, la
  phase 5 les compare entre elles, pas au web.

---

## 4. Ce que la comparaison à contenu identique a établi

Bibliothèque identique (les 4 titres du serveur de test), mêmes largeurs, même
machine, Chromium des deux côtés. Le détail est dans `mesures.csv`.

| Ce qui est comparé | Web (préservé) | OSD (natif) | Verdict |
|---|---|---|---|
| Police du titre de panneau | Cinzel Variable, **chargée** | Georgia, **repli** | **écart structurel** (E1) |
| Police des étiquettes | Jost Variable, **chargée** | Segoe UI, **repli** | **écart structurel** (E1) |
| Police du titre de carte | `--font-ui` (pile système) | `Arial` (même pile, résolue) | conforme |
| Taille du titre de carte, 1280 | 14 px | 14 px | conforme |
| Taille du titre de carte, 1920 | 19 px (palier 100 rem) | non mesuré à 1920 | à compléter |
| Ratio des cartes | 1,78 | 1,77 (16/9 = 1,7778) | conforme |
| Colonnes, 1280 | 4 × 279 px | 5 × 219 px | densité différente, **assumée** (§6.1 : le panneau OSD a son propre plancher de 12 rem) |
| Colonnes, 550 | 2 × 245 px | 2 × 244 px | conforme |
| Zone sensible de la timeline | 48 px | 24 px | **écart** (E4) |
| Cibles sous 44 px | aucune | la timeline seule | **écart** (E4) |
| Débordement horizontal | oui à 550 (E6) | non, sauf à 320 (E3) | deux défauts distincts |
| Règle de progression | présente, film entamé | présente, film entamé, 2,81×3,04 px | conforme |
| `lang` du document | `fr` | `fr`, bascule en `en` pendant la saisie (E2) | **écart** (E2) |

---

## 5. Comportement observé qui n'est pas un écart

- **Minuterie d'inactivité.** À 2,6 s : meubles visibles, `data-idle=false`.
  À 3,8 s : `data-idle=true`, opacité 0. Sur pause après 2,6 s : meubles
  toujours là, opacité 1. Conforme à §6b, aux trois tailles testées.
- **Menu des pistes.** 5 lignes, 2 coches, aucune piste nommée par une URL ni
  par `?profile=`, la piste externe dit « FICHIER EXTERNE ». Flèche : le menu
  reste ouvert, aucun seek. `Échap` : le menu se ferme, le focus revient au
  bouton.
- **Cibles de l'OSD.** Toutes à 52×52 ou plus, sauf la timeline (E4) ; le bouton
  principal à 52×52, au-dessus du plancher de 44 et sous les 56 px « principaux »
  de §9 — à trancher avec D3b plutôt qu'à corriger en silence.
- **Formulaire d'installation.** Aucune écriture avant confirmation : après six
  captures, `%APPDATA%\Theia\setup.json` porte toujours son horodatage du
  15/09 22:42:49 et les deux répertoires isolés sont vides.
- **Couleurs du terminal.** Or `#C19C00` pour l'accent (l'entrée la plus proche
  de `#C8A24A` dans la palette héritée de seize), corps `#CCCCCC`, aide
  `#767676`, fond `#0C0C0C`. La sonde n'a décrit que trois cellules : la palette
  complète reste **partiellement** mesurée, et `print…` / le résumé n'ont pas été
  atteints (le formulaire a été quitté par `Échap`, comme prévu).

---

## 6. Limites de cette phase, dites franchement

1. Aucun résultat natif sur pixels : ni film derrière l'OSD, ni curseur, ni
   plein écran, ni HDMI, ni Atmos (E11, E12).
2. Les preuves OSD sont **simulées** (`window.__TAURI__` remplacé, mêmes trames
   de statut que le harnais existant). Elles disent ce que la page dessine, pas
   ce que WebView2 compose.
3. La bibliothèque de comparaison est **synthétique** (4 titres, aucune affiche
   TMDB : le cache d'images est vide). Les cartes ont été dessinées avec les
   visuels de la sonde et le repli texte, ce qui est un état réel du produit,
   mais aucune affiche réelle n'a été jugée.
4. `go test` a tourné en partie sur cache ; `cargo test` est frais. Les durées
   sont celles observées, pas des références de performance.
5. La suite `L` a tourné sur **8396** (sa valeur par défaut), pas sur 8395. La
   suite `P` n'a pas tourné : elle code 8397 en dur, et c'est D0.
6. Le contrôle des polices par `document.fonts.check()` s'est révélé
   trompeur (il répond `true` pour une famille jamais déclarée). Toute la
   campagne s'appuie donc sur les `@font-face` présentes, les fichiers émis et
   la famille résolue par le style calculé.

---

## 7. Prochaine action si la phase 1 est autorisée

1. Poser D0 à D5 (voir `decisions-benjamin.md`) ; D1 à D3 ne bloquent que leur
   propre correction, D0 bloque la suite `P`, D5 bloque la validation réelle.
2. Phase 1.2 : ajouter au harnais `render-check.mjs` les assertions qui
   **échouent** aujourd'hui — `@font-face` déclarée et fichier effectivement
   chargé (pas `fonts.check` seul), cible de la timeline, touches `k`/espace/`l`
   dans le champ d'adresse, `minWidth` × 2 atteint. Chaque assertion doit être
   vue rouge avant d'être crue verte.
3. Ne rien corriger avant que D1 à D3 soient tranchées : elles décident *quoi*
   corriger.
