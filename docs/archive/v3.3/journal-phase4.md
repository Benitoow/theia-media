# Journal — phase 4 (fenêtres, langues et vraie vidéo), 16 septembre 2026

Suite de `journal-phase3.md`. Dépôt `C:\Users\starx\Documents\CODE\Theia`.

## Commits de cette phase

```
191a971 Record what the pointer actually does, and the language gesture
50db02b The document says which language it is in, from the first paint
```

`render check` passe intégralement (`exit 0`) après le second.

## 4.1 — Matrice complète des tailles — fait

Huit largeurs × quatre états = **32 combinaisons** mesurées :
320×180, 375×812, 390×780, 480×800, 576×800, 704×396, 1280×720, 1920×1080.

| Résultat | Valeur |
|---|---|
| Cibles sous 44 px | **0**, partout |
| Débordements horizontaux | **1** : 320×180 en lecture et menu ouvert |

**Le débordement de 320×180 est arithmétique, pas un bug.** La rangée dispose de
272 px et contient, après les abattages de §6b : lecture (52) + horloge (106) +
pistes (52) + plein écran (52) + fermeture (52) = **314 px**, plus les
gouttières. §6b interdit deux choses à la fois — replier la rangée, et perdre un
nombre de l'horloge — donc la seule sortie est de réduire quelque chose : la
gouttière à cette largeur, la taille des cibles sous leur plancher de 44 px, ou
l'horloge à un seul nombre. **Arbitrage requis, pas correctif appliqué.**

C'est aussi la taille minimale que la fenêtre accepte sur cette machine
(`minWidth: 640`, `minHeight: 360` en pixels physiques, soit 320×180 CSS à
200 %). Sur un écran à 100 %, la même fenêtre ne peut pas descendre si bas.

## 4.2 — Langues — fait, avec un défaut réel corrigé

- **Défaut mesuré.** En anglais stocké, un chargement neuf dessinait **toutes les
  phrases en anglais** et laissait `document.documentElement.lang` à `"fr"`.
  L'attribut n'était posé que dans `switchLanguage`, donc ni la langue par défaut
  ni le choix stocké ne l'atteignaient — et ce que le document déclare est ce que
  lisent un lecteur d'écran et la césure du navigateur.
- **Correctif.** Un `$effect` maintient `document.documentElement.lang` égal à la
  langue active, ce qui couvre le premier rendu comme chaque changement.
- **Vérifié.** Anglais stocké → rechargement → `lang=en` et copie anglaise ;
  retour au français → `lang=fr`.
- **Deux erreurs de mon test, corrigées avant d'être crues** :
  1. le « redémarrage » était simulé par un **nouveau contexte** de navigateur,
     qui a son propre stockage vide : je demandais à un contexte neuf de se
     souvenir de quelque chose qu'on ne lui avait jamais dit. Un rechargement dans
     le même contexte est ce qu'un redémarrage est ici ;
  2. la vérification lit la copie du catalogue plutôt qu'une chaîne en dur, pour
     rester vraie si une phrase est reformulée.
- **Assertions ajoutées** : la puce change la langue à chaud sans rechargement,
  la copie change avec elle, le choix est stocké, rien ne manque dans l'un ou
  l'autre catalogue à l'écran.

## 2.1 (suite) — Le pointeur — constat précis, toujours ouvert

Mesure de 20 s, film de 180 s en lecture, fenêtre au premier plan, pointeur sur
l'image :

| Fenêtre | Échantillons | Cachés |
|---|---|---|
| 6 s | 15 | 2 (les derniers) |
| 20 s | 49 | 4 |

Le masquage **est appliqué et agit** — `GetCursorInfo` rapporte le pointeur caché
— mais il **clignote** : quatre échantillons cachés sur quarante-neuf, en courtes
séquences plutôt qu'en une longue absence. Quelque chose rend le pointeur pendant
que l'OSD est inactif. La commande native est appelée avec les bons arguments
(tracé dans le stderr du lecteur) ; **la cause du retour n'est pas établie**.

**Erreur de ma part, à ne pas refaire (2e fois).** Mon instrumentation temporaire
déclarait son compteur **après** le premier `$effect` qui l'utilisait : un `const`
dans sa zone morte a levé un `ReferenceError` dans l'effet, l'effet ne tournait
plus et le lecteur paraissait cassé d'une façon nouvelle. Le même piège que la
veille, sous une autre forme — une instrumentation qui change ce qu'elle mesure.
Le code est revenu au commit propre et l'arbre ne porte plus rien de temporaire.

## 4.3 / 4.4 — Natif et vraie vidéo — partiellement fait

- **Vérifié** : le lecteur lance la fixture de 180 s, `vo=gpu-next`,
  `hwdec=d3d11va`, position qui avance, film visible derrière l'OSD, titre en
  Cinzel.
- **Fabriqué pour 4.4** : `playback-data\media\Guard.Long.2026.mp4`, 180 s, avec
  le FFmpeg épinglé — il couvre le besoin de « lecture 60 s » du plan sans média
  réel.

### 4.4 — Lecture de 60 s, mesurée

| Mesure | Valeur |
|---|---|
| Position | 3,6 s → 63,8 s |
| CPU consommé | **3,59 s sur 60 s** (~6 % d'un cœur) |
| Mémoire | 188,6 → 189,3 Mo, stable |
| Audio | `wasapi`, mode `passthrough` |
| Décodage / rendu | `d3d11va` / `gpu-next` |
| stderr | 0 ligne |
| Fermeture | propre, par la fenêtre |

Consigné aussi dans `player/README.md`.

### 4.3 — Mouvement constaté à deux instants

Deux captures à 2–3 s d'écart : **octets différents**, position du moteur qui
avance (`12,83 → 13,83` puis `6,58 → 19,88`), `pause=false`.

### Allégation retirée : « un clic empêche les meubles de partir »

**J'ai écrit ce défaut, puis je l'ai retiré. Il n'existe pas.** Voici comment
l'erreur est née et comment elle est morte, parce que la leçon vaut plus que
l'entrée.

Mes sondes devaient **cliquer** pour prendre le premier plan : `SetForegroundWindow`
est refusé sans geste utilisateur. Or un clic sur l'image **met le film en
pause** — c'est sa fonction — et le §6b **exige** que les meubles restent
visibles en pause. Je mesurais donc un lecteur en pause et j'appelais ça un
défaut. Une touche « reprendre » envoyée depuis le processus de capture
n'atteignait pas la page, ce qui a maintenu la méprise pendant deux rounds.

Séparé proprement, en prenant le premier plan **sans clic** (attache au thread
qui le détient) :

| Geste | État du moteur | Meubles |
|---|---|---|
| Aucun clic, pointeur sur l'image | `pause=false`, position qui avance | **cachés après 3 s** |
| Un clic sur l'image | `pause=true`, position figée à 43,375 | affichés — **c'est la règle** |

Geste G12c corrigé en conséquence. Le harnais, lui, passait déjà : il n'y avait
rien à corriger dans le produit.

**Le pointeur, en revanche, ne s'en tire pas.** Même mesure propre — fenêtre au
premier plan sans clic, film de 180 s en lecture (`pause=false`, position de
4,7 s à 29 s), pointeur sur l'image, **quinze secondes sans aucune entrée** :
`GetCursorInfo` a lu le pointeur **affiché sur 35 échantillons sur 37**, les deux
lectures cachées isolées aux indices 6 et 29 au lieu de former une longue
absence. Le masquage natif est appliqué et **ne tient pas** sur WebView2. C'est
le vrai défaut de cette famille, et il est maintenant décrit exactement au lieu
d'être imputé à un clic.

**La leçon, qui vaut mieux que les deux entrées :** une sonde qui doit cliquer
pour voir quelque chose est une sonde qui change l'état qu'elle mesure.

### 4.2 (suite) — Les erreurs sont lisibles

Trois états couverts : serveur qui ne répond pas (dans les deux langues), moteur
qui ne démarre pas, repli audio. Chacun doit montrer une **phrase** et jamais la
clé du catalogue — le repli de `t()` est `?? key`, donc une phrase manquante
afficherait `connectionFailed` au milieu d'un écran français. Tout passe.

## Points à trancher, pas des défauts

1. **Débordement à 320×180** : voir 4.1 — il faut choisir ce qui cède.
2. **Fin de film** (G25b) : le moteur garde la dernière image et rapporte
   `pause`, donc les meubles restent sur une image figée, sans phrase ni retour à
   la bibliothèque.
3. **Le pointeur masqué ne tient pas** (G5b) : mesuré proprement, 35
   échantillons « affiché » sur 37 en quinze secondes de lecture sans aucune
   entrée. Le masquage natif est appliqué et ne persiste pas sur WebView2.
   Cause non établie ; c'est le premier point à reprendre si le pointeur compte.

## Ce qui reste non vérifié

Le plein écran observé par un œil humain ; la persistance du pointeur ; le
plein écran et le retour ; le passthrough HDMI/Atmos (pas d'amplificateur ni de
sortie HDMI audio ici) ; mDNS réel ; la suite playback `P` (D0 = A, port 8397).
