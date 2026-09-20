# Journal — l'enquête sur le pointeur, 16 septembre 2026

Suite de `journal-phase4-3.md`. Dépôt `C:\Users\starx\Documents\CODE\Theia`.
M. Berthier a demandé de chercher la cause plutôt que de la consigner.

## Le constat de départ

Le pointeur revenait pendant l'inactivité. Mesuré la veille : `GetCursorInfo`
rapportait « showing » dans **35 échantillons sur 37**, caché aux indices 6 et 29.
Le mécanisme natif (`player_set_cursor` → `ShowCursor`) était appelé avec les bons
arguments. La cause était inconnue.

## Ce que j'ai écarté, mesure par mesure

**1. Windows ne garde-t-il pas le pointeur caché ?** Sonde
`probes\cursor-persist.ps1` : `ShowCursor(FALSE)` renvoie **−1** (le compteur du
fil a bien été décrémenté), et **59 échantillons sur 59** lisent malgré tout
« showing ». Donc le compteur du fil n'est pas ce que `GetCursorInfo` rapporte :
`GetCursorInfo` lit l'état de la file d'entrée qui possède la fenêtre active.
Conclusion : chercher côté fil n'aurait rien donné, et c'est cette mesure qui l'a
dit avant que je perde une heure à instrumenter Rust.

**2. Le CSS est-il seulement appliqué au bon élément ?** C'est la question qui a
tout donné. `.osd[data-idle='true']` déclarait `cursor: none` et
`getComputedStyle` répondait bien `none`. Mais `.osd` porte
`pointer-events: none` : Chromium résout le curseur contre **l'élément réellement
sous le pointeur**, et cet élément était `body`, qui répondait `auto`.

Chaîne mesurée à 1280×720, film en lecture, meuble caché
(`probes\cursor-target.mjs`) :

| | avant | après |
|---|---|---|
| élément sous le pointeur | `body` | `body` |
| curseur résolu de cet élément | **`auto`** | **`none`** |
| `.osd` | `none` | `none` |
| `html` | `auto` | `none` |

**Une règle de curseur posée sur un élément qui ne peut pas être survolé ne
s'applique jamais.** Le défaut était dans le produit, pas dans WebView2.

## Pourquoi aucune vérification ne l'avait vu

L'assertion existante demandait : « le meuble a disparu, et `html` **ou** `.osd`
vaut `none` ». Les deux moitiés étaient vraies séparément — le meuble disparaissait,
et `.osd` valait bien `none` — et la seule question qui compte, « quel curseur
voit la personne qui pointe », n'était jamais posée. C'est exactement la forme de
défaut que la phase 0 reprochait au reste du chantier : une assertion qui décrit
le code au lieu de décrire l'expérience.

## Le correctif

La règle est passée sur la racine, là où la résolution aboutit :

```css
html:has(.osd[data-idle='true']) body,
html:has(.osd[data-idle='true']) {
	cursor: none !important;
}
```

Portée par le même attribut `data-idle` que celui qui énonce déjà l'état, pour
qu'aucune seconde source de vérité n'apparaisse. `!important` est nécessaire :
`cursor: pointer` est posé sur chaque commande plus bas dans le fichier, et trois
boutons sont enfants de `.osd` — ils sont de toute façon inertes à l'inactivité
(`pointer-events: none` deux règles plus bas), donc aucun curseur atteignable
n'est retiré.

**L'assertion a été renforcée dans le même commit**, et sa valeur a été prouvée en
remettant l'ancienne règle :

```
the furniture hid but the element under the pointer (body) resolves "auto"
  - declared none: html=auto body=auto osd=none
1 render check(s) failed
```

Elle échoue donc bien sur le défaut qu'elle aurait dû attraper la première fois.

## Le résultat, mesuré sur la vraie fenêtre

Sonde `probes\cursor-native.ps1`, film de 180 s en lecture, pointeur posé **une
fois** au milieu de l'image puis plus jamais touché (chaque `SetCursorPos` est un
vrai mouvement de souris, et la règle d'inactivité réagit aux mouvements), fenêtre
au premier plan sans clic (un clic sur l'image **met en pause** — l'erreur commise
en phase 4.3) :

| | avant | après |
|---|---|---|
| échantillons | 37 | 48 |
| pointeur caché | **2** | **15** |
| pointeur affiché | 35 | 33 |
| position du pointeur | inchangée | inchangée (720,450 dans les 48) |

Deux handles alternent : `65539` (la flèche) et `0` (aucun curseur). La chronologie
n'est plus un clignotement mais une bascule à cadence régulière : affiché sur les
échantillons 10 à 16, caché 17-18, affiché 19-37, caché 38-48 — soit une transition
toutes les 4 à 5 secondes. Le pointeur n'a pas bougé, donc ce n'est pas un
mouvement qui relance la minuterie.

## Ce qui reste, et ce que je propose

Le pointeur est maintenant caché dans le cas dominant — trois fois plus
d'échantillons cachés — mais il revient encore par intermittence. Ce qui reste est
côté plateforme : WebView2 réapplique son propre curseur, et le masquage natif du
produit est appliqué depuis le fil qui exécute la commande IPC, pas depuis la
boucle de messages de la fenêtre.

**La suite est une décision, pas une sonde.** La réponse Win32 classique est de
traiter `WM_SETCURSOR` dans la procédure de la fenêtre et d'y répondre
`SetCursor(NULL)` tant que le meuble est inactif : ce message est celui par lequel
Windows demande au propriétaire de la fenêtre quel curseur afficher, et y répondre
prime sur ce que le webview enfant demande. Cela demande une sous-classe de
fenêtre et du code `unsafe` dans `player/theia-player/src/main.rs` : c'est un
changement produit, donc il attend un arbitrage. L'autre voie, sur la table, est
de **retirer le masquage natif** et d'assumer un pointeur visible, puisque le CSS
couvre désormais le cas qu'il peut couvrir.

## Fichiers

- `player/ui/src/osd.css` — la règle déplacée, avec la mesure dans le commentaire.
- `player/ui/src/App.svelte` — le commentaire du `$effect` natif corrigé : il
  affirmait que « le CSS ne suffit pas sur WebView2 », ce qui était faux.
- `player/ui/scripts/render-check.mjs` — l'assertion renforcée.
- `probes\cursor-persist.ps1`, `probes\cursor-target.mjs`, `probes\cursor-native.ps1`.
- `phase4\native43b\cursor-native.json` et `cursor-native-samples.txt`.
