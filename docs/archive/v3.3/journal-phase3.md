# Journal — phase 3 (identité visuelle du lecteur), 16 septembre 2026

Suite de `journal-phase1-2.md`. Dépôt `C:\Users\starx\Documents\CODE\Theia`.

## Commits de cette phase

```
3505b7d Assert the clock on a two-hour film, and a title long enough to truncate
bf1ef1d The pointer is hidden at the window, because the stylesheet could not
0b9a9fd The idle timer was restarted by every status frame
742d247 The OSD wears the product's faces, and the timeline is a real target
```

`render check` passe intégralement (`exit 0`) après chacun d'eux.

## 3.1 — Les fontes — fait

- **Défaut.** L'OSD ne déclarait **aucune** `@font-face` : 0 règle, 0 `.woff2`
  émis, `document.fonts` vide. Titres en Georgia, étiquettes en Segoe UI.
- **Correctif.** Les deux déclarations sortent de `web/src/app.css` — qui
  commence par `@import 'tailwindcss'` et ne peut pas être importé par un
  document sans Tailwind — vers `web/src/lib/fonts.css`, à côté de
  `tokens.css`. Les deux surfaces l'importent.
- **Preuve que c'est un déplacement et non un changement.** Les 41 fichiers de
  `web-dist/_app/immutable/assets` ont le **même SHA-256** avant et après une
  reconstruction complète par `build.ps1`.
- **Dépendances.** `@fontsource-variable/cinzel` et `jost` déclarés par l'OSD :
  les deux mêmes paquets, aux mêmes versions, que le web embarque déjà depuis
  npm. Aucune dépendance nouvelle pour le projet ; signalé à M. Berthier.

## 3.2 — La timeline — fait

- **Décision D3b appliquée.** Zone sensible de 24 → 44 px, ligne peinte
  inchangée à 4 px. `docs/design-system.md` §6b amendé dans le même commit.
- **Mesuré après coup** à 390, 550 et 1280 dans quatre états chacun : aucun
  débordement, **aucune cible sous 44 px**.

## 3.3 — Les cartes — vérifié, aucun correctif nécessaire

Le ratio 16/9, les trois replis (backdrop, poster contenu, titre en texte), la
règle de progression réservée aux films entamés et le focus visible étaient
**déjà couverts** par le harnais et **déjà corrects**. Ce qui manquait a été
ajouté : un titre assez long pour devoir être tronqué, vérifié comme tronqué
(`text-overflow: ellipsis`, `white-space: nowrap`) et ne débordant pas de la
fenêtre. Commit `3505b7d`.

## 3.4 — L'horloge et la barre — vérifié, aucun correctif nécessaire

Le harnais ne testait que le cas court. Ajouté et vérifié sur la version
courante :

| Cas | Attendu | Mesuré |
|---|---|---|
| Film de 10 min | `2:08` / `10:00` | conforme |
| Film de 2 h 20 | `1:25:38` / `2:20:08` | conforme |
| Durée inconnue | `0:12` / `--:--` | conforme |

Le séparateur est bien une règle dessinée et non un glyphe typé, les deux nombres
n'ont pas la même couleur, et la barre ne se replie pas : mesuré à 390, 550 et
1280, aucune cible essentielle perdue. Commit `3505b7d`.

## 2.1 (suite) — Le pointeur — mécanisme posé, persistance NON vérifiée

- **Défaut mesuré.** `cursor: none` n'atteint pas le pointeur de WebView2 : avec
  les meubles manifestement partis et le film seul à l'écran, `GetCursorInfo` a
  rapporté le pointeur **dessiné sur 15 échantillons sur 15**. Chromium honore la
  règle — le harnais l'assertait déjà — WebView2 dessine son propre pointeur
  au-dessus de la surface vidéo.
- **Correctif.** Commande Tauri `player_set_cursor`, appelée par l'OSD quand
  `idle` change. `ShowCursor` est un **compteur** et non un drapeau : un booléen
  garde la trace, sinon un joueur qui perd le compte laisse un bureau sans
  pointeur. Le masquage n'a lieu que si le pointeur est dans le rectangle du
  lecteur, car il est global au processus.
- **Vérifié.** La commande native s'exécute avec les bons arguments et agit :
  trace dans le stderr du lecteur, `hidden=true inside=true wanted=true` quand
  les meubles partent, `hidden=false` à leur retour. Le pointeur a été rapporté
  caché pendant la fenêtre d'inactivité. La moitié OSD est assertée par le
  harnais via une doublure qui enregistre les appels : cache demandé quand les
  meubles partent, retour demandé au réveil, jamais laissé caché en pause.
- **NON vérifié, et c'est écrit dans le commit.** Le pointeur ne reste pas caché
  pendant toute la période d'inactivité. Les traces montrent des cycles
  cache/rend, et `GetCursorInfo` a lu « showing » sur 19 échantillons sur 20 au
  dernier passage. **Le mécanisme fonctionne, sa persistance non, et la cause
  n'est pas établie.** Geste G5b dans `player/PROTOCOL.md`, statut non vérifié.
- **Quatre fautes de mesure corrigées en route**, chacune ayant produit une
  conclusion fausse avant d'être comprise :
  1. le film durait 45 s et mes mesures tombaient **après sa fin** — à la fin
     d'un film le moteur garde la dernière image et rapporte `pause`, donc les
     meubles restent : c'était correct, et mes lectures ne mesuraient rien.
     Corrigé par un clip de 180 s fabriqué avec le FFmpeg épinglé ;
  2. `SetCursorPos` est un vrai mouvement de souris : chaque échantillon
     relançait la minuterie de trois secondes ;
  3. `MainWindowHandle` renvoie une fenêtre interne de Tao en 13×13 : il faut
     énumérer les fenêtres et prendre la classe « Tauri Window » ;
  4. `SetForegroundWindow` est refusé quand un autre processus détient le premier
     plan — une mesure a porté sur le Microsoft Store au lieu du lecteur.
- **Erreur de ma part, à ne pas refaire.** La première instrumentation écrivait
  dans des `$state` **depuis l'effet** : c'est une écriture réactive dans un
  effet, elle bouclait 1000 fois et faisait paraître le produit cassé alors qu'il
  ne l'était pas. L'instrumentation retenue écrit directement dans le DOM.

## Constat à trancher, pas un défaut

**Fin de film.** Le moteur garde la dernière image (`keep-open`) et rapporte
`pause: true` : les meubles restent affichés sur une image figée, sans phrase ni
retour à la bibliothèque. C'est une décision de produit, pas un défaut — geste
G25b, à trancher avec M. Berthier.

## Ce qui reste non vérifié après cette phase

- La persistance du pointeur caché (G5b).
- Le plein écran observé par un œil humain, la lisibilité à trois mètres.
- Le passthrough HDMI/Atmos : cette machine n'a ni amplificateur ni sortie HDMI
  audio. Le refus du bitstream et le repli PCM sont observés, pas l'acceptation.
- mDNS réel ; la suite playback `P` (D0 = A, elle garde son port 8397).
