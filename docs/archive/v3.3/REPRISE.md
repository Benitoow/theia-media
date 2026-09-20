# Note de reprise — 17 septembre 2026, fin de la phase 7

**Les phases 0 à 7 du plan V3.3 sont exécutées.** L'arbre est propre, aucun
processus ne tourne, rien n'a été publié. Le bilan complet est dans
`bilan-phase7.md` ; cette note dit seulement où reprendre.

## Commits locaux — 28, aucun envoi

```
92776cb Record the campaign's own footprint
1aeb697 Record the gestures this campaign moved
f6f6e15 Escape leaves fullscreen before it leaves the film
8694114 Remove the native pointer hide (decision 125)
80fc00e Record that the WM_SETCURSOR hook cannot work here
abc43fe Put the cursor rule where the cursor is resolved
2bef896 Give the clock one number at the window's own minimum (decision 124)
2f6ac19 Verify movement at two instants and fullscreen, in the real window
7fb4c54 Measure the form's width in cells, and stop photographing the wrong window
f8583dd Drive --service, --uninstall and --check through the real binary
d933c70 Check that leaving the installer writes nothing, on a disk
d8d2e00 docs(v3.3): record the real library run through the public API
2ea9c87 Measure the installer's width instead of looking at it
2ba7d1e Withdraw the click fault, and say what the pointer actually does
988db72 Record the click that keeps the furniture up, and assert the simulation half
02bd65a Assert the sentences a person reads when something is wrong
191a971 Record what the pointer actually does, and the language gesture
50db02b The document says which language it is in, from the first paint
3505b7d Assert the clock on a two-hour film, and a title long enough to truncate
bf1ef1d The pointer is hidden at the window, because the stylesheet could not
0b9a9fd The idle timer was restarted by every status frame
9d2f18e One server is not a question, and the guard refused it
742d247 The OSD wears the product's faces, and the timeline is a real target
a58c18a A key press belongs to whatever the viewer is looking at
e72900d Assert the menu's keys and where focus goes when it closes
42eaa91 Never hide the furniture when hiding it takes something away
1237edb The pointer belongs to the furniture, not to the document
7140cfa Assert the four things the OSD check could not see
249532e (avant le chantier)
```

Poussée en retard : `origin/main` est toujours à `7dd4124`, l'état d'avant le
chantier. Rien n'a quitté cette machine.

## Toutes les portes sont vertes

`go test ./...` 0 · `check:render` 0 · `check:i18n` fr 33 / en 33 · `contrast.mjs`
11/11 · `check-locales.mjs` 684 valeurs · `check-backdrops.mjs` 5/5 ·
`svelte-check` 0 erreur 0 avertissement · `npm test` 81 réussis / 3 ignorés ·
`test:playback` 39 réussis / 5 ignorés · `build.ps1` OK ·
`build-player.ps1 -Release -Bundle` OK, moteur revérifié aux deux pins, bundle
extrait à neuf et lancé sans variable d'environnement.

## Décisions prises par M. Berthier

- **320×180** : l'horloge passe à un seul nombre sous 30rem. Appliqué, asserté,
  décision 124. Le débordement a disparu.
- **Fin de film** : dernière image figée, meubles affichés, aucune phrase, aucun
  retour automatique. Décision prise ; G25b mis à jour.
- **Pointeur** : chercher la cause, puis tenter `WM_SETCURSOR`, puis retirer le
  masquage natif. Les trois ont été faits ; décisions 125 et le journal du crochet
  disent ce que la mesure a répondu.
- **Échap en plein écran** : trouvé en jouant la vérification que D2 demandait.
  Un seul Échap fermait le lecteur ; corrigé, mesuré avant/après, décision 126.

## Ce qui reste ouvert, et qui n'est pas du travail de code

1. **L'esthétique de l'installeur** (phase 5.2) : la hiérarchie, la teinte d'accent
   et la respiration attendent l'arbitrage de M. Berthier. La largeur, la palette
   et l'absence de débordement sont mesurées et tenues ; ce qui manque est un
   jugement visuel, et aucune capture cadrée fiable n'a pu être produite sur cette
   machine (le cadre visible est un onglet Windows Terminal).
2. **La série invisible pour la liste de films** — 10 des 11 éléments de la
   bibliothèque réelle. Décision 97 : cela attend une décision de produit.
3. Le plein écran vu par un œil humain, la lisibilité à trois mètres, le
   passthrough HDMI/Atmos, le mDNS réel, le comportement au logon, les autres
   plateformes : tous demandent du matériel que cette machine n'a pas.

## Si le travail reprend

- **Le pointeur** : `player/ui/src/osd.css` (la règle, seule mécanisme depuis la
  décision 125). Le crochet `WM_SETCURSOR` est fermé : la fenêtre qui décide du
  curseur appartient au processus WebView2.
- **Échap** : `player/ui/src/App.svelte`, l'ordre menu → plein écran → film, avec
  l'état tenu depuis `tauri://resize`.
- **Les sondes** sont dans
  `%TEMP%\theia-v33-polish-20260915-231530\probes\` : `fullscreen-escape.ps1`
  (la séquence Échap), `cursor-native.ps1` (le pointeur sur la vraie fenêtre),
  `cursor-target.mjs` (quel élément résout le curseur), `phase43-native.ps1`
  (mouvement, plein écran), `window-tree.ps1` (l'arbre des fenêtres).
- **Avant de reconstruire le lecteur** : arrêter le serveur d'aperçu de l'OSD
  (`p1-preview.pid` dans le dossier de campagne), sinon `npm install` échoue avec
  EPERM sur `rollup.win32-x64-msvc.node`. C'est arrivé deux fois.
- **Le serveur d'aperçu n'écoute que sur `::1`** : l'utiliser par
  `http://localhost:5199/`, pas par `http://127.0.0.1:5199/`.
- **`Start-Process cmd.exe -ArgumentList` avec une redirection ne produit aucun
  fichier.** Écrire la ligne dans un `.cmd` et lancer ce fichier.

