# Modernisation du moteur - V3.2 preview 3

Mesures réalisées le 10 septembre 2026 sous Windows 11 sur un AMD Ryzen AI 9
HX 370. L'ancien exécutable local et le nouveau ont reçu chacun deux séries de
50 requêtes par route, après trois échauffements, sur deux copies identiques
d'une base synthétique de 10 000 films. Les valeurs ci-dessous sont la moyenne
des deux séries. Le script reproductible est `scripts/measure-engine.ps1`.

## Résultats mesurés

| Mesure | Avant | Après | Écart |
| --- | ---: | ---: | ---: |
| Liste de 500 films, p50 | 66,99 ms | 51,81 ms | **-22,7 %** |
| Liste de 500 films, p95 | 74,96 ms | 57,55 ms | **-23,2 %** |
| Accueil complet, p50 | 79,52 ms | 49,56 ms | **-37,7 %** |
| Accueil complet, p95 | 88,99 ms | 54,01 ms | **-39,3 %** |
| Recherche avec résultats, p50 | 22,85 ms | 21,54 ms | **-5,7 %** |
| Recherche avec résultats, p95 | 26,75 ms | 23,10 ms | **-13,6 %** |
| Recherche sans résultat, p50 | 33,34 ms | 33,80 ms | +1,4 % |
| Recherche sans résultat, p95 | 36,20 ms | 36,63 ms | +1,2 % |
| CPU total du scénario | 11,71 s | 11,12 s | **-5,1 %** |
| Démarrage | 616,80 ms | 631,65 ms | +2,4 % |
| Mémoire privée en fin de scénario | 67,14 Mio | 74,37 Mio | +10,8 % |
| Working set en fin de scénario | 38,44 Mio | 44,00 Mio | +14,5 % |
| Exécutable Windows AMD64 | 17,32 Mio | 17,40 Mio | +0,43 % |

Le temps de démarrage varie davantage entre les séries que l'écart moyen :
600–633 ms avant et 590–673 ms après. Il n'y a donc pas de gain de démarrage
démontré. Les instantanés mémoire sont stables dans leur direction mais restent
des mesures de processus prises après le scénario, pas une courbe de pic.

Les réponses JSON de la liste, de l'accueil et de la recherche ont été comparées
octet pour octet entre les deux exécutables. Les trois sont identiques, avec des
charges respectives de 381 091, 28 856 et 15 020 octets. Les gains ne viennent
pas d'un retrait de données dans l'API.

La taille et la mémoire n'ont pas baissé dans cette tranche. Le binaire gagne
78 336 octets et la mémoire privée finale environ 7,2 Mio. Ce coût paie notamment
la récupération automatique des mises à jour, la coordination des travaux et
les diagnostics persistants déjà présents dans la preview. Compresser
l'exécutable avec UPX aurait amélioré le chiffre de téléchargement au prix du
démarrage, des faux positifs antivirus et de mises à jour plus fragiles ; ce
n'est pas une optimisation acceptable.

## Changements du moteur

- Les lectures de listes SQLite ne chargent plus les gros champs réservés aux
  pages de détail. L'accueil lance ses lignes indépendantes sur un pool fixe de
  deux connexions et restitue leur ordre historique.
- Le watcher réutilise l'inventaire de fichiers qui a détecté le changement. Un
  changement de dossier ne déclenche plus immédiatement un second parcours
  complet du disque ou du partage réseau.
- Les conversions, inspections, recherches de keyframe et extractions de
  sous-titres partagent un coordinateur. Une lecture demandée par l'utilisateur
  interrompt proprement une génération de preview et cette preview reste
  réessayable.
- Les sorties de FFmpeg sont bornées en mémoire. Deux inspections simultanées du
  même fichier et de la même version partagent désormais un seul processus.
- La provenance du runtime FFmpeg est centralisée et exposée dans les
  diagnostics : release `b6.1.1`, asset de plateforme, URL et SHA-256. Le hash et
  la version déclarée sont contrôlés avant la première utilisation du processus.
- Une mise à jour crée un snapshot SQLite cohérent avec le WAL, sauvegarde la
  configuration et journalise les versions. Le nouvel exécutable n'efface le
  précédent qu'après avoir servi `/api/health` avec la bonne version et interrogé
  le catalogue. Une erreur de démarrage restaure la base, la configuration et
  l'ancien exécutable.
- Le build local peut réutiliser un `web-dist` déjà validé avec
  `-SkipFrontend`. La CI compile le frontend une seule fois puis embarque les
  mêmes octets dans les six cibles.
- Go est figé en 1.26.6. La CI ajoute `govulncheck` 1.8.0, `npm audit` et des
  propositions Dependabot groupées mensuelles sans fusion automatique.

## Vérifications finales

- `go test ./...` : réussite.
- `go vet ./...` : réussite.
- `govulncheck@v1.8.0 ./...` sous Go 1.26.6 : aucune vulnérabilité atteignable.
- `npm audit --audit-level=high` : aucune vulnérabilité.
- Tests unitaires du frontend : 34 réussites.
- Garde Playwright téléphone, bureau et télévision : 81 réussites, 3 scénarios
  explicitement ignorés par leur condition historique.
- Lecture réelle générée : 8 scénarios réussis, couvrant direct play, remux,
  conversion, seek, pause, pistes audio, sous-titres et reprise après erreur.
- Compilation `CGO_ENABLED=0` : Windows AMD64/ARM64, Linux AMD64/ARM64 et macOS
  AMD64/ARM64 réussies.
- FFmpeg local vérifié : SHA-256
  `04e1307997530f9cf2fe35cba2ca7e8875ca91da02f89d6c7243df819c94ad00`,
  encoders utilisables mesurés `h264_amf`, `h264_mf`, `libx264`.
- Profil CPU reproductible ajouté : liste de 500 films à environ 7,0 ms/op et
  recherche absente sur 10 000 films à environ 22,4 ms/op. SQLite puis le pliage
  des chaînes dominent le coût restant.
- Les 62 fichiers de source et de tests sous `web/src` et `web/tests` ont le
  même hash agrégé avant et après cette modernisation. Aucun frontend n'a été
  modifié par ce chantier.

Le renouvellement de FFmpeg, l'ajout de ffprobe, un runtime personnalisé, HLS,
les médias préconvertis et la purge automatique du cache restent hors de cette
preview. Ils changent la distribution ou le comportement produit et demandent
une validation séparée.
