# Journal — phase 4.3 (la fenêtre native, mesurée), 16 septembre 2026

Suite de `journal-phase4.md`. Dépôt `C:\Users\starx\Documents\CODE\Theia`.

## Ce que le plan demande, et ce qui manquait

La phase 4.3 demandait : lancer le binaire natif, capturer la fixture, **constater
le mouvement à deux instants**, voir le film derrière l'OSD, puis le plein écran
et le retour. Le journal précédent disait « partiellement fait » : la lecture était
prouvée, le mouvement et le plein écran ne l'étaient pas.

Les deux le sont maintenant, et la façon dont le premier a failli être manqué est
la partie utile de cette note.

## La sonde, et mon erreur

Sonde : `probes\phase43-native.ps1`. Elle lance le bundle
(`dist\theia-player-windows-amd64\theia-player.exe`, `THEIA_LIBMPV` mis sur le DLL
du bundle), attend une fenêtre qui a une taille, prend le premier plan, laisse
l'OSD s'endormir, puis photographie la fenêtre à deux instants.

**Premier passage, résultat faux : « moved=False ».** Deux captures au SHA-256
identique. Le réflexe est de conclure que le film ne joue pas. C'était mon propre
protocole : pour prendre le premier plan je cliquais au milieu de la fenêtre,
parce que `SetForegroundWindow` est refusé quand un autre processus détient le
premier plan — et **dans ce produit un clic sur l'image est une commande de
lecture**, il met en pause. Le Space qui suivait n'a pas repris la lecture
(focus WebView2), et la sonde a photographié deux fois une image figée. Le
timecode peint dans la fixture le dit noir sur blanc : `00:00:00.825`, image 21,
aux **deux** instants.

Une sonde qui met en pause ce qu'elle mesure puis annonce que rien n'a bougé est
pire que pas de sonde. Corrigé en prenant le premier plan par
`AttachThreadInput` + `BringWindowToTop` + `SetForegroundWindow`, sans aucun clic —
le mécanisme que `scripts/capture-window.ps1` utilise déjà.

## Résultat du passage corrigé

| Mesure | Valeur |
|---|---|
| Fenêtre fenêtrée | **1280×720 réels = 640×360 CSS**, dpi 192 (200 %) |
| Premier plan obtenu | oui, sans clic |
| Instant A | image **21**, `00:00:00.825` |
| Instant B (2 s plus tard) | image **211**, `00:00:08.792` |
| Mouvement | **établi par le timecode du film**, pas par le hash |
| Plein écran | **1440×900** (toute la dalle), timecode **00:00:11.083**, image 266 |
| Retour | **1280×720**, la fenêtre d'origine, exactement |
| Sortie | `Échap` ferme le lecteur, PID disparu, aucun processus laissé |
| OSD au-dessus du film | titre en Cinzel lisible sur l'image, sans cadre ni bordure |

Le mouvement n'est pas déduit d'une différence d'octets — deux images peuvent
différer pour un curseur qui clignote. Il est lu **dans l'image elle-même** : la
fixture peint son timecode et son numéro d'image, et ils passent de 21 à 211. Le
carré violet a quitté le centre pour le coin inférieur gauche. C'est ce que « le
mouvement constaté à deux instants » veut dire.

Le plein écran montre aussi, en passant, que la composition tient : le titre et le
mot THEIA sont dessinés **par-dessus** le film, nets, sans fond opaque.

## La limite, nommée

- **La fixture fait 640×360.** Le plein écran l'agrandit donc par le logiciel du
  moteur : c'est un test de composition, pas de mise à l'échelle d'une vraie
  source 1080p ou 4K. Le remux 53,79 Go de la bibliothèque réelle a été lu par ce
  même lecteur (relevé 6.4), mais pas capturé en plein écran.
- **`--diagnostics` n'atteint pas un fichier quand le binaire est lancé par
  `Start-Process -RedirectStandardError`** : le fichier reste à 0 octet. Un
  binaire GUI n'a pas de handle stderr valide dans ce montage. La preuve de
  lecture vient donc du chemin headless (`cmd /c "... --media ... --diagnostics"`),
  qui écrit bien : `pos` 0,58 → 8,58 sur 16 trames, `passthrough`, `wasapi`,
  `d3d11va`, `gpu-next`, 0 ligne d'erreur. C'est la même sonde que la mesure de
  60 s de la phase 4.4.
- Le plein écran est vérifié **par le rectangle de la fenêtre et par l'image** ;
  il n'a pas été regardé par un œil humain sur un téléviseur.

## Fichiers

- `probes\phase43-native.ps1` (nouveau), `phase4\native43\` :
  `native-instant-a.png` (image 21), `native-instant-b.png` (image 211),
  `native-fullscreen.png` (image 266), `phase43-native.json`.

## Prochaine action

Phase 5 : la hauteur de la fenêtre native n'est pas en cause, mais la teinte et la
respiration de l'installeur restent à l'arbitrage esthétique de M. Berthier.
