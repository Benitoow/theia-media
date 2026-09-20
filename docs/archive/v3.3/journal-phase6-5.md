# Journal — phase 6.5 et la séquence Échap en plein écran, 17 septembre 2026

Dépôt `C:\Users\starx\Documents\CODE\Theia`. Suite de `journal-wmsetcursor.md`.

Cette unité a deux parties : fermer proprement la phase 6.5 du plan (périmètre
protégé), et jouer une vérification que le plan demande explicitement et que
personne n'avait faite.

## 6.5 — Périmètre protégé

| Contrôle | Résultat |
|---|---|
| `git diff --check` sur les 25 commits | **exit 0** — aucun espace blanc fautif, aucun marqueur de conflit |
| `web/src` | **2 fichiers touchés**, et c'est l'extraction des fontes |
| `internal`, `cmd` (code produit) | **0 fichier touché** — seuls des fichiers de test |
| `scripts/capture-tui.ps1` | outil de développement, pas du produit |

**L'extraction des fontes est sans effet, et c'est vérifié plutôt qu'affirmé.** Les
deux blocs `@font-face` de `web/src/app.css` à la base (`249532e`) et ceux de
`web/src/lib/fonts.css` aujourd'hui ont été extraits de git et comparés : **24
lignes de part et d'autre, identiques hors espaces**. Le bundle construit contient
bien les deux déclarations, les deux fichiers `.woff2` ont les mêmes empreintes
qu'avant (`DMuuCU8H`, `ObQm3Zd1` dans leur nom), et la seule différence dans
`app.css` est un commentaire réécrit et une ligne `@import` ajoutée avant
`@layer base` — donc l'ordre du bundle est inchangé.

Aucune donnée personnelle n'a servi de bac à sable : le dossier autorisé a été
scanné deux fois dans une base jetable, et vérifié inchangé après coup.

## La séquence Échap en plein écran — un défaut réel

Le plan, décision D2 : « En plein écran, proposer d'abord sa sortie et faire
valider la séquence complète. » La moitié fenêtrée avait été vérifiée ; la moitié
plein écran ne l'avait jamais été.

**Sonde** : `probes\fullscreen-escape.ps1`, écrite pour cette unité. Elle entre en
plein écran par la touche du produit (`f`), lit le rectangle de la fenêtre, envoie
un premier Échap, regarde **si le processus vit encore** et **quel rectangle il
occupe**, puis un second Échap. Le premier plan est pris par `AttachThreadInput`,
sans clic — un clic sur l'image est une commande de lecture dans ce produit.

**Résultat avant correctif, sur la vraie fenêtre :**

```
windowed 1280x720 (dpi 192)
fullscreen 1440x900 is-screen=True
after first escape: running=False rect=
```

**Un seul Échap en plein écran fermait le lecteur.** Le gestionnaire passait
directement de « le menu de pistes est-il ouvert » à `close()`. Sur un téléviseur,
c'est un film perdu — et la position n'est enregistrée que toutes les quelques
secondes.

**Correctif** (décision 126) :

- ordre Échap : **menu, puis plein écran, puis film** ;
- l'état plein écran est tenu depuis `tauri://resize`, seul signal émis par Tauri
  2.11 pour un changement de plein écran, et `isFullscreen()` est **demandé** à ce
  moment-là plutôt que supposé — donc un spectateur qui sort par F11 reste suivi ;
- le menu est testé **avant** le plein écran, parce que `fullscreen` est une copie
  et peut être périmé, alors que l'état du menu est celui du DOM.

**C'est l'assertion qui a imposé cet ordre.** Ma première version testait le plein
écran en premier, et `check:render` a répondu :
`Escape did not close the open track menu while fullscreen`. L'assertion a été
écrite avant d'être crue.

**Résultat après correctif, même sonde, même fixture :**

| | avant | après |
|---|---|---|
| fenêtré, `f` | 1440x900, toute la dalle | 1440x900, toute la dalle |
| après un Échap | **lecteur disparu** | **vivant, revenu à 1280x720** |
| second Échap | inatteignable | ferme le lecteur |
| le film | illisible, il n'existait plus | `pos 0,58 → 4,63`, `pause=false`, `d3d11va`/`gpu-next` |
| image | — | `after-first-escape.png`, timecode à l'image **262**, meuble affiché |

L'assertion du harnais couvre les quatre étapes : `f` entre, un Échap sort sans
fermer, le suivant ferme, et menu ouvert Échap ferme le menu **en restant en plein
écran**.

## Une erreur de sonde, corrigée avant d'être crue

Le premier relevé du moteur affichait `engine_status_lines: 0`. J'ai d'abord lu
« le lecteur ne dit rien », puis j'ai mesuré : `Start-Process -FilePath cmd.exe
-ArgumentList @('/c', "`"$exe`" --version > `"$log`" 2>&1")` **ne produit aucun
fichier**, alors que la même ligne écrite dans un `.cmd` produit 3932 octets. La
redirection passe maintenant par un fichier `.cmd`. C'est la quatrième fois dans ce
chantier qu'une sonde fautive ressemble à un produit fautif, et la seule raison
pour laquelle celle-ci n'a pas été crue est que le relevé était vide — un silence
est plus suspect qu'un chiffre.

## Fichiers

- `player/ui/src/App.svelte` — l'ordre Échap, l'état plein écran, l'écoute de
  `tauri://resize`.
- `player/ui/scripts/render-check.mjs` — le simulacre tient un vrai état de plein
  écran (`isFullscreen`, `setFullscreen`, `close` tracé) et l'assertion des quatre
  étapes.
- `docs/DECISIONS.md` — décision 126.
- `docs/v3.3.md` — la ligne du tableau de vérification.
- `probes\fullscreen-escape.ps1`, preuves dans `phase4\fullscreen-escape{,2,3}\`.

## Prochaine action

Le reste de la liste « en attente » du bilan : l'arbitrage esthétique de
l'installeur, et la série invisible pour la liste de films (décision 97).
