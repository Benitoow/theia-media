# Modernisation du moteur - V3.2 preview 5

Clôture locale du 12 septembre 2026. Cette tranche rassemble les derniers
travaux retenus avant une éventuelle publication de la 3.2. Rien n'a été
publié, commité ou poussé par cette validation.

## Runtime et mises à jour récupérables

- Un FFmpeg officiel 6.1.1 est reconnu par son hash puis remplacé par Jellyfin
  FFmpeg 8.1.2-4. Un exécutable manuel ou altéré n'est jamais lancé : il n'est
  déplacé qu'après vérification complète du remplaçant officiel et reste
  conservé sous `.unmanaged`. Les échanges incomplets sont annulés.
- Le parcours réel sur une copie isolée de la bibliothèque a migré le hash
  Windows AMD64 `04e13079…` vers `54658034…`; le nouvel exécutable a déclaré
  `ffmpeg version 8.1.2-Jellyfin`.
- Le harnais `scripts/verify-update` fabrique deux vrais binaires. Le scénario
  sain installe 3.2.1 depuis 3.2.0, redémarre sur un nouveau PID, sert la santé
  attendue et nettoie l'ancien binaire et le journal. Le scénario malsain lance
  réellement la cible, constate sa santé invalide, restaure 3.2.0 et redémarre
  l'ancienne version en conservant l'échec pour diagnostic.

## Lecture et ressources

- Seule une lecture active émet les heartbeats. Un remux mis en pause libère
  au bout de deux minutes sa requête, son MediaSource et son processus FFmpeg,
  après sauvegarde de la position absolue. La reprise crée un pipe neuf au même
  endroit. Un démarrage lent n'est plus pris pour une pause.
- Le buffer fragmenté conserve six secondes de réserve puis adapte sa cible
  roulante de 30 à 15, 7,5 et 6 secondes lors d'une saturation. Le lot refusé
  est retenté, l'arrière est évincé et le réglage sûr est mémorisé par classe de
  sortie. `MediaSource` et `ManagedMediaSource` sont acceptés.
- Le lecteur conserve maintenant l'horloge absolue après un seek dans ses
  diagnostics. Les URL de remux portent une génération locale afin qu'une
  reprise ne puisse pas réutiliser un pipe natif déjà fermé.
- `Player.svelte` passe de 1 895 à 1 683 lignes. Les pistes/qualités/sous-titres
  et l'aide vivent dans `PlayerTrackMenu.svelte` et `PlayerHelp.svelte`; le
  parent garde le cycle de vie, le focus, la télécommande et l'horloge.

## Bibliothèque

- Le watcher recalcule son attente sur l'échéance réelle de stabilité du
  fichier. Un fichier neuf stable n'attend plus arbitrairement un second scan
  environ une minute plus tard.
- Les séries sont chargées par pages jusqu'à épuisement comme les films. Le
  test de contrat récupère 1 203 identifiants distincts aux offsets 0, 500 et
  1 000 et vérifie qu'une page vide ne crée pas de boucle.

## Endurance mesurée

Le harnais `scripts/verify-endurance` a utilisé le vrai binaire et le vrai
FFmpeg sur le corpus généré :

| Mesure | Résultat |
|---|---:|
| Flux remux ouverts, lus puis annulés | 100/100 |
| FFmpeg restant après chaque annulation | 0 |
| RSS Theia avant le cycle | 27,3 Mio |
| RSS final et pic observé | 32,9 Mio |
| Flux continu complet | 30 min, 42,4 Mio en 46,049 s |
| FFmpeg restant à la fin | 0 |

Ce test prouve l'absence d'orphelin dans ce scénario et une mémoire bornée sur
100 cycles ; il ne remplace pas une campagne de plusieurs jours sur toutes les
familles matérielles.

## Bibliothèque et média réels isolés

La bibliothèque personnelle n'a pas été modifiée. Une copie de données dans
`%TEMP%`, servie sur le port 8395, a indexé un remux MKV de 7,21 Gio : HEVC
1920 × 804 à 23,98 i/s, 7 268,64 s, trois pistes audio et deux sous-titres.
L'inspection a pris 1 972 ms et choisi le remux avec conversion disponible.

Microsoft Edge 152 a présenté 55 images avant seek, atteint environ 3 997 s,
changé de piste audio, libéré le flux sur pause longue puis repris autour du
même GOP. Deux passages ont fini sans erreur de page et avec zéro FFmpeg avant
et après. Le data-dir normal, le port 8383 et la source vidéo sont restés
intacts.

## Navigateurs

Le garde de lecture possède désormais quatre projets distincts : Chromium,
Microsoft Edge stable installé, Firefox et WebKit. Les scénarios couvrent
lecture directe, conversion, remux, seek, pause, épisode MPEG-2, pistes,
sous-titres, reprise d'erreur, pression de quota et disponibilité tardive de
FFmpeg.

Le WebKit de Playwright sous Windows ne fournit ni `MediaSource` ni
`ManagedMediaSource`. Il valide donc le fallback natif, l'image, la conversion
et les commandes qu'il expose, mais ses assertions de seek remux/MSE et de
libération d'un pipe MSE sont explicitement ignorées. Ce résultat n'est pas une
validation de Safari sur matériel Apple ; elle reste à faire.

## Portes de sortie

La validation finale a passé : tests et vet Go, format, `govulncheck` sans
vulnérabilité atteignable, 43 tests unitaires frontend, contrôle Svelte sans
avertissement, 81 contrôles de mise en page avec trois skips historiques, 39
scénarios de lecture multi-navigateurs avec cinq skips de capacité documentés,
audit npm de l'application et du site sans vulnérabilité, build local complet
et six cross-builds `CGO_ENABLED=0`. L'agrégat reproductible des 68 fichiers de
source et de test frontend vaut
`D96C5BD82A77759CB8033BD53E22283E4055D95B1C15FE0E660C924BAFEC0B47`.

Les harnais d'updater et d'endurance ont été rejoués après le build final. Le
parcours Edge sur le vrai MKV a lui aussi été rejoué après les corrections de
cycle de vie : 1 920 × 804, 55 images avant seek, piste audio changée, seek à
3 997 s, pause à 3 996 s, reprise autour du GOP à 3 995 s, aucune erreur de
page et zéro FFmpeg avant/après.

Le workflow de tag exécute ces deux crash-tests sur `windows-latest` avant de
fabriquer les six assets. L'endurance télécharge elle-même le runtime épinglé et
vérifié ; elle ne dépend pas d'un FFmpeg opportuniste installé sur le runner.
Un échec empêche donc le job de build et, par dépendance, la publication.

Les cibles macOS/ARM64 et les familles GPU non présentes sur cette machine ne
sont que compilées. Safari physique, téléviseurs et endurance longue restent
des preuves terrain, pas des résultats inventés par le CI.
