# Documentation de Theia

Ce dossier sépare les documents qui gouvernent encore le produit, les notes de
version destinées aux utilisateurs et les campagnes techniques conservées comme
preuves. Une mesure historique reste utile ; elle n'a simplement pas à se faire
passer pour la documentation courante.

## À lire avant de modifier Theia

| Document | Rôle |
|---|---|
| [`spec-fondatrice.md`](spec-fondatrice.md) | Ce que Theia est et refuse de devenir |
| [`DECISIONS.md`](DECISIONS.md) | Les décisions techniques et produit, avec leur raisonnement. Chaque entrée porte son statut et ses sujets ; l'index en tête dit ce qui gouverne encore |
| [`design-system.md`](design-system.md) | Les règles visuelles et d'interaction |
| [`v3.md`](v3.md) | Le périmètre livré et vérifié de la génération V3 |
| [`v3.3.md`](v3.3.md) | L'historique de la génération native introduite en V3.3 |
| [`v3.4-release-readiness.md`](v3.4-release-readiness.md) | La publication 3.4, les six plateformes et les vérifications des téléchargements publics |
| [`v4.md`](v4.md) | La direction acceptée pour V4 et les points à spécifier |
| [`field-testing.md`](field-testing.md) | Le protocole de validation dans de vrais foyers |

## Notes de version

- [`Theia 3.4`](releases/v3.4.0.md) — dernière version V3 : six applications
  desktop, fiches films/séries natives, progression des épisodes et nouvelle
  politique de support. Les fonctions V4 sont des projets, pas des fonctions livrées.
- [`Theia 3.3.6`](releases/v3.3.6.md) — fin de la maintenance 3.3, installation
  et identification du dossier de données ; la vérification des images Mac
  a ensuite été renforcée en 3.4.
- [`Theia 3.3.5`](releases/v3.3.5.md) - les réglages du lecteur portent enfin sur
  le film - Lecture et Sous-titres, mesurés sur le moteur lui-même - et le lecteur
  cesse d'envoyer des trames que personne ne lit.
- [`Theia 3.3.4`](releases/v3.3.4.md) - la génération native atteint Apple
  Silicon : le lecteur y dessine le film lui-même, l'archive s'exécute telle
  qu'elle est téléchargée.
- [`Theia 3.3.3`](releases/v3.3.3.md) - les réglages deviennent un rail et un
  panneau, et le bundle hors-ligne cesse de servir un serveur qui n'est pas le
  produit.
- [`Theia 3.3.2`](releases/v3.3.2.md) - la commande `theia`, le `PATH`, App
  Paths, et un `--uninstall` qui reprend ce que l'installation a écrit.
- [`Theia 3.3.1`](releases/v3.3.1.md) - première maintenance de la génération
  native : les alias d'assets V3.2 disparaissent, Windows x64 reste le seul
  installeur publié.
- [`Theia 3.3.0`](releases/v3.3.0.md) - la génération native : trois artefacts
  exécutables, une identité visuelle.
- [`Theia 3.2`](releases/v3.2.0.md) - refonte du moteur de lecture, fiabilité,
  performances et mises à jour transactionnelles.
- [`Theia 3.1`](releases/v3.1.0.md) - dernière note de la phase précédente.
- [`Theia 3.0`](releases/v3.0.0.md) - arrivée de la génération V3.

## Archives techniques

- [`V3.2`](archive/v3.2/README.md) - plans de travail, mesures A/B, validations
  matérielles et campagnes sur médias réels.
- [`V3.3`](archive/v3.3/README.md) - journaux, mesures et preuves de la campagne
  qui a construit la génération native (phases 0 à 7).
- [`V2`](archive/README.md) - roadmap et documents de coordination du cycle V2.

Les dossiers `archive/v3.4/` conservent les mesures et références visuelles du
cycle 3.4. Le bilan public courant reste le registre de publication ci-dessus.

Les images utilisées par la documentation publique vivent dans
[`screenshots/`](screenshots/). Elles ne définissent pas à elles seules le
comportement actuel de l'application.
