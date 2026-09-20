# Vérifications de fin de phase 0

Date : 15 septembre 2026, ~00:10 heure locale.
Dépôt : `C:\Users\starx\Documents\CODE\Theia`, branche `main`.

## Surfaces gelées

- `before/frozen-surfaces.csv` : **313 fichiers**, SHA-256 et taille.
- Recalcul intégral en fin de campagne : **0 modifié, 0 manquant**.
- Périmètre haché : `web/**`, `cmd/theia-server/**`, `internal/**` sauf
  `internal/setup/`, `player/**`, `go.mod`, `go.sum`,
  `web/package-lock.json`, `player/Cargo.lock`, `player/libmpv.json`,
  `embed.go`, `build.ps1`, `build-player.ps1`, `build-release.ps1`.

## État Git

| Contrôle | Résultat |
|---|---|
| `git rev-parse HEAD` | `249532e37770c4678fa321ee85de4f92808b7caf` |
| `git status --short` | vide |
| `git status --porcelain --untracked-files=all` | 0 ligne |
| `git diff --binary` | 0 caractère |
| `git diff --check` | exit 0 |

Aucun commit, aucun tag, aucun envoi, aucune écriture GitHub.

## Processus et ports

| PID | Programme | Chemin | Port | Sort |
|---|---|---|---|---|
| 18256 | `theia-server` | dépôt `theia-server.exe`, données isolées | 8395 | arrêté par `Stop-Process -Id 18256` |
| 18164 | `node` | prévisualisation Vite de l'OSD | 5199 | arrêté par `Stop-Process -Id 18164` |
| 36040 | `theia-player` | `player\target\debug` | — | fermé par `capture-window.ps1` |

Après arrêt : **0 écouteur** sur 8383, 8395, 8396, 8397, 5199 ;
**0 processus** dont le nom contient `theia`. 8383 n'a jamais été occupé.
Aucun arrêt par nom, aucun balayage de `conhost`.

## Données personnelles et système

- `%APPDATA%\Theia` : constaté par `Test-Path` et liste de noms, **jamais
  ouvert**. `setup.json` porte toujours son horodatage du 15/09 22:42:49 —
  inchangé par les six captures du formulaire.
- `%APPDATA%\Theia-AVANT-TEST-20260915-2236` : constaté, non ouvert.
- `%LOCALAPPDATA%\Programs\Theia` : constaté, **ni lancé, ni modifié, ni
  désinstallé**.
- Aucune base personnelle ouverte ; la seule base touchée est
  `<runDir>\playback-data\theia.db`, fabriquée par `internal/testfixture` dans
  un répertoire jetable.
- Aucune installation, aucune écriture de registre, aucune entrée de menu.
- `D:\` n'existe pas ; aucun lecteur monté ou démonté.

## Fichiers créés dans le dépôt

Aucun. Les pilotes de mesure (`probes/measure.mjs`, `probes/probe2.mjs`) et
toutes les sorties vivent dans le répertoire de campagne, hors du dépôt.

## Limites assumées

1. Les preuves OSD sont **simulées** : Chromium avec `window.__TAURI__`
   remplacé. Elles ne prouvent pas WebView2.
2. Aucune lecture native n'a été observée dans la vraie fenêtre.
3. Le curseur natif n'a pas pu être observé : `CopyFromScreen` ne dessine pas
   le pointeur.
4. La bibliothèque comparée est synthétique, sans affiche TMDB.
5. La suite playback (`P`) n'a pas été lancée : elle code 8397, c'est D0.
