# Bilan de la phase 7 — revue locale finale

**Date :** 16 septembre 2026
**Dépôt :** `C:\Users\starx\Documents\CODE\Theia`, branche `main`
**Portée :** phases 1 à 7 du plan V3.3, la phase 0 étant livrée
**Ce qui n'a pas été fait, par consigne :** aucun envoi. Zéro `push`, zéro tag,
zéro release, zéro PR, aucune écriture GitHub. Tout est en commits locaux.

---

## 1. Ce qui a été corrigé et vérifié

Vingt-trois commits locaux depuis `249532e`. Chaque ligne ci-dessous nomme la
preuve et son environnement.

### Défauts de l'OSD trouvés par la campagne et corrigés

| Défaut | Preuve | Environnement |
|---|---|---|
| `cursor: none` posé sans condition — plus de pointeur sur l'écran de connexion ni sur la bibliothèque | assertion + capture | Chromium, `check:render` |
| La minuterie d'inactivité était relancée par **chaque trame de statut** (500 ms), donc le meuble ne se cachait jamais | assertion sur la cadence réelle du moteur | Chromium |
| Minuterie armée survivant à l'état qui l'avait armée | assertion | Chromium |
| `k`, espace et `l` avalés pendant la saisie d'une adresse | assertion « taper n'est pas un raccourci » | Chromium |
| Les flèches scrrubaient le film menu ouvert ; `Échap` perdait le focus | assertion menu | Chromium |
| Un seul serveur découvert ne se connectait pas | assertion découverte (0 / 1 / plusieurs) | Chromium |
| L'OSD ne déclarait **aucune** `@font-face` — titres en Georgia, libellés en Segoe UI | `document.fonts.check` + statut `loaded` réel | Chromium, bundle vérifié au SHA-256 |
| Frise de 24 px au lieu du plancher de 44 | mesure des cibles | Chromium, 8 largeurs |
| Barre dessinée sans film, langue inaccessible | assertion D1 | Chromium |
| Débordement de la rangée entre 480 et 684 px (barre de défilement horizontale dans le lecteur installé) | mesure à 550 px | fenêtre native, écran 200 % |
| `document.documentElement.lang` restait `fr` sous un choix anglais stocké | assertion langue | Chromium |
| Débordement à 320×180 : 314 px demandés dans 272 px | assertion + capture | Chromium (décision 124) |
| **En plein écran, Échap fermait le lecteur au lieu d'en sortir** | corrigé, mesuré avant/après, asserté en quatre étapes | fenêtre native, 1440×900 (décision 126) |
| **Le pointeur ne se cachait pas à l'inactivité** | cause trouvée, correctif CSS appliqué, moitié native retirée | fenêtre native WebView2 (décisions 124 et 125) |

### Un défaut que j'avais annoncé et qui n'existait pas

J'ai d'abord écrit qu'« un clic sur l'image empêche le meuble de se cacher ».
C'était faux : mes sondes cliquaient pour prendre le premier plan, un clic **met le
film en pause**, et §6b garde le meuble tant que le film est en pause. Commit
`2ba7d1e` retire le défaut et dit ce que le pointeur fait réellement.

### Défauts de l'installeur

| Défaut | Preuve |
|---|---|
| `--service` sans `--yes` installait une entrée de démarrage non confirmée | test de sous-processus : refus **avant** écriture, les deux répertoires vérifiés absents |
| « Échap » affiché sous la question sans être lié (Huh ne lie que Ctrl+C) | test du lien de touche + test du modèle |
| Aucune vérification que « non », Échap et Ctrl+C n'écrivent rien | 5 gardes qui lancent `RunInteractive` contre un `APPDATA` redirigé et des répertoires temporaires, puis parcourent les trois |
| Description tronquée à 120 colonnes — **fausse alerte** : c'était la photo qui était coupée par le bord de l'écran | garde `TestNoPageDrawsWiderThanItsForm` sur les chaînes rendues |
| La forme s'arrête à 76 colonnes et ne déborde jamais | mesure du tampon de console à 80 / 100 / 120 / 200 colonnes, deux langues |

### Le lecteur natif

| Mesure | Valeur |
|---|---|
| Mouvement à deux instants | image **21** → **211** (00:00:00.825 → 00:00:08.792), lu dans le timecode peint par la fixture |
| Plein écran et retour | 1440×900 (image 266) → 1280×720, le rectangle d'origine exactement |
| Film derrière l'OSD | titre en Cinzel net sur l'image, sans cadre |
| Lecture de 60 s | 3,59 s de CPU (~6 % d'un cœur), 188,6 → 189,3 Mo stable, `wasapi`, `d3d11va`, `gpu-next`, 0 ligne d'erreur |
| Fenêtre à sa taille minimale | 320×180 CSS : rien ne déborde, 4 commandes, horloge à un seul nombre |

### La bibliothèque réelle

`C:\Users\starx\Documents\Films`, **jamais modifiée**, scannée dans une base
jetable :

| | |
|---|---|
| Scan | 11 fichiers, 118,2 Go, **0 problème**, 1 film et 1 série de 10 épisodes |
| Lecture | `Range: bytes=0-262143` → **HTTP 206, 262 144 octets** |
| Position écrite | 42,5 s relue **42,5 s** |
| La bibliothèque après | **11 fichiers, 118,2 Go, même horodatage à la seconde — inchangée** |
| Limite connue, reconfirmée | 10 des 11 éléments sont des épisodes, invisibles pour la liste de films |

### L'installeur, bout en bout

Installation réelle, `--check`, liste des applications Windows, recherche Windows,
Flow Launcher, désinstallation réelle : tout est vérifié. Le bundle du lecteur
contient exactement ses quatre fichiers et le SHA-256 du moteur correspond au pin
`player/libmpv.json`.

---

## 2. Ce qui est vérifié (récapitulatif des portes)

Toutes rejouées après le dernier changement de code :

| Porte | Résultat |
|---|---|
| `go test ./...` | **exit 0**, 24 paquets (dont `cmd/theia-setup` 6,9 s et `internal/setup` 5,4 s) |
| `check:render` (OSD) | **exit 0**, 113 modules, après renforcement de l'assertion du pointeur |
| `check:i18n` (OSD) | fr 33 / en 33 |
| `contrast.mjs` | 11 rapports, tous conformes |
| `check-locales.mjs` | 684 valeurs, 54 fonctions |
| `check-backdrops.mjs` | 5 images pleine largeur, toutes cadrées |
| `svelte-check` | 0 erreur, 0 avertissement |
| `npm test` (mise en page, 4 moteurs) | **81 réussis, 3 ignorés, 0 échec** |
| `npm run test:playback` | **39 réussis, 5 ignorés, 0 échec**, exit 0 |
| `build.ps1` | serveur 17,7 Mo |
| `build-player.ps1 -Release -Bundle` | bundle 43,3 Mo, moteur revérifié aux deux pins, extrait à neuf et lancé sans variable d'environnement |
| Séquence Échap dans le binaire du bundle extrait | plein écran 1440×900 → un Échap → 1280×720, film à `pos 0,58 → 4,67`, second Échap ferme |
| `git diff --check` sur les 30 commits | propre |
| Installeur sans couleur | 478 caractères identiques, 377 cellules peintes dans les deux cas |

---

## 3. Ce qui n'est pas vérifié, et pourquoi

1. **Le pointeur revient, et la partie atteignable depuis ce dépôt est épuisée.**
   Trois choses ont été établies par la mesure, pas par l'argument. La règle de
   curseur était posée sur un élément qui ne peut jamais être survolé
   (`.osd` a `pointer-events: none`), donc elle ne s'appliquait pas — corrigé, sur
   la racine, et asserté contre l'élément que la personne pointe réellement. Un
   crochet `WM_SETCURSOR` a été construit, installé et mesuré : la fenêtre Tauri a
   reçu **quinze messages pendant une lecture, et aucun `WM_SETCURSOR`** — parce
   que **la fenêtre qui décide du curseur est `Chrome_WidgetWin_1`, dans le
   processus WebView2**, et qu'un processus ne peut pas sous-classer la fenêtre
   d'un autre. La moitié native `ShowCursor` a été retirée (décision 125) : son
   compteur lisait −1 dans un passage où `GetCursorInfo` rapportait le pointeur
   affiché 59 fois sur 59, et il pouvait cacher un pointeur sur un bureau auquel
   il ne le rendait pas. Ce qui reste — la plateforme redessine un pointeur
   au-dessus d'une page qui demande `none` — est mesuré à **5, 4 et 6 échantillons
   cachés sur 48** sur trois passages consécutifs du build final. Il n'y a plus
   d'étape disponible depuis l'intérieur de ce dépôt.
2. **Le plein écran vu par un œil humain, et la lisibilité à trois mètres.** Le
   rectangle et l'image sont vérifiés ; le jugement visuel ne peut pas l'être ici.
3. **Le passthrough HDMI / Atmos.** Aucun amplificateur, aucune sortie HDMI audio
   sur cette machine. Le chemin de requête et le chemin de refus sont prouvés ;
   le chemin d'acceptation ne peut pas l'être ici.
4. **Le mDNS réel.** La pile Go refuse une liaison multicast IPv6 sur cette
   machine ; l'annonce démarre et se décrit, la navigation ne reçoit rien.
5. **`--from` en installation réussie, à travers `main`.** Un vrai `install`
   s'enregistre dans `HKCU\Software\Theia` ; le registre n'a pas d'`APPDATA` à
   rediriger et le nom de clé est une constante d'un binaire non-test. Lancer ce
   chemin ici aurait écrit l'entrée réelle de M. Berthier. Les refus et les
   lectures sont vérifiés par sous-processus ; l'installation réussie est vérifiée
   une couche plus bas, dans `internal/setup`.
6. **La composition des pixels par WebView2** au sens strict — le film vu *à
   travers* l'OSD transparent. La capture native montre le titre dessiné sur le
   film, ce qui est la composition observable ; le rendu exact de la transparence
   sur un téléviseur reste à l'œil du mainteneur.
7. **Une capture d'écran cadrée de l'installeur à 80×24 et 100×30.** La mesure de
   largeur est fiable (tampon de console) ; la photo ne l'est pas sur cette
   machine, où le cadre visible est un onglet Windows Terminal qu'aucun titre du
   programme n'atteint. `capture-tui.ps1` le dit maintenant au lieu de produire une
   image trompeuse.
8. **Le comportement au logon de l'entrée de démarrage.** L'entrée est écrite et
   relue ; le redémarrage n'a pas été fait.
9. **macOS, Linux, Android TV, Apple TV, iOS.** Rien n'a tourné sur ces plateformes.
10. **La fenêtre native à 320×180.** L'arithmétique de la rangée y est corrigée et
    assertée dans le navigateur (décision 124). La même état photographié dans la
    vraie fenêtre n'a **pas** été concluant : le bundle publié, fenêtre à 640×360
    réels (`GetClientRect` et `GetDpiForWindow` le confirment : 320×180 CSS à
    192 dpi), premier plan pris sans clic, pointeur bougé pour réveiller le
    meuble — l'image montre le titre et le film, **et aucun meuble**. Cela ne
    permet pas de distinguer un défaut produit d'une erreur de mesure, et trois
    fois dans ce chantier un défaut annoncé s'est révélé être dans la sonde. C'est
    donc déclaré **non vérifié**, avec les étapes exactes, plutôt que déclaré
    défectueux.

---

## 4. Ce qui est en attente

1. **Rien sur le pointeur** : les trois voies ont été épuisées (CSS corrigé,
   `WM_SETCURSOR` mesuré comme impossible, masquage natif retiré). Le défaut
   résiduel est un comportement WebView2, inscrit comme tel.
2. **L'esthétique de l'installeur** (phase 5.2) : la hiérarchie, la teinte d'accent
   et la respiration restent à l'arbitrage de M. Berthier. La largeur, elle, est
   mesurée et tenue.
3. **Une capture fiable de l'installeur** : les mesures de couleur et de largeur
   sont dans le relevé, mais aucune photo cadrée n'est montrable sur cette machine.
4. **La série invisible pour la liste de films** : 10 des 11 éléments de la
   bibliothèque réelle. C'est une fonctionnalité tournée vers la bibliothèque, donc
   la décision 97 s'applique — elle attend une décision plutôt que d'arriver sans
   qu'on l'ait demandée.

---

## 5. Les décisions prises pendant cette phase

| Décision | Ce qui a été décidé | État |
|---|---|---|
| **124** | À la taille minimale de fenêtre, l'horloge donne son nombre total et garde le temps écoulé | appliqué, asserté, vérifié |
| **125** | Le masquage natif du pointeur est retiré ; la règle CSS est le mécanisme entier | appliqué, mesuré sur trois passages |
| **126** | Échap sort du plein écran avant de fermer le film, et ferme le menu avant les deux | appliqué, mesuré avant/après, asserté en quatre étapes |

Les trois sont écrites dans `docs/DECISIONS.md`, avec la mesure qui les a forcées.

**Une vérification que le plan demandait et que personne n'avait faite** a produit
le troisième défaut : la décision D2 dit « en plein écran, proposer d'abord sa
sortie et faire valider la séquence complète ». La moitié fenêtrée était vérifiée,
la moitié plein écran jamais jouée. Mesurée : plein écran 1440×900, **un seul
Échap, `running=False`** — le lecteur avait disparu. Sur un téléviseur, c'est un
film perdu. Corrigé et revérifié : après un Échap la fenêtre revient à 1280×720
avec le film à `pos 0,58 → 4,63`, `pause=false`, et le second Échap ferme.

---

## 6. État de la machine après le chantier

- **Aucun processus Theia en cours**, aucun écouteur sur 8383, 8395, 8396, 8397,
  5199. Le serveur d'aperçu de l'OSD a été arrêté par son PID enregistré.
- **Le port 8383 n'a jamais été touché**, `%APPDATA%\Theia` jamais modifié par une
  commande de ce chantier.
- **`C:\Users\starx\Documents\Films` intact** — vérifié deux fois, 11 fichiers,
  118,2 Go, horodatage inchangé.
- **Arbre de travail propre** : `git status` ne montre que `v3.3-phase0/`, le
  dossier de livrables de la phase 0, volontairement non suivi.
- **La console DSH sur 3080 est vivante** — c'est par elle que ce bilan est rendu.

---

## 6 bis. Le périmètre du chantier, vérifié

| Contrôle | Résultat |
|---|---|
| `git diff --check` sur tous les commits | **propre** — aucun espace blanc fautif, aucun marqueur de conflit |
| `internal/` et `cmd/`, code produit | **aucune modification** — les quatre fichiers Go touchés sont trois fichiers de test |
| `web/src` | **une extraction de fontes**, vérifiée déclaration par déclaration comme identique (24 lignes de part et d'autre) |
| Bundle web | les deux `@font-face` présents, mêmes empreintes `.woff2`, ordre du bundle inchangé |
| Médiathèque réelle | scannée deux fois en base jetable, **jamais modifiée**, vérifiée inchangée après coup |

---

## 7. Les journaux de ce chantier

| Fichier | Ce qu'il couvre |
|---|---|
| `diagnostic.md`, `mesures.csv` | la phase 0 : le diagnostic et ses mesures |
| `journal.md`, `journal-phase1-2.md` | la mise en place, les premiers défauts de l'OSD |
| `journal-phase3.md` | la découverte, la langue, les raccourcis |
| `journal-phase4.md`, `journal-phase4-3.md` | les fenêtres, les langues, la vraie vidéo |
| `journal-phase5.md` | l'installeur |
| `journal-pointeur.md` | l'enquête sur le pointeur, la cause trouvée |
| `journal-wmsetcursor.md` | le crochet essayé et retiré, l'arbre des fenêtres |
| `bilan-phase7.md` | ce document |
| `preuves/` | les captures, les journaux de sonde, les relevés JSON |
