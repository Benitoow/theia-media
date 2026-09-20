# Journal de campagne — phase 0

Répertoire : `C:\Users\starx\AppData\Local\Temp\theia-v33-polish-20260915-231530`
Dépôt : `C:\Users\starx\Documents\CODE\Theia`, HEAD `249532e`, branche `main`.
Toutes les commandes natives ont été suivies de `$LASTEXITCODE` ; aucun échec
n'a été contourné.

---

## 0.1 — Cadre et inventaire — 23:15 → 23:22

- **Fichiers lus en entier :** `CLAUDE.md`, `AGENTS.md`,
  `docs/spec-fondatrice.md`, `docs/design-system.md`, `docs/v3.3.md`,
  `build.ps1`, `build-player.ps1`, `scripts/capture-tui.ps1`,
  `scripts/capture-window.ps1`, `player/ui/package.json`, `web/package.json`,
  `web/playwright.config.js`, `web/playwright.playback.config.js`,
  `web/tests/serve.mjs`, `web/tests/serve-playback.mjs`,
  `internal/testfixture/main.go`, `player/ui/scripts/render-check.mjs`,
  `player/ui/src/App.svelte`, `player/ui/src/osd.css`,
  `player/ui/src/lib/catalogues.js`,
  `player/ui/src/components/{FilmCard,TrackMenu}.svelte`,
  `player/theia-player/{tauri.conf.json,src/mpv.rs,src/main.rs}`,
  lus partiellement là où c'est indiqué.
- **Commandes.** `git rev-parse HEAD`, `git status --short`,
  `Get-NetTCPConnection -LocalPort 8383,8395,8397,5199`, inventaire machine.
- **Codes de sortie.** Tous 0. Aucun processus Theia en écoute au départ ;
  8383 libre, 8395 libre, 8397 libre, 5199 libre.
- **Résultat.** Faits confirmés tels que le plan les annonçait :
  `%APPDATA%\Theia-AVANT-TEST-20260915-2236` existe (modifié 15/09 22:27),
  `%LOCALAPPDATA%\Programs\Theia` existe, `D:\` n'existe pas. Machine :
  Windows 11 26200, écran 1440×900 à **192 ppp (200 %)**, un seul moniteur.
- **Incertitudes.** Le plan dit « un dossier ne prouve pas une installation
  fonctionnelle » : c'est exact, et nous n'avons ni lancé ni inspecté
  l'installation personnelle. `Get-Process *theia*` ne renvoie rien.
- **Action suivante.** 0.2.

## 0.2 — Protection des surfaces gelées — 23:16

- **Commande.** Boucle `git ls-files` + `Get-FileHash -Algorithm SHA256` sur
  `web/**`, `cmd/theia-server/**`, `internal/**` sauf `internal/setup/`,
  `player/**`, plus les manifests et scripts de construction.
- **Résultat.** 313 fichiers hachés dans `before/frozen-surfaces.csv`.
  `git diff --binary` vide, `git status --porcelain` = 0 ligne,
  `git diff --check` exit 0.
- **Preuve.** `before/frozen-surfaces.csv`.
- **Action suivante.** 0.3.

## 0.3 — Baseline — 23:15 → 23:18

Sept travaux lancés, chacun avec son journal dans `logs/`. Détail complet dans
`diagnostic.md` §1. Résumé : **tout est vert**, aucun échec initial à garder
visible. `theia-server.exe` construit (B), OSD construit (O), planches de
référence écrites (R), lecteur natif construit et testé (N), suite de mise en
page 81/84 avec 3 ignorés (L), `theia-setup` construit et photographié.

- **Décision de méthode.** Les travaux longs sont allés en arrière-plan pendant
  la lecture des documents, mais aucun n'a été lancé deux fois de front :
  `build-player.ps1` et `npm ci` se disputent des fichiers verrouillés.
  L'ordre O → N a été respecté.
- **Incertitude.** `go test` a répondu en partie depuis le cache ; les paquets
  frais (`internal/setup`) ont bien été recompilés et exécutés.
- **Action suivante.** 0.4.

## 0.4 — Comparaison web / OSD — 23:18 → 23:45

- **Démarrage du serveur de test.** Port 8395 vérifié libre, PID 18256 possédé
  et enregistré dans `server.pid`. `GET /api/health` → `{"status":"ok",...}`.
- **Bibliothèque isolée.** `go run ./internal/testfixture --data-dir
  <runDir>\playback-data` : FFmpeg épinglé téléchargé et vérifié
  (`sha256=546580347aa7…`, 87,9 Mo), 4 fichiers fabriqués, scan
  `found=4 added=4 problems=0` en 15,7 ms. **Données 100 % synthétiques**,
  aucun dossier personnel touché.
- **Pilotes écrits hors du dépôt** dans `probes/` : `measure.mjs` (typographie
  calculée, cibles, débordements, minuterie) et `probe2.mjs` (géométrie carte
  par carte, clavier, audit des CSS construits). Ils empruntent Playwright à
  `web/node_modules` plutôt que d'en installer un second. **Aucun fichier du
  dépôt n'a été touché.**
- **Trois erreurs de mesure commises et corrigées**, notées parce qu'elles
  auraient produit de faux constats :
  1. une requête agrégée `.film-art` a mesuré l'élément plein cadre du lecteur
     et non une carte — d'où une « carte de 1280×720 » qui n'existait pas ;
  2. une boucle clavier a été écrite **après** la connexion, alors que la
     connexion retire le champ d'adresse du DOM : le pilote a expiré sur un
     champ que le produit avait eu raison d'enlever ;
  3. le sélecteur de carte a été écrit `.films > .film` alors que `FilmCard`
     rend `<li><button class="film">`.
  Chaque correction est dans le code du pilote, pas dans le produit.
- **Résultat.** 9 écarts mesurés (E1 à E9, E11 à E14) ; `mesures.csv` porte
  60 lignes de mesures avec leur preuve.
- **Action suivante.** 0.5.

## 0.5 — Natif et terminal — 23:17 → 23:52

- **Capture native.** `scripts/capture-window.ps1 -Exe
  player\target\debug\theia-player.exe -Arguments '--mute --diagnostics'
  -Width 1100 -Height 700` : pid 36040, DPI 192, fenêtre 1100×700 réels
  = **550×350 CSS**. libmpv épinglé fourni par `THEIA_LIBMPV` (digest vérifié).
  Le captureur ferme le processus lui-même ; `Get-Process theia-player` ne
  renvoie plus rien après coup.
- **Capture terminal.** `capture-tui.ps1` à 100×30, 80×24 puis 60×24, avec
  `-Probe`. La sonde a lu le tampon et décrit trois cellules de couleur
  (`#C19C00` or, `#CCCCCC` corps, `#767676` aide, fond `#0C0C0C`).
  `ConsoleProbe.cs` existe bien dans TEMP (7 905 octets) — vérifié avant usage,
  comme le plan l'exige.
- **Aucune écriture système.** Après les trois captures,
  `%APPDATA%\Theia\setup.json` porte toujours 22:42:49 et les répertoires
  `setup-data` / `setup-install` sont vides. Le formulaire est sorti par
  `Échap`, jamais par « oui ».
- **Limites.** Pas de film joué dans la fenêtre native (fixture 45 s, plan
  demande 60 s) ; curseur invisible sur la capture pour une raison d'outil, donc
  **non vérifié** et non « absent ».
- **Action suivante.** 0.6.

## 0.6 — Rédaction — 23:52 → 00:10

`diagnostic.md`, `mesures.csv`, `decisions-benjamin.md`, ce journal. Aucun
correctif produit. Aucun commit. Aucun envoi.

---

## Vérifications de fin de campagne

- `git status --short` vide ; `git diff --binary` vide ; `git diff --check` 0.
- Les 313 empreintes de `before/frozen-surfaces.csv` ont été recalculées :
  **identiques** (voir `after/verification.md`).
- Seuls nos PID ont été arrêtés : serveur de test 8395 (18256) et
  prévisualisation OSD 5199 (18164), chacun nommé, vérifié et arrêté
  individuellement. `Stop-Process -Name` n'a jamais été employé.
- Aucune donnée personnelle ouverte, aucune application installée ou
  désinstallée, aucun port personnel touché (8383 jamais occupé).
