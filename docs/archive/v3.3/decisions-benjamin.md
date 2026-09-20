# Décisions à trancher — phase 0

Chaque question porte l'état présent mesuré, les options, et l'avis de
l'exécutant avec sa raison. **Aucune n'est une autorisation déjà acquise** :
rien de ce qui suit n'a été appliqué. Une attente ne bloque que le travail qui
en dépend ; le reste du chantier peut avancer sans elle.

Répondre par la lettre suffit (exemple : « D0 A, D1 A, D2 A, D3a A, D3b A,
D4 A, D5 C »).

---

## Réponses reçues — 15 septembre 2026

Posées et tranchées via l'agent DSH. **Aucune n'a encore été appliquée** : rien
n'a été modifié dans le dépôt, la phase 1 n'est pas commencée.

| Décision | Réponse | Écart avec l'avis rendu |
|---|---|---|
| D0 — Ports | **A** — suite playback sur 8397, tests manuels sur 8395 | conforme |
| D1 — Sans film | **B** — masquer entièrement la barre tant qu'aucun film n'est chargé | **contre l'avis** (A était recommandé) — voir la conséquence écrite sous D1 |
| D2 — `Échap` | **A** — garder la fermeture actuelle | conforme |
| D3a — Cartes | **A** — pas de rectangle doré, amender l'ancien §6.2 | conforme |
| D3b — Timeline | **A** — zone interactive de 44 px autour d'une ligne de 4 px | conforme |
| D4 — Terminal | **A** — hiérarchie sobre, affiner sans encadrer | conforme |
| D5 — Média réel | **B** — dossier autorisé : `C:\Users\starx\Documents\Films` | conforme (B), chemin précisé par M. Berthier |

**Deux points à ne pas perdre de vue :**

1. **D1 = B retire le dernier accès à la langue du lecteur natif.** Ce n'est pas
   une objection de principe : c'est une conséquence mesurée, écrite sous D1, et
   elle doit être traitée dans l'unité 2.3 — avec l'amendement de
   `docs/design-system.md` §6b qui va avec. Traiter 2.3 et l'écart E2 (le
   raccourci `l` qui bascule la langue pendant la saisie) dans le même
   mouvement règle les deux.
2. **Le dossier autorisé est la bibliothèque réelle complète** (11 fichiers,
   118,2 Go, dont le remux Atmos de 53,79 Go). Il sera scanné dans une base
   isolée, sans jamais écrire dans la bibliothèque ni dans `%APPDATA%\Theia`.

---

## D0 — Ports des suites de test

**État présent, vérifié.** `web/playwright.playback.config.js:5` et
`web/tests/serve-playback.mjs:18` codent **8397** en dur ; c'est aussi le port
écrit dans `config.json` par `internal/testfixture/main.go:65`. La suite de mise
en page (`npm --prefix web test`) utilise **8396** par défaut et accepte
`THEIA_TEST_URL` ; elle a été lancée sur 8396 pendant cette campagne, pas sur
8395. Le cadrage demande 8395.

- **A. Autoriser la suite playback existante sur 8397, garder 8395 pour les
  tests manuels.** Rien n'est modifié dans `web/`, le harnais gelé reste intact.
- **B. Imposer 8395 à tous les serveurs de test.** Exige un lanceur temporaire
  hors `web/`, des chemins absolus vers les mêmes tests, et la preuve que les
  scénarios sont identiques — c'est-à-dire un risque de modifier un harnais gelé
  pour un gain nul.

**Avis : A.** Le port n'est pas ce que la suite vérifie, et toucher `web/` pour
cela contredit l'invariant « serveur et interface web préservés ».

**Ce que cela débloque :** la suite P (phase 6.2).

---

## D1 — Commandes de lecture sans film chargé

**État présent, vérifié.** À l'ouverture, `status.media` est vide et le panneau
bibliothèque s'affiche — mais la barre de lecture reste dessinée : la timeline
(502×24), le bouton lecture/pause (52×52), l'horloge (`--:--`), le menu des
pistes est masqué (correct), et la langue, le plein écran et la fermeture sont
là. Mesuré à 550×350 ; captures
`before/osd-native-viewport/osd-connect-550x350-fr.png`,
`before/10-native-connect-1100x700-fr.png`.

- **A. Garder la barre, mais rendre inutilisables les commandes qui n'ont pas
  d'objet (lecture, timeline, horloge) et laisser langue et fermeture
  accessibles.** L'écran cesse de mentir sur ce qu'il peut faire.
- **B. Masquer entièrement la barre tant qu'aucun film n'est chargé.** Plus net,
  mais la langue devient inaccessible en petit écran : l'OSD n'a nulle part
  ailleurs où la changer.

**Avis rendu : A.** B supprime le seul accès à la langue sur une fenêtre
étroite, ce que le plan interdit explicitement (« ne pas supprimer l'accès à la
langue en petit écran »).

> ### TRANCHÉ — 15/09/2026 : **B** (contre l'avis rendu)
>
> **Conséquence directe, à traiter dans l'unité 2.3, pas à découvrir en phase 4.**
> Masquer la barre sans film chargé retire le dernier endroit où la langue se
> change dans le lecteur natif : `switchLanguage()` (`App.svelte:256-264`) n'est
> appelé que par `button.control--desktop`, qui vit dans `.controls`
> (`App.svelte:475-482`). Ce bouton disparaît déjà sous 30 rem
> (`osd.css:620-624`), et il disparaîtrait maintenant dans **tous** les états
> sans film, à toutes les largeurs.
>
> `docs/design-system.md` §6b dit pourquoi ce bouton existe :
> « The native player has nowhere else to switch it, unlike the web application,
> and at 550px there is room for it. » C'est donc **§6b qui devient faux** et
> qui doit être amendé dans le même commit que le changement, comme le veut la
> règle du dépôt — pas contourné en silence.
>
> **Contrainte que B doit respecter pour être acceptable :** l'accès à la langue
> doit rester joignable sans film chargé. Trois formes possibles, à arbitrer en
> 2.3 puis à documenter :
> 1. un sélecteur de langue compact sur l'écran de connexion ou dans l'en-tête
>    du panneau bibliothèque ;
> 2. un sélecteur de langue dans la fenêtre de connexion uniquement, la barre
>    restant masquée partout ailleurs ;
> 3. `L` conservé comme raccourci clavier seul — **insuffisant en soi** : un
>    écran tactile n'a pas de clavier, et la décision D1 vise justement à
>    éclaircir l'écran pour qui n'a pas de souris.
>
> **Corollaire mesuré à ne pas oublier :** le raccourci `l` est aujourd'hui
> celui qui bascule la langue **pendant la saisie de l'adresse** (écart E2). Le
> déplacer ou le rendre dépendant du contexte règle les deux problèmes d'un
> coup — c'est la raison de traiter 2.3 et E2 ensemble.

**Ce que cela débloque :** les unités 2.3, 3.2 et 3.4. Le libellé de 1.1 et de
3.2 doit désormais dire « barre masquée sans film » et non « commandes
inertes ».

---

## D2 — `Échap` hors menu et hors plein écran

**État présent, vérifié.** `App.svelte:281-288` : si le menu est ouvert, `Échap`
le ferme ; sinon il ferme le lecteur (`currentWindow.close()`).

- **A. Garder la fermeture actuelle.** Le geste est celui d'une fenêtre de
  lecteur, et le menu garde la priorité.
- **B. Revenir à la bibliothèque.** Plus indulgent, mais il faut définir ce que
  devient un film en cours, et tester la reprise — un parcours entier pour un
  geste que personne n'a signalé comme gênant.

**Avis : A dans ce chantier.** En plein écran, la séquence doit être : `Échap`
sort d'abord du plein écran, puis ferme — à faire valider par M. Berthier, car
c'est la seule partie qui touche le confort réel.

**Ce que cela débloque :** l'unité 2.1 (avec E12).

---

## D3a — Rectangle doré sur les cartes

**État présent, vérifié.** L'OSD dessine déjà le texte le plus récent : bordure
1 px `--accent` au survol et au focus (`osd.css:435-446`) **plus** l'anneau de
focus 2 px (`osd.css:449-453`), et une bordure transparente de 1 px au repos
pour ne pas déplacer la carte d'un pixel (`osd.css:388`). Le préambule V3.1 et
§6.2 disent la même chose ; l'ancien §6.2 cité par le plan disait l'inverse.

- **A. Suivre le web actuel et le préambule V3.1, sans rectangle doré.**
  Amendement documentaire de l'ancien §6.2 avant toute modification.
- **B. Revenir littéralement à l'ancien §6.2.**

**Avis : A** — le code, la charte actuelle et le web sont déjà d'accord ; c'est
le document ancien qui traîne.

**Ce que cela débloque :** l'unité 3.3, en principe rien à changer.

---

## D3b — Zone sensible de la timeline

**État présent, vérifié.** L'OSD : ligne peinte 4 px, zone sensible **24 px**
(`osd.css:97-103`, conforme à §6b). Le web : ligne 4 px, zone sensible
**48 px** (`app.css:3128-3135`, conforme à §9). §6b dit 24 px, §9 dit 44 px
minimum. Deux surfaces du même produit répondent donc différemment.

- **A. Zone interactive de 44 px autour d'une ligne de 4 px** pour l'OSD, comme
  le web et comme §9. Documenter l'arbitrage dans `design-system.md` avant de
  l'appliquer.
- **B. Conserver 24 px** au titre de §6b.

**Avis : A**, pour la cohérence avec §9 et avec le web — mais c'est bien §6b
qu'il faut amender, pas contourner.

**Ce que cela débloque :** les unités 3.2 et 3.4.

---

## D4 — Présentation du terminal

**État présent, vérifié.** L'interface est sobre : un titre « THEIA » en or
(`#C19C00`), une phrase d'introduction, une question encadrée par une barre
verticale, les options, et une ligne d'aide en gris (`#767676`), sur fond
`#0C0C0C`. Aucune bordure de section. Le texte se replie proprement à 80 et à
60 colonnes, sans perte d'information. Captures :
`before/00-setup-fr-100x30.png`, `80x24`, `60x24`.

- **A. Hiérarchie sobre et peu de bordures** — l'état actuel, à affiner
  (alignements, respiration, accent).
- **B. Sections davantage encadrées.**

**Avis : A.** Le parcours est validé et lisible ; la phase 5 ne doit pas
transformer l'installeur en tableau de bord.

**Ce que cela débloque :** la phase 5 entière.

---

## D5 — Média réel

**État présent, vérifié.** Aucun dossier de médias autorisé n'est connu de cette
session. `D:\` n'existe pas. La bibliothèque personnelle dans
`%APPDATA%\Theia` n'a **pas** été ouverte. La seule bibliothèque disponible est
synthétique : 4 titres, 45 s, sans affiche TMDB. Le plan demande une mesure de
lecture de 60 s, donc la fixture ne suffit pas.

- **A. M. Berthier remonte lui-même sa bibliothèque** (ou branche le disque).
- **B. Il autorise un dossier local précis**, qui sera scanné dans une base
  isolée, sans jamais toucher à la sienne.
- **C. Différer la validation réelle.** Le jalon reste alors **non vérifié** et
  le restera : aucune mire ne valide une collection.

**Avis rendu : A ou B.** C est honnête mais laisse la phase 6.4 vide.

> ### TRANCHÉ — 15/09/2026 : **B**, avec le chemin précis
>
> **Dossier autorisé : `C:\Users\starx\Documents\Films`**
>
> Vérifié en lecture seule pendant cette session, **sans ouvrir un seul
> fichier** et sans écrire quoi que ce soit :
>
> | Constat | Valeur |
> |---|---|
> | Existe | oui, dossier |
> | Structure | 1 fichier à la racine + 1 dossier de série (2 niveaux) |
> | Fichiers | **11 fichiers, tous `.mkv`** |
> | Taille | **118,2 Go** |
> | Modifié | 15/09/2026 08:38:51 |
>
> C'est **exactement** la bibliothèque déjà décrite dans `docs/v3.3.md:392-426` :
> le remux 2160p de *Star Wars, épisode III* (53,79 Go, piste TrueHD Atmos) et
> les 10 épisodes de *Shogun* — soit 1 film et 1 série, ce qui explique aussi le
> constat déjà écrit là-bas : le lecteur natif ne liste qu'un film sur onze,
> faute de navigation par série.
>
> **Ce que cette autorisation permet, et ses bornes :**
> - le scan se fera dans une **base isolée** (répertoire de campagne), jamais
>   dans `%APPDATA%\Theia` — la bibliothèque de M. Berthier et son historique de
>   lecture ne sont pas touchés, c'est la règle que `v3.3.md` documente déjà
>   (« scanned into a throwaway data directory so their own viewing history was
>   never touched ») ;
> - les fichiers seront lus en flux, jamais modifiés ni renommés ; la
>   vérification de fin de phase 6.5 compare l'empreinte du dossier avant/après,
>   comme la campagne précédente l'a fait (11 fichiers, 118,2 Go, 0 modifié) ;
> - `D:\` n'est toujours pas remonté, et rien n'est restauré automatiquement.
>
> **Ce que cela débloque :** les phases 4.4 et 6.4, et lève la seule réserve qui
> rendait le jalon non vérifiable. Le média de 60 s demandé par 4.4 existe ici :
> les épisodes et le remux dépassent largement la minute.

---

## Deux points qui ne sont pas des questions mais des constats à connaître

1. **E1 (fontes) et E2 (clavier) sont des défauts, pas des goûts.** Ils
   s'observent, ils se reproduisent, et ils ne dépendent d'aucune décision.
   Leur correction est une correction, pas une direction artistique.
2. **E6 (débordement du web à 550 px)** touche une surface gelée. Le corriger
   demande une décision explicite, parce que V3.3 dit que le player web ne
   reçoit que des correctifs de régression et de sécurité. À trancher avec D0
   ou séparément.
