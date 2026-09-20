# Journal — phase 5 (esthétique de l'installeur), 16 septembre 2026

Dépôt `C:\Users\starx\Documents\CODE\Theia`. **Aucun changement esthétique
appliqué** : la phase a été mesurée avant d'être touchée, et la mesure a retiré le
défaut qu'on croyait voir.

## Commit

```
2ea9c87 Measure the installer's width instead of looking at it
```

## 5.1 — Le formulaire, observé et mesuré

### Ce qui a été regardé

Captures aux tailles du plan, en français et en anglais :
`phase5\tui-fr-120x40.png`, `phase5\tui-en-120x40.png`. La sonde de console
relit le tampon et confirme les couleurs déjà mesurées en phase 0 : or `#C19C00`
pour « THEIA », titre de question et option sélectionnée ; corps `#CCCCCC` ;
aide `#767676` ; fond `#0C0C0C`.

Le formulaire est lisible et sobre : marque, phrase d'introduction, question
encadrée d'une barre verticale, trois options dont une en or, ligne d'aide.
Aucune bordure de section — ce que D4 = A a demandé.

### Un défaut cru, puis retiré par la mesure

La capture à 120 colonnes **semblait** couper la description française au bord
droit (« un mini-PC dans un placard et u… »). **Elle ne la coupe pas.**

`scripts/capture-tui.ps1` dimensionne la fenêtre de console, et à 120 colonnes
cette fenêtre dépasse le bord d'un écran de 1440 pixels : **c'est l'image qui est
rognée, pendant que le formulaire a exactement la largeur qu'il doit avoir.**

C'est la **deuxième fois** dans ce chantier qu'un artefact de capture a été à une
phrase d'être écrit comme un défaut — la première était une fenêtre non
DPI-consciente prise pour un débordement (phase 0, E3). Même remède les deux
fois : **une affirmation de largeur se mesure sur la chaîne rendue, pas sur
l'image.**

### La garde qui en est sortie — `internal/setup/width_test.go`

`TestNoPageDrawsWiderThanItsForm` rend **chaque page**, dans **les deux langues**,
aux **trois largeurs du plan** (74, 94, 114 colonnes) et pour **les trois rôles**.
Elle retire les séquences d'échappement et le remplissage de la carte, puis échoue
si une ligne visible dépasse la largeur donnée au formulaire.

**Résultat : elle passe.** Aucune page ne déborde, à aucune de ces combinaisons.

Un premier essai de cette mesure annonçait « 78 runes dans un formulaire de 74
colonnes ». C'était faux : les séquences ANSI et le remplissage étaient comptés
comme des caractères. Une vérification de largeur qui compte les échappements est
une vérification qui parle de la palette.

### Hauteurs de page, mesurées

Chaque page fait **15 lignes**, dont **6 à 10 lignes vides** selon la page. C'est
`clamped()` qui réserve la hauteur du plus grand écran à chaque page, ce que le
code assume (« Huh réserve la hauteur qu'on lui donne pour chaque page »). La
page du service, la plus courte, porte **10 lignes vides sur 15**.

**Constat, pas encore correction :** c'est de l'espace dépensé, et le plan demande
justement « alignements, largeur, accent et respiration ». Le corriger demande de
toucher à `formSize`/`clamped`, donc au dimensionnement que les tests et les
captures actuelles décrivent — **arbitrage à rendre visible avant de le faire**,
pas à appliquer en silence.

### Ce que je n'ai pas pu vérifier

Le rendu du **résumé** de la dernière page : mon diagnostic n'a rien rendu, donc
je ne peux rien affirmer sur sa mise en page. `TestTheConfirmationCarriesTheAnswersSomebodyJustGave`
prouve que les valeurs y sont ; sa forme n'est pas mesurée.

## 5.2 à 5.4 — Non commencées

- **5.2** (alignements, largeur, accent, respiration) : dépend de l'arbitrage sur
  la hauteur de page ci-dessus.
- **5.3** (captures FR/EN à 80×24, 100×30, 120×40) : la capture à 120×40 est
  faite ; 80×24 l'était en phase 0. **Le rognage à l'écran doit être réglé dans
  l'outil avant de produire des preuves comparables** — sinon on compare des
  images rognées.
- **5.4** (tests setup : « non », Échap et Ctrl+C n'installent rien ; `--from`,
  `--force`, `--uninstall` couverts en ressources isolées) : ces tests
  **existent et passent** (`internal/setup`, 4,0 s, exit 0) ; il reste à le
  vérifier explicitement plutôt qu'à le supposer.

## Vérifications du round

| Contrôle | Résultat |
|---|---|
| `go test ./...` | **exit 0**, aucun paquet en échec |
| `node scripts/contrast.mjs` | 11/11 conformes |
| `node web/scripts/check-locales.mjs` | 684 valeurs, 54 fonctions |
| `node web/scripts/check-backdrops.mjs` | 5 images cadrées |
| `npm --prefix web run check` | 0 erreur, 0 avertissement |
| `npm --prefix web test` (layout) | 81 réussis, 3 ignorés, 0 échec |
| `npm --prefix player/ui run check:i18n` | fr 33 / en 33 |
| `.\build.ps1` | serveur 17,7 Mo, construit |
| `.\build-player.ps1` | 14,7 Mo, construit |
| Nouvelle garde de largeur | **passe**, 2 langues × 3 largeurs × 3 rôles |
