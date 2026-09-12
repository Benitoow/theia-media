# Refonte du backend de lecture - V3.2 preview 4

Clôture du 11 septembre 2026 de la refonte décrite dans
[`plan-refonte-lecture.md`](plan-refonte-lecture.md) (tranches 3 à 6 ; les
tranches 1 et 2, filet doré et plan unique, étaient closes à la preview 3 ou
peu s'en faut). Tout est consigné ici, et le détail tranche par tranche reste
dans le tableau d'état du plan. Aucun commit : cet état s'ajoute à la base non
commitée des décisions 98–102, comme convenu.

## Changements du moteur

- Un seul service (`internal/playback`) possède l'exécution de la lecture :
  spawn de FFmpeg, en-têtes, porte du premier octet, kill sur erreur de copie,
  journaux. Les jumeaux film/épisode gardent leurs routes, tables et
  identités (décision 39) ; leurs corps partagés vivent dans `stream_twin.go`
  - `internal/api` sort de la refonte avec 384 lignes de moins que la preview 3.
- Les flux convertis sont enregistrés avant même l'existence de leur
  processus. Le plafond des remux (4 simultanés) refuse **avant** tout spawn,
  avec la forme `transcode_busy` (503 + `Retry-After: 1`) que le transport
  retente déjà - la seule réponse nouvelle que le contrat gelé autorisait.
  Les transcodes sont enregistrés pour le kill mais budgétés par le limiteur
  transcode, qui dérive désormais son plafond **une fois** par processus de la
  sonde d'encodeurs, jamais par requête.
- `main.go` tue les flux sur chaque chemin qui termine le processus : avant le
  drain gracieux, avant chaque `os.Exit` de la mise à jour, plus un filet
  `defer`. La décision 104 consigne le filet ; la 105, la priorité interactive
  et la dérivation unique du plafond ; la 103, la source unique de la décision.
- L'extraction de sous-titres embarqués est bornée (`boundedio.Head`, 8 Mio).
  Un échec répond toujours 415 avant toute écriture ; une sortie qui dépasse
  le plafond commet la réponse et streame la suite plutôt que tronquer en
  silence. Décision 106.
- Le `Flush` après `WriteHeader` est **adopté sur mesure** (voir ci-dessous),
  et commenté à l'endroit exact du code avec ses chiffres.

## Mesures de cette preview

**Premier octet d'un flux converti** (A/B, même film généré 10 min 1080p,
même machine, 10 passages par condition, médiane) :

| Condition | Client simple | Client gzip |
| --- | ---: | ---: |
| Sans flush | 75 ms | 78 ms |
| Avec flush | **48 ms** | **60 ms** |

Le gain vient du `compressWriter`, qui retient la réponse jusqu'à 1 400
octets pour décider d'un encodage ; le flush commet la décision et laisse les
premiers octets suivre. Aucun effet de bord sur les en-têtes observables.

**Arrêt pendant trois lectures vivantes** (`scripts/verify-shutdown`, binaire
réel, film généré, trois remux tenus, trois processus FFmpeg comptés dans la
liste des processus, interruption console) :

| Mesure | Valeur |
| --- | ---: |
| Sortie du serveur après l'interruption | **20 ms** |
| FFmpeg orphelins dans la liste des processus | **0** |

Avant la refonte, le chemin de mise à jour sortait par `os.Exit(0)` sans tuer
les processus suivis : l'orphelin était la normal, pas l'exception. Le même
script documente honnêtement l'autre face : un client qui tient un flux ouvert
sans le lire parque la boucle de copie côté écriture et coûte le délai de
drain (10 s) - le kill et la mort des encodeurs restent immédiats, le délai
appartient à la connexion bloquée, pas à un processus.

**Matériel** (campagne complète dans
[`hardware-measurements-tranche-6.md`](hardware-measurements-tranche-6.md),
script reproductible `scripts/measure-hardware`) : sur le Ryzen AI 9 HX 370 /
Radeon 890M, le décodage logiciel d'une source 4K HEVC bat `d3d11va` de 2,4x
(13,95x contre 5,75x temps réel) ; `h264_mf` encode 28 % plus vite que
`h264_amf` mais dépasse un plafond de 2 Mb/s de 44 % (2,87 Mb/s mesurés)
pendant que `h264_amf` tient 2,19 - l'ordre de priorité de `Best()` est
vindiqué par la mesure, le moteur ne change pas. La cible reste H.264 SDR ;
HEVC, AV1 et le tone mapping GPU restent des options documentées, pas adoptées
(décision 107, décision 58 respectée sans supersession).

## Vérifications finales

- `go test ./...` : 21 paquets verts ; `go vet`, `gofmt` nets.
- Garde Playwright téléphone, bureau et télévision : 81 réussites, 3
  scénarios ignorés par leur condition historique.
- Lecture réelle générée : 8 scénarios réussis sur le binaire réel et le
  FFmpeg épinglé.
- Six cibles `CGO_ENABLED=0` (Windows/Linux/macOS × AMD64/ARM64).
- Corps dorés de `/info` : six fichiers générés et stables entre exécutions ;
  la première génération avait échoué sur un répertoire `testdata/` absent -
  cause corrigée et consignée dans le plan.
- Contrat intact : codes 400/404/415/503, équivalence legacy (écart `?audio=`
  conservé, épinglé), asymétrie `?h` film/épisode conservée, rebasage des
  sous-titres, piste image refusée nommée.
- Frontend : 34 tests unitaires, locales fr/en (683 valeurs), svelte-check
  sans erreur, garde de contraste. L'ancre d'agrégat des 62 fichiers
  (`node web/scripts/frontend-anchor.mjs`) vaut
  `58D8EE16C11DBEC14C1BCD21E05757D6958546966DA0DF203BCB1815CEBDAD99` -
  l'ancre de la preview 3 n'avait pas de méthode reproduisible ; le script
  est désormais la spécification.

## Outils laissés au dépôt

`scripts/measure-hardware` (campagne matérielle reproductible),
`scripts/verify-shutdown` (acceptation d'arrêt de bout en bout),
`web/scripts/frontend-anchor.mjs` (empreinte du frontend). Les trois
fonctionnent sans dépendance au-delà du dépôt et du FFmpeg épinglé.

## Limites dites franchement

La lecture réelle sur la bibliothèque de 274 films (film difficile en entier,
seeks, pistes, pause, redémarrage pendant la lecture) reste la porte du
mainteneur - les suites l'ont couverte sur médias générés, pas sur sa
bibliothèque. Le chemin `os.Exit(0)` de la mise à jour est vérifié au niveau
unitaire ; un bout-en-bout de mise à jour avec serveur d'essai n'a pas été
monté. Le premier scan d'une base neuve n'expose le film qu'au second passage
du watcher (~60 s) : observé, noté, hors chantier. Les familles matérielles
autres qu'AMD attendent leurs machines pour produire leurs propres tableaux.

## Hors de cette preview

Les cibles HEVC/AV1, le tone mapping GPU, le 4:2:2, HLS, ffprobe, un runtime
FFmpeg différent, le découpage de `Player.svelte`, la réduction d'empreinte :
changement produit ou lot distinct du plan de modernisation, à valider
séparément.
