// French catalogue. The site ships in French and English for the same reason
// the application does; a new language is a new file beside this one, never a
// hunt through the template.
export default {
	lang: 'fr',
	dir: '',
	other: { code: 'en', label: 'English', href: 'en/' },

	title: 'Theia — votre médiathèque, un seul binaire',
	description:
		'Un serveur multimédia personnel dans un seul binaire Go. Films et séries, métadonnées TMDB, transcodage, profils et accès distant WireGuard. Aucune configuration, aucun compte, aucun abonnement.',

	nav: { features: 'Ce que c’est', shots: 'À l’écran', download: 'Télécharger', github: 'GitHub' },

	hero: {
		kicker: 'Serveur multimédia personnel',
		title: 'Vos films.\nUn binaire.\nVotre réseau.',
		lede: 'Theia transforme des dossiers de films et de séries en une médiathèque que l’on ouvre depuis la télévision, le téléphone ou l’ordinateur déjà présents dans la maison. Pas de compte, pas d’abonnement, pas de Docker, pas d’interface à installer à côté. Vous lancez le binaire, vous choisissez les dossiers, vous regardez.',
		cta: 'Télécharger',
		ctaOther: 'Toutes les plateformes',
		source: 'Voir le code',
		meta: 'GPL-3.0 · Windows, macOS, Linux · 17 Mo'
	},

	claims: {
		title: 'Trois choses, et elles tiennent',
		items: [
			{
				k: 'Un fichier',
				t: 'Rien à installer autour',
				d: 'Le serveur, l’interface, la base de données et le lecteur sont dans le même exécutable. La seule dépendance externe est FFmpeg, que Theia télécharge lui-même, épinglé et vérifié par empreinte SHA-256.'
			},
			{
				k: 'Aucun compte',
				t: 'Rien à créer, rien à connecter',
				d: 'Pas d’inscription, pas de bibliothèque dans le nuage, pas de plan payant qui débloque le matériel de votre propre machine. Les seules connexions sortantes vont à TMDB pour les métadonnées et à GitHub pour les mises à jour.'
			},
			{
				k: 'Chez vous',
				t: 'Les fichiers ne bougent pas',
				d: 'Vos films restent là où ils sont, sur vos disques. Theia les lit, les indexe et les diffuse sur votre réseau. Rien n’est téléversé, rien n’est analysé ailleurs, rien n’est mesuré.'
			}
		]
	},

	shots: {
		title: 'À l’écran',
		lede: 'L’interface est en français par défaut ; l’anglais est complet et bascule sans recharger. Les affiches, résumés et vignettes viennent de TMDB.',
		home: 'L’accueil propose de reprendre le film laissé en cours, puis ce qui est commencé, récent, le mieux noté, et une suggestion qui change chaque jour.',
		library: 'La bibliothèque : recherche sur le titre, le réalisateur, le genre et l’année, cinq tris, filtres par genre et par état de visionnage.',
		series: 'Les séries vivent à côté des films, dans la même grille.',
		serie: 'Une série, saison par saison, avec la reprise gardée épisode par épisode.',
		settings: 'L’accès distant : un tunnel WireGuard embarqué, éteint tant que vous ne l’allumez pas.',
		onboarding: 'Au premier lancement, une adresse et un QR code. Il n’y a rien d’autre à configurer.',
		profiles: 'Chacun sa progression. Un nom, une photo, aucun mot de passe.'
	},

	does: {
		title: 'Ce que Theia fait',
		groups: [
			{
				t: 'La bibliothèque',
				items: [
					'Analyse un ou plusieurs dossiers et tient un catalogue SQLite local.',
					'Lit un titre et une année dans des noms de fichiers ordinaires. Un fichier apparaît même quand l’analyse échoue.',
					'Regroupe plusieurs fichiers sous un même film : un remux et un encodage 1080p font une fiche, pas deux.',
					'Gère les séries : saisons, épisodes, reprise par épisode.',
					'Récupère titres, résumés, affiches, durées, notes, réalisateur, genres et distribution depuis TMDB, et met les images en cache.'
				]
			},
			{
				t: 'Le visionnage',
				items: [
					'Un accueil construit autour de ce que vous regardiez.',
					'Un lecteur qui laisse choisir la piste audio, les sous-titres et la qualité pendant le film.',
					'Des sous-titres pris dans le fichier ou dans le .srt posé à côté, dessinés par Theia pour qu’ils tombent sur l’image et non dans les bandes noires.',
					'La position enregistrée en continu, et une recherche qui fonctionne même sur un flux remuxé.'
				]
			},
			{
				t: 'La maison',
				items: [
					'Des profils : un nom, une photo locale facultative, une progression séparée. Aucun mot de passe, aucun rôle.',
					'Un accès distant WireGuard embarqué, avec appairage à usage unique et révocation par appareil. Aucun relais, aucun serveur de rendez-vous.',
					'Deux langues, le français par défaut, le choix appartenant à chaque navigateur.'
				]
			},
			{
				t: 'L’exploitation',
				items: [
					'Une adresse et un QR code au premier lancement.',
					'Des mises à jour vérifiées par empreinte, refusées pendant une lecture, et réversibles si le nouveau binaire ne démarre pas.'
				]
			}
		]
	},

	refuses: {
		title: 'Ce que Theia refuse',
		lede: 'La moitié de l’argument. Ces absences sont des décisions, écrites et datées, pas une feuille de route.',
		items: [
			['Des comptes et des permissions', 'Un foyer n’est pas une organisation. Le port local n’a pas d’authentification, et c’est délibéré.'],
			['La télévision en direct et les enregistrements', 'Un autre produit, avec ses tuners et ses grilles.'],
			['Un système de greffons', 'La surface d’extension d’un logiciel est aussi sa surface de panne.'],
			['La musique et les photos', 'Deux médiathèques de plus, deux fois le travail, aucun rapport avec un film.'],
			['Des applications natives', 'Le navigateur de votre télévision existe déjà.'],
			['La moindre mesure d’usage', 'Rien n’est compté, rien n’est envoyé.']
		]
	},

	numbers: {
		title: 'Mesuré, pas estimé',
		lede: 'Relevé sur la machine du mainteneur, sur une bibliothèque réelle. Aucun chiffre ici n’est une estimation.',
		items: [
			['17 Mo', 'le binaire, interface comprise'],
			['18 Mo', 'de mémoire vive au repos'],
			['22 Mo', 'pendant un remux soutenu'],
			['0,8 Mo', 'la base SQLite'],
			['0', 'conteneur, service, dépendance système'],
			['1', 'dépendance externe : FFmpeg, téléchargé et vérifié']
		]
	},

	download: {
		title: 'Télécharger',
		lede: 'Choisissez votre plateforme. Le lien pointe toujours sur la dernière version publiée.',
		detected: 'Détecté sur cet appareil',
		all: 'Toutes les plateformes',
		digest:
			'GitHub publie une empreinte SHA-256 pour chaque fichier, sur la page de la version. C’est la même empreinte que le mécanisme de mise à jour de Theia vérifie avant d’installer quoi que ce soit.',
		steps: {
			title: 'Après le téléchargement',
			windows: [
				'Lancez <code>theia.exe</code>. Windows SmartScreen prévient qu’il ne connaît pas l’éditeur : <em>Informations complémentaires</em>, puis <em>Exécuter quand même</em>.',
				'Une adresse et un QR code s’affichent. Ouvrez-les depuis n’importe quel appareil du réseau.'
			],
			macos: [
				'<code>chmod +x theia-darwin-arm64</code> puis lancez-le depuis le terminal.',
				'macOS bloque un binaire non notarisé : <em>Réglages Système → Confidentialité et sécurité → Ouvrir quand même</em>.'
			],
			linux: [
				'<code>chmod +x theia-linux-amd64</code> puis <code>./theia-linux-amd64</code>.',
				'Aucun paquet, aucun service à déclarer. Un <code>systemd</code> si vous en voulez un, jamais parce qu’il le faut.'
			]
		},
		warning: {
			title: 'Avant de l’exposer',
			body:
				'Theia n’a pas d’authentification sur son port local, et c’est une décision. N’ouvrez jamais le port 8383 sur votre box. Pour y accéder de l’extérieur, servez-vous de l’accès distant intégré : un tunnel WireGuard séparé, une clé par appareil, et des droits de lecture seule.'
		}
	},

	compare: {
		title: 'Face aux autres',
		lede: 'Les lignes qui vont contre Theia restent. Elles sont la raison pour laquelle il est petit.',
		head: ['', 'Theia', 'Plex', 'Jellyfin', 'Emby'],
		rows: [
			['Un seul binaire, sans dépendance', 'Oui', 'Non', 'Non', 'Non'],
			['Aucun compte pour un usage local', 'Oui', 'Compte requis', 'Oui', 'Oui'],
			['Aucune fonction payante', 'Oui', 'Pass Plex', 'Oui', 'Emby Premiere'],
			['Aucune mesure d’usage', 'Oui', 'Non', 'Oui', 'Partielle'],
			['Films et séries', 'Oui', 'Oui', 'Oui', 'Oui'],
			['Accès distant sans service tiers', 'Oui', 'Via Plex', 'Manuel', 'Via Emby'],
			['Télévision en direct et enregistrement', 'Non', 'Oui', 'Oui', 'Oui'],
			['Musique, photos, livres', 'Non', 'Oui', 'Oui', 'Oui'],
			['Applications natives TV et mobile', 'Non', 'Oui', 'Oui', 'Oui'],
			['Greffons et extensions', 'Non', 'Oui', 'Oui', 'Oui'],
			['Plusieurs utilisateurs avec droits', 'Non', 'Oui', 'Oui', 'Oui']
		],
		note: 'Établi en août 2026 à partir de la documentation publique de chaque projet, et mesuré sur Theia lui-même.'
	},

	footer: {
		licence: 'Theia est un logiciel libre sous licence GNU General Public License v3.0.',
		tmdb: 'Ce produit utilise l’API TMDB mais n’est ni approuvé ni certifié par TMDB.',
		credits:
			'FFmpeg est téléchargé à la demande et reste sous sa propre licence. Inter et Playfair Display sont servis localement sous SIL Open Font License 1.1.',
		links: { repo: 'Code source', releases: 'Versions', issues: 'Signaler un problème', security: 'Sécurité' }
	}
};
