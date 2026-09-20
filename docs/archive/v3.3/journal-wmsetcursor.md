# Journal — le crochet `WM_SETCURSOR`, essayé et retiré

17 septembre 2026. Dépôt `C:\Users\starx\Documents\CODE\Theia`.
Suite de `journal-pointeur.md`. M. Berthier avait choisi l'option A.

## Ce que j'ai construit

Un crochet de procédure de fenêtre, tel que demandé :

- `cursor_wndproc`, qui répond `WM_SETCURSOR` par `SetCursor(NULL)` et retourne
  `TRUE` quand le meuble est inactif ;
- `CursorState { pointer_inside, hidden }` en `AtomicBool`, remplaçant le compteur
  `ShowCursor` par un état que la procédure de fenêtre peut lire ;
- `install_window_hook`, installé depuis `player_set_cursor` par
  `window.run_on_main_thread(...)`, avec `SetWindowLongPtrW` et un
  `CallWindowProcW` de repli vers la procédure d'origine.

Le crochet **s'installe** : la trace confirme
`hook installed on 0xe0880: previous=0x7ffd1595cc00`.

## Ce que la mesure a montré

**Le message n'arrive jamais.** J'ai tracé *tous* les messages reçus par la
procédure de la fenêtre Tauri pendant une lecture complète : **15 messages, aucun
`WM_SETCURSOR`**. Signature observée : `0x46` (WM_WINDOWPOSCHANGING), `0x86`,
`0xc21c`, `0x90`, `0x47`, `0x6`, `0x1c`, `0x272`, `0x2`, `0x465`, `0x82`, `0xf`.

J'ai donc étendu le crochet à toute la descendance (`EnumChildWindows` récursif,
plus `WM_PARENTNOTIFY` pour les fenêtres créées après coup). Résultat :
**`EnumChildWindows` depuis l'intérieur du processus ne trouve aucun enfant**, et
aucun `WM_SETCURSOR` n'arrive davantage.

L'arbre réel, énuméré depuis l'extérieur (`probes\window-tree.ps1`) :

```
TOP  hwnd=788604   pid=13596  Tauri Window        1280x720
  CH hwnd=787108   pid=13596  Tauri Window        1280x720
  CH hwnd=854082   pid=13596  WebView2            1280x720
  CH hwnd=1049846  pid=13596  Chrome_WidgetWin_0  1280x720
  CH hwnd=1968142  pid=15652  Chrome_WidgetWin_0  1280x720   <- autre processus
  CH hwnd=1312518  pid=15652  Chrome_WidgetWin_1  1280x720   <- le pointeur est ici
  CH hwnd=657528   pid=11212  Internet Explorer_Server  1280x720
  CH hwnd=2424972  pid=13596  mpv                 1280x720
```

Le pointeur est sur `hwnd=1312518`, un enfant **Chromium appartenant au processus
WebView2 (PID 15652)** — pas au lecteur. Son ancêtre racine est bien la fenêtre
Tauri, mais `WM_SETCURSOR` n'est visiblement pas remonté jusqu'à elle, et un
processus ne peut pas sous-classer la fenêtre d'un autre.

**Résultat mesuré du crochet : 4 échantillons cachés sur 48** — moins bien que le
CSS seul (15 sur 48). Le crochet n'apporte rien et ajoute une sous-classe avec du
`unsafe`.

## Décision : le code natif est revenu à l'état committé

`git checkout -- player/theia-player/src/main.rs`. Aucune instrumentation ne
subsiste (`THEIA_CURSOR_TRACE`, les compteurs, `player_cursor_debug` : tout est
parti). Le lecteur recompile proprement.

Ce qui reste et qui marche est le correctif CSS, committé dans `abc43fe` : la
règle de curseur est sur l'élément où Chromium la résout, et le pointeur est caché
**15 fois sur 48** au lieu de **2 sur 37**.

## Ce que cette tentative apprend, et qui vaut d'être écrit

1. **La fenêtre qui compte n'appartient pas au lecteur.** Le curseur est décidé
   sur `Chrome_WidgetWin_1` dans le processus WebView2. Aucune sous-classe depuis
   le lecteur ne peut l'atteindre : `SetWindowLongPtrW` ne fonctionne que dans son
   propre processus.
2. **`WM_SETCURSOR` n'est pas remonté à la fenêtre racine** sur ce montage, ce qui
   était l'hypothèse de l'option A. L'hypothèse est maintenant mesurée et fausse.
3. **Ce qui reste possible**, et qui n'a pas été essayé :
   - `SetWindowsHookEx(WH_CALLWNDPROC)` avec un DLL injecté — hors de question :
     cela demanderait une DLL native et toucherait tous les processus de la
     session ;
   - `SetCursor(NULL)` en boucle depuis le lecteur — un combat de boucle contre le
     compositeur de WebView2, laid et coûteux ;
   - **retirer le masquage natif et garder le CSS**, c'est-à-dire l'option B, qui
     était l'alternative sur la table et que la mesure rend maintenant la plus
     honnête : le crochet promis ne peut pas fonctionner, et le mécanisme actuel
     (`ShowCursor`) est un compteur de fil qui peut laisser un bureau sans
     pointeur.
4. **La bonne question n'a peut-être jamais été Win32.** Le pointeur revient par
   intermittence *alors que le CSS demande `none`* — donc Chromium lui-même, ou la
   couche qui compose au-dessus de la vidéo, redessine un pointeur. C'est un
   ticket WebView2, pas un défaut que ce dépôt peut corriger par une sous-classe.

## Fichiers

- `player/theia-player/src/main.rs` : **inchangé** (retour à `abc43fe`).
- `probes\window-tree.ps1` : l'arbre des fenêtres, réutilisable.
- Traces : `phase4\native43d\`, `native43e\`, `native43f\`, `native43g\`.
