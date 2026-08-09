// French catalogue. Keep it strictly shape-compatible with en.js: the site
// build walks both catalogues recursively and refuses a partial translation.
export default {
	lang: 'fr',
	dir: '',
	canonical: 'https://benitoow.github.io/theia-media/',
	ogLocale: 'fr_FR',
	other: { code: 'en', label: 'EN', href: 'en/' },

	meta: {
		title: 'Theia — Votre film. Pas la plateforme.',
		description:
			'Theia transforme les dossiers de votre appareil en médiathèque web personnelle, sans compte, abonnement ni cloud obligatoire.',
		ogAlt:
			'Theia, serveur média personnel : une capture réelle du lecteur et un téléchargement explicite par système et architecture.'
	},

	skip: 'Aller au contenu',
	nav: {
		label: 'Navigation principale',
		moments: 'L’expérience',
		difference: 'La différence',
		download: 'Télécharger'
	},

	hero: {
		eyebrow: 'Un serveur média. Un seul binaire.',
		title: 'Votre film. Pas la plateforme.',
		lead:
			'Choisissez vos dossiers. Theia organise votre bibliothèque et lit vos films sur les appareils de la maison — sans compte, abonnement ni cloud obligatoire.'
	},

	playerDemo: {
		status: 'Capture réelle · commandes de démonstration',
		title: 'Theia Demo',
		alt: 'Capture réelle du lecteur Theia montrant un paysage abstrait original, noir et or.',
		caption:
			'Capture réelle du lecteur Theia sur un média original créé pour cette page. Les commandes reproduisent le chrome interactif du lecteur.',
		play: 'Lire',
		pause: 'Mettre en pause',
		back10: 'Reculer de 10 secondes',
		forward10: 'Avancer de 10 secondes',
		mute: 'Couper le son',
		unmute: 'Rétablir le son',
		tracksOpen: 'Audio, sous-titres et qualité',
		tracksTitle: 'Audio, sous-titres et qualité',
		audio: 'Piste audio',
		quality: 'Qualité',
		subtitles: 'Sous-titres',
		loading: 'Préparation de la lecture…',
		position: 'Position dans le film',
		volume: 'Volume',
		noJs:
			'Sans JavaScript, la capture et toutes les informations restent visibles ; seules les commandes de démonstration sont statiques.',
		trackOptions: {
			audio: [
				['Automatique', ''],
				['Français', '5.1 · DTS']
			],
			quality: [
				['1080p', 'Fichier original'],
				['720p', 'Réencodé']
			],
			subtitles: [
				['Aucun', ''],
				['Français', 'SRT externe']
			]
		}
	},

	downloads: {
		eyebrow: 'Essayer maintenant',
		title: 'Votre système d’abord. L’architecture ensuite.',
		lead: 'Theia ne déclenche jamais un fichier deviné. Choisissez explicitement les deux.',
		statusIdle: 'Aucun système sélectionné.',
		statusSelected: 'Système sélectionné :',
		systems: { windows: 'Windows', macos: 'macOS', linux: 'Linux' },
		chooseArch: 'Choisir une architecture',
		download: 'Télécharger Theia pour',
		releaseNotes: 'Notes de version',
		shaLabel: 'Empreintes SHA-256',
		noJs: 'JavaScript est désactivé : les six fichiers restent disponibles ci-dessous.',
		warnings: {
			windows:
				'SmartScreen peut prévenir que l’éditeur est inconnu : Theia n’est pas signé. Vérifiez l’empreinte, puis choisissez « Informations complémentaires » et « Exécuter quand même ».',
			macos:
				'macOS peut bloquer l’ouverture : Theia n’est pas notarisé. Après vérification de l’empreinte, utilisez Réglages Système → Confidentialité et sécurité → Ouvrir quand même.',
			linux: 'Rendez le binaire exécutable avec chmod +x, puis lancez-le depuis votre terminal.'
		}
	},

	moments: {
		eyebrow: 'Trois moments, pas un inventaire',
		title: 'De vos dossiers au canapé. Puis plus loin.',
		lead: 'Theia se comprend par ce qu’il permet de faire, pas par une procession de captures.',
		items: [
			{
				label: 'DÉCOUVRIR',
				title: 'Vos dossiers deviennent une bibliothèque.',
				body:
					'Au premier lancement, vous indiquez où sont vos films et séries. Theia analyse, classe et présente ce qui est déjà sur cet appareil.',
				alt: 'Capture réelle de la bibliothèque Theia, avec des films présentés en cartes panoramiques.'
			},
			{
				label: 'REGARDER',
				title: 'Le lecteur fait le travail difficile.',
				body:
					'Lecture directe quand c’est possible, adaptation quand c’est nécessaire. La piste audio, les sous-titres et la qualité restent compréhensibles.',
				alt: ''
			},
			{
				label: 'ACCÉDER À DISTANCE',
				title: 'Un appareil autorisé. Pas un compte externe.',
				body:
					'Theia prépare un accès WireGuard pour retrouver votre bibliothèque hors de chez vous, sans héberger vos médias dans un service Theia.',
				alt: 'Capture réelle des réglages d’accès distant WireGuard dans Theia.'
			}
		],
		watchFacts: [
			['Audio', 'VF · DTS 5.1'],
			['Sous-titres', 'Français · SRT'],
			['Qualité', 'Original · 1080p']
		]
	},

	difference: {
		eyebrow: 'Pourquoi Theia',
		title: 'Moins d’écosystème. Plus de maîtrise.',
		items: [
			['Un binaire', 'Le serveur, l’interface, la base et le lecteur voyagent ensemble. Pas de pile Docker à assembler.'],
			['Pas de compte Theia', 'Votre bibliothèque reste sur votre machine. L’accès local ne dépend pas d’un service distant.'],
			['Le navigateur suffit', 'Téléviseur, téléphone ou ordinateur : ouvrez Theia, choisissez qui regarde, lancez le film.']
		],
		limitsTitle: 'Les limites, dites avant le téléchargement',
		limits: [
			'Theia est plus jeune et moins extensible que Plex ou Jellyfin.',
			'Le transcodage dépend des codecs et de la puissance de votre machine.',
			'Les sous-titres image comme PGS ou VobSub sont signalés mais ne sont pas rendus.'
		],
		compare: 'Lire la comparaison détaillée sur GitHub',
		safetyTitle: 'Sûr par défaut, à condition de ne pas improviser le réseau.',
		safetyBody:
			'Theia est pensé pour votre réseau local. Ne publiez pas son port d’administration sur Internet : utilisez l’accès distant WireGuard intégré. Le code est GPL-3.0 et chaque binaire publié possède une empreinte SHA-256.'
	},

	closing: {
		eyebrow: 'Prêt à essayer',
		title: 'Votre bibliothèque est déjà là.',
		body: 'Il reste à choisir le binaire qui correspond vraiment à votre machine.',
		cta: 'Choisir mon système'
	},

	faq: {
		eyebrow: 'Trois réponses utiles',
		title: 'Avant de lancer le binaire',
		items: [
			[
				'Mes fichiers quittent-ils ma machine ?',
				'Non pour le fonctionnement normal : Theia lit vos dossiers et sert l’interface depuis cet appareil. Les métadonnées de films et séries peuvent être recherchées auprès de TMDB.'
			],
			[
				'Pourquoi Windows ou macOS affiche-t-il un avertissement ?',
				'Les binaires publiés ne sont pas encore signés pour SmartScreen ni notarisés par Apple. L’avertissement est attendu ; vérifiez l’empreinte SHA-256 avant de continuer.'
			],
			[
				'Est-ce un remplacement complet de Plex ou Jellyfin ?',
				'Pas pour tout le monde. Theia vise une installation plus directe et un usage domestique clair. Il assume un écosystème plus petit et moins d’extensions.'
			]
		]
	},

	footer: {
		body:
			'Theia est un logiciel libre sous GPL-3.0. Lecture et transcodage : FFmpeg, sous sa propre licence.',
		tmdb: 'This product uses the TMDB API but is not endorsed or certified by TMDB.',
		navLabel: 'Liens légaux et projet',
		source: 'Code source',
		license: 'Licence GPL-3.0'
	}
};
