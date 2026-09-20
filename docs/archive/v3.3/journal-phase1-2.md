# Journal de campagne — phases 1 et 2

Suite de `journal.md` (phase 0). Dépôt : `C:\Users\starx\Documents\CODE\Theia`.
Toutes les commandes natives sont suivies de `$LASTEXITCODE`.

Décisions reçues le 15/09/2026 : **D0 A, D1 B, D2 A, D3a A, D3b A, D4 A,
D5 B avec le chemin `C:\Users\starx\Documents\Films`.**

---

## Phase 1 — Critères d'acceptation

### 1.1 — Décisions et critères — ~15 min

- **Fichiers.** `v3.3-phase0/decisions-benjamin.md` (hors dépôt) : tableau des
  réponses en tête, blocs « TRANCHÉ » datés sous D1 et D5.
- **Résultat.** Les six décisions sont fermées. Aucune direction artistique
  nouvelle n'a été inventée : D3a et D3b renvoient à la charte existante.
- **Point de vigilance soulevé.** D1 = B retire le dernier accès à la langue du
  lecteur natif. La conséquence est écrite sous D1 et a été traitée en 2.3.
- **Incertitude.** Aucune.

### 1.2 — Assertions manquantes dans `render-check.mjs` — ~50 min

- **Fichier.** `player/ui/scripts/render-check.mjs` (+261 lignes).
- **Commande.** `npm --prefix player/ui run check:render -- http://127.0.0.1:5199/ <out>`
- **Résultat.** Un quatrième bloc ajouté, couvrant les catégories que le plan
  nomme : fontes, cibles, clavier, temporisation, nombre de commandes IPC, plus
  le menu, le curseur, les exceptions d'inactivité et la découverte.
- **Preuve du « rouge d'abord ».** 12 échecs reproduits sur la version livrée,
  dont 8 sur les fontes, 3 sur le clavier et 1 sur la timeline.
- **Preuve que les blocs d'origine n'ont pas bougé.** Le nouveau bloc a été
  temporairement retiré, la suite relancée seule : `render check passed`, exit 0.
  Puis restauré (`git diff --stat` : 261 insertions).
- **Erreur de mesure commise et corrigée.** La première version de l'assertion
  d'inactivité déplaçait la souris sur `.title-bar` : le produit gardait
  correctement les meubles parce que le pointeur reposait sur eux. C'était mon
  test qui était faux, pas le lecteur. Corrigé en déplaçant le pointeur sur
  l'image.
- **Commit.** `7140cfa`.

### 1.3 — Protocole natif — ~25 min

- **Fichier.** `player/PROTOCOL.md` (141 lignes), `player/README.md` mis à jour.
- **Contenu.** 26 gestes numérotés G1–G26 en trois classes qui ne s'échangent
  pas : simulation (Chromium, `window.__TAURI__` simulé), automation (le vrai
  programme et ses `--diagnostics`), observation manuelle. Chaque geste porte son
  temps, son attendu et sa limite.
- **Ce qui a été écrit noir sur blanc.** Le curseur appartient à la troisième
  classe : `capture-window.ps1` photographie l'écran avec `CopyFromScreen`, qui
  ne dessine pas le pointeur, donc son absence d'un PNG ne prouve rien.
- **Commit.** `7140cfa`.

---

## Phase 2 — Curseur, focus, interactions

### 2.1 — Le curseur — ~35 min

- **Défaut reproduit.** `html` et `body` portaient `cursor: none` sans
  condition : l'écran de connexion et le panneau bibliothèque n'avaient aucun
  pointeur. Mesuré : `html=none body=none osd=none` avant qu'un film soit choisi.
- **Fichiers.** `player/ui/src/osd.css`, `player/ui/scripts/render-check.mjs`.
- **Correctif.** Le curseur est attaché à `.osd[data-idle='true']` — l'attribut
  qui dit déjà si les meubles sont partis volontairement. Les six règles
  `cursor: pointer` des contrôles sont intactes.
- **Résultat.** 16 échecs → 12. Les cinq points du curseur passent.
- **Incertitude, non levée.** WebView2 honore-t-il cette règle au-dessus d'une
  vraie image ? Chromium n'est pas WebView2, et la capture d'écran ne peut pas
  répondre. **Non vérifié**, laissé ouvert.
- **Commit.** `1237edb`.

### 2.2 — La temporisation — ~45 min

- **Fichiers.** `player/ui/src/App.svelte`, `render-check.mjs`.
- **Trois constats, chacun mesuré avant correction :**
  1. le focus n'était pas implémenté du tout — meubles à l'opacité 0 avec
     `button.control--primary` encore focalisé, donc l'Enter suivant aurait
     pressé une cible invisible ;
  2. sans film chargé, la minuterie emportait le panneau bibliothèque trois
     secondes après l'arrêt de la souris ;
  3. le menu ouvert était déjà couvert, et l'est maintenant par une assertion.
- **Correctif.** `focusInFurniture` suivi par `focusin`/`focusout`, `!status.media`
  ajouté à la garde, et un Tab clavier qui relance le compte plutôt que de
  compter un temps que personne n'a passé immobile.
- **Assertions ajoutées.** Visible à 2,5 s, caché à 3,5 s ; réveil en moins de
  0,5 s sur une touche et sur un mouvement ; jamais caché en pause, en tampon,
  sans film, menu ouvert, ou avec le focus sur un contrôle.
- **Erreur de mesure commise et corrigée.** L'assertion « rien n'est chargé »
  passait pour la mauvaise raison : l'état précédent laissait `ready` faux, donc
  le correctif du tampon répondait à une question que le test ne posait pas. Elle
  traverse maintenant un statut `ready` **sans** média, ce qui est le seul état
  qui distingue les deux défauts. Elle a alors échoué, correctement.
- **Résultat.** 13 → 12 échecs (le troisième était réel et corrigé).
- **Commit.** `42eaa91`.

### 2.3 — Clavier, menu, focus, et la conséquence de D1 — ~60 min

- **Fichiers.** `App.svelte`, `TrackMenu.svelte`, `osd.css`,
  `docs/design-system.md`, `render-check.mjs`, `player/README.md`.
- **Trois défauts mesurés :**
  1. les raccourcis globaux vivaient dans le champ d'adresse : `k` ne s'écrivait
     pas, l'espace basculait la lecture, et `l` changeait la langue en pleine
     adresse (`html lang` fr→en, mesuré) ;
  2. les flèches cherchaient dans le film menu des pistes ouvert ;
  3. `Échap` fermait le menu en laissant le focus sur le `body`.
- **Correctif.** Un garde-fou là où les touches arrivent — un champ éditable
  reçoit tout sauf `Échap` ; le menu possède ses flèches en ordre de lecture ; le
  focus revient au bouton d'ouverture ; l'espace sur un bouton focalisé est laissé
  au bouton.
- **D1 = B mis en œuvre avec sa conséquence.** Sans film, la barre n'est plus
  dessinée du tout, et son espace va à la bibliothèque. Le sélecteur de langue
  passe dans l'en-tête pour exactement ces états, et retourne dans la barre dès
  qu'un film joue — une seule fois à l'écran, toujours joignable.
- **Deux défauts supplémentaires trouvés à 550×350** (la vraie fenêtre de
  M. Berthier) : la bibliothèque défilait parce que la barre réservait 11 rem pour
  un film inexistant, et « SE CONNECTER » se repliait sur deux lignes derrière le
  bouton suivant. Corrigés : l'espace libéré va à la bibliothèque, et un libellé
  de bouton ne se replie plus ni ne rétrécit sous lui-même.
- **Piège d'ordre CSS rencontré.** `.controls--hidden { display: none }` écrit
  **avant** `.controls { display: flex }` ne faisait rien : une classe contre une
  classe est une égalité, et l'ordre tranche. Mesuré (`display: flex` malgré la
  classe appliquée), corrigé, et commenté — même piège que `button.control--mute`
  documenté dans le projet.
- **Erreur de mesure commise et corrigée.** L'assertion du menu utilisait
  `.menu-anchor button`, qui correspondait aussi aux cinq lignes du popover : le
  popover est un enfant du bouton. Remplacé par `button[aria-haspopup=menu]`.
- **Amendement documentaire.** `docs/design-system.md` §6b : la clause « the
  native player has nowhere else to switch it » devient fausse, elle est amendée
  dans le même commit.
- **Résultat.** 12 → 9 échecs.
- **Commit.** `a58c18a`.

### 2.4 — La découverte — ~30 min

- **Défaut confirmé puis reproduit.** `findServers()` pose `busy = true`, puis
  appelle `connect()`, qui retourne immédiatement quand `busy` est vrai : avec un
  seul serveur, `player_discover` s'exécutait et `player_connect` jamais.
- **Correctif.** `connect(url, { force })`, utilisé seulement par l'appelant déjà
  à l'intérieur d'une opération gardée.
- **Assertions.** Zéro, un et plusieurs serveurs, plus le bouton de recherche
  utilisable à la fin.
- **Limite dite.** mDNS lui-même n'est pas prouvé : le répondeur de cette machine
  refuse un bind multicast IPv6 et ne répond rien. Ce qui est vérifié est le
  comportement de l'OSD **étant donné** une réponse.
- **Résultat.** Suite complète verte après 3.2 (voir ci-dessous).
- **Commit.** `9d2f18e`.

---

## Phase 3 — Identité visuelle, entamée

### 3.1 + 3.2 — Les fontes et la timeline — ~55 min

- **Défaut principal.** L'OSD ne déclarait **aucune** `@font-face` : 0 règle dans
  son CSS construit, 0 `.woff2` émis, `document.fonts` vide. Titres en Georgia,
  étiquettes en Segoe UI — l'essentiel de l'écart typographique entre les deux
  surfaces, et structurel.
- **Correctif.** Les deux déclarations sortent de `web/src/app.css` (qui commence
  par `@import 'tailwindcss'` et ne peut donc pas être importé par un document
  sans Tailwind) vers `web/src/lib/fonts.css`, à côté de `tokens.css`. Les deux
  surfaces l'importent. **Une déclaration, deux surfaces, pas de seconde copie.**
- **Preuve que l'extraction est un déplacement et non un changement.** Les 41
  fichiers de `web-dist/_app/immutable/assets` ont le **même SHA-256** avant et
  après reconstruction complète par `build.ps1`.
- **Dépendances.** `@fontsource-variable/cinzel` et `jost` déclarés par l'OSD —
  les deux mêmes paquets, aux mêmes versions, que le web embarque déjà depuis
  npm. Aucun n'est nouveau dans le projet ; c'est une dépendance existante rendue
  explicite. Signalé à M. Berthier.
- **D3b appliqué.** La zone sensible de la timeline passe de 24 à 44 px, la ligne
  peinte reste à 4 px, et §6b est amendé dans le même commit.
- **Résultat.** **`render check passed`, exit 0** — les 12 assertions rouges de
  la phase 1 sont vertes. Mesuré ensuite à 390, 550 et 1280 dans quatre états
  chacun : aucun débordement, **aucune cible sous 44 px**.
- **Contrôles du web préservé relancés.** `contrast.mjs` (11/11), `check-locales`
  (684 valeurs), `check-backdrops` (5 images cadrées), `go test ./internal/api
  ./internal/stream` : tous verts.
- **Commit.** `742d247`.

---

## Vérification native (en cours)

- **Reconstruction.** Première tentative échouée : `npm install` ne pouvait pas
  supprimer `esbuild.exe`, tenu par le serveur de prévisualisation Vite. C'est le
  piège que `CLAUDE.md` documente. Prévisualisation arrêtée par **PID identifié**,
  puis reconstruction relancée.
- **Ce qui sera mesuré.** Capture de la fenêtre réelle à 1100×700 (550×350 CSS à
  200 %), lecture d'un film de la fixture en mode `--mute --diagnostics`, et
  recherche de l'état du curseur par appel Win32 si l'outillage le permet.
- **Ce qui restera non vérifié, quoi qu'il arrive.** Le curseur au-dessus d'une
  image en mouvement, la composition WebView2 sur pixels, le plein écran observé
  par un œil humain, et le passthrough HDMI — cette machine n'a ni amplificateur
  ni sortie HDMI audio.
