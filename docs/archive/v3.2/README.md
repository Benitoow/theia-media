# Archive technique de Theia 3.2

Ces documents racontent la construction de la 3.2 : hypothèses, plans,
benchmarks, résultats intermédiaires et validations sur de vrais médias. Ils
sont conservés comme preuves et comme historique de raisonnement. La synthèse
destinée aux utilisateurs se trouve dans la
[note de version 3.2](../../releases/v3.2.0.md).

## Parcours conseillé

| Document | Contenu |
|---|---|
| [`plan-modernisation-moteur.md`](plan-modernisation-moteur.md) | Audit initial et ordre de modernisation du moteur |
| [`plan-refonte-lecture.md`](plan-refonte-lecture.md) | Plan détaillé de la refonte du parcours de lecture |
| [`engine-modernisation-3.2-preview3.md`](engine-modernisation-3.2-preview3.md) | SQLite, bibliothèque et premières mesures avant/après |
| [`engine-modernisation-3.2-preview4.md`](engine-modernisation-3.2-preview4.md) | Service de lecture, premier octet, cycle de vie et matériel |
| [`engine-modernisation-3.2-preview5.md`](engine-modernisation-3.2-preview5.md) | Endurance, updater réel, navigateurs et fermeture du chantier |
| [`hardware-measurements-tranche-6.md`](hardware-measurements-tranche-6.md) | Mesures des encodeurs et décodeurs disponibles sur la machine AMD |
| [`real-media-validation.md`](real-media-validation.md) | Validation de bout en bout sur le remux UHD HDR de 53,79 Gio |
| [`ffmpeg-8.1.2-validation.md`](ffmpeg-8.1.2-validation.md) | Qualification de FFmpeg 8.1.2-4 et correction du tampon Edge |

## Ce qui reste courant

Les règles du produit ne sont pas archivées ici. Pour comprendre l'état
présent, lire [`../../spec-fondatrice.md`](../../spec-fondatrice.md),
[`../../DECISIONS.md`](../../DECISIONS.md),
[`../../design-system.md`](../../design-system.md) et
[`../../v3.md`](../../v3.md).
