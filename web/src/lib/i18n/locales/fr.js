export const metadata = {
	code: 'fr',
	label: 'Français',
	htmlLang: 'fr',
	localeTag: 'fr-FR'
};

const decimalFormatter = new Intl.NumberFormat(metadata.localeTag, {
	minimumFractionDigits: 1,
	maximumFractionDigits: 1
});

export function formatDecimal(value) {
	return decimalFormatter.format(value);
}

export const strings = {
	appName: 'Theia',
	tagline: 'Serveur média personnel',
	tmdbAttribution: 'This product uses the TMDB API but is not endorsed or certified by TMDB.',

	a11y: {
		mainNavigation: 'Navigation principale'
	},

	nav: {
		home: 'Accueil',
		library: 'Films',
		settings: 'Réglages',
		back: 'Retour'
	},

	library: {
		title: 'Tous les films',
		loading: 'Chargement de la bibliothèque…',
		loadingProgress: (n) =>
			n > 0 ? `Chargement de la bibliothèque… ${n} films reçus` : 'Chargement de la bibliothèque…',
		search: 'Rechercher',
		searchPlaceholder: 'Titre, réalisateur, genre…',
		clear: 'Effacer',
		sort: 'Trier',
		sortTitle: 'Titre',
		sortYear: 'Plus récents',
		sortRating: 'Mieux notés',
		sortAdded: 'Ajoutés récemment',
		sortRuntime: 'Durée',
		genre: 'Genre',
		allGenres: 'Tous les genres',
		status: 'Visionnage',
		statusAll: 'Tous',
		statusUnseen: 'Jamais commencés',
		statusInProgress: 'En cours',
		statusFinished: 'Terminés',
		countAll: (n) => (n === 1 ? '1 film' : `${n} films`),
		countFiltered: (shown, total) => `${shown} sur ${total} films`,
		noResults: 'Aucun film ne correspond',
		noResultsBody: 'Essayez un autre mot, ou retirez un filtre.',
		reset: 'Tout réafficher',
		seeAll: 'Tout voir',
		scrollPrevious: 'Faire défiler la rangée vers la gauche',
		scrollNext: 'Faire défiler la rangée vers la droite',
		ratingLegend: (rating) => `${formatDecimal(rating)} / 10`,
		unwatchedBadge: 'Jamais lu',
		finishedBadge: 'Vu'
	},

	hero: {
		details: 'Voir la fiche',
		resumeEyebrow: 'Vous étiez en train de regarder',
		resume: 'Reprendre',
		remaining: (duration) => `Il reste ${duration}`
	},

	rows: {
		continue: {
			title: 'Continuer à regarder',
			href: '/films?status=progress'
		},
		recent: {
			title: 'Récemment ajoutés',
			href: '/films?sort=added'
		},
		top_rated: {
			title: 'Les mieux notés',
			href: '/films?sort=rating'
		},
		tonight: {
			title: 'Au hasard ce soir',
			href: null,
			hint: 'Une sélection qui change chaque jour'
		}
	},

	home: {
		loading: 'Chargement de la bibliothèque…',
		emptyTitle: 'Aucun film, pour l’instant',
		emptyBody:
			'Ajoutez un ou plusieurs dossiers depuis les réglages, puis lancez une analyse. Theia s’occupe du reste.',
		unreachableTitle: 'Serveur injoignable',
		unreachable:
			"La page est chargée mais l'API ne répond pas. Le serveur a probablement été arrêté.",
		retry: 'Réessayer'
	},

	welcome: {
		eyebrow: 'Premier lancement',
		title: 'Theia est prêt',
		body:
			"Scannez ce code avec un téléphone ou une tablette du même réseau, ou tapez l'adresse à la main sur une TV. Il n'y a rien à configurer.",
		enter: 'Voir la bibliothèque'
	},

	problems: {
		directory_unreadable:
			"Dossier illisible. Si c'est un disque externe, il n'est probablement pas branché.",
		not_a_directory: "Ce chemin désigne un fichier, pas un dossier.",
		subdirectory_unreadable: 'Sous-dossier ignoré : Theia n’a pas pu le lire.',
		file_unreadable: 'Fichier ignoré : Theia n’a pas pu lire ses informations.',
		save_failed: "Ce fichier n'a pas pu être enregistré dans la bibliothèque.",
		metadata_unavailable:
			'Les métadonnées n’ont pas pu être récupérées pour cette analyse. Elles seront réessayées.',
		metadata_key_rejected:
			'TMDB a refusé la clé API. Vérifiez-la dans les réglages ; les films restent listés sans affiche.',
		unknown: 'Un problème inattendu est survenu pendant l’analyse.'
	},

	updateReasons: {
		up_to_date: 'Theia est à jour.',
		no_release: 'Aucune version n’a encore été publiée.',
		github_unreachable: 'GitHub est injoignable pour l’instant. Réessayez plus tard.',
		playback_active: 'Une lecture est en cours.',
		no_binary_for_platform: 'Cette version ne propose pas de binaire pour ce système.',
		download_not_verified:
			"Le fichier téléchargé ne correspond pas à l'empreinte publiée. Rien n'a été installé.",
		binary_did_not_run:
			"Le fichier téléchargé n'a pas démarré correctement. Rien n'a été installé.",
		replace_failed:
			"Le remplacement du binaire a échoué. La version précédente a été remise en place.",
		development_build: ''
	},

	update: {
		heading: 'Mise à jour',
		current: 'Version installée',
		latest: 'Dernière version publiée',
		check: 'Vérifier maintenant',
		checking: 'Vérification…',
		install: 'Installer',
		installing: 'Installation…',
		upToDate: 'Theia est à jour.',
		available: 'Une nouvelle version est disponible.',
		ready: 'Mise à jour installée. Theia redémarre — rechargez la page dans quelques secondes.',
		deferred:
			"Mise à jour reportée : une lecture est en cours. Elle sera proposée à nouveau une fois l'écran libre.",
		failed:
			"La mise à jour a échoué. La version installée n'a pas été touchée et continue de fonctionner.",
		unsupported:
			"Cette version ne se met pas à jour toute seule : elle a été compilée localement et ne peut pas se comparer à une version publiée.",
		notes: 'Notes de version',
		lastChecked: 'Dernière vérification'
	},

	connect: {
		heading: 'Connecter un appareil',
		address: 'Adresse sur le réseau local',
		copy: 'Copier',
		copied: 'Copié',
		mdns: 'Également joignable à',
		mdnsCaveat:
			"Ce nom ne fonctionne pas sur Android — utilisez l'adresse IP ci-dessus dans ce cas.",
		otherAddresses: 'Autres adresses de cette machine',
		otherAddressesHint:
			"Si le code ne mène nulle part, cette machine a plusieurs cartes réseau et Theia a peut-être choisi la mauvaise. Essayez l'une de ces adresses.",
		virtual: 'carte virtuelle'
	},

	player: {
		play: 'Lire',
		pause: 'Pause',
		back10: 'Reculer de 10 secondes',
		forward10: 'Avancer de 10 secondes',
		position: 'Position dans le film',
		volume: 'Volume',
		mute: 'Couper le son',
		unmute: 'Rétablir le son',
		fullscreen: 'Plein écran',
		close: 'Fermer le lecteur',

		resume: 'Reprendre',
		resumeAtMinutes: (minutes) => `Reprendre à ${minutes} min`,
		resumeAt: 'Vous vous étiez arrêté à',
		fromStart: 'Reprendre du début',
		continueWatching: 'Continuer à regarder',

		buffering: 'Mise en mémoire tampon…',
		seeking: 'Repositionnement…',
		remuxBadge: 'Réencapsulé à la volée',
		preparing:
			'Préparation de la lecture — ffmpeg est téléchargé une seule fois, cela peut prendre un moment.',

		unavailable: "La lecture n'a pas pu être préparée.",
		noFfmpeg:
			"Ce fichier doit être réencapsulé, mais aucune version de ffmpeg n'est disponible pour cette plateforme.",
		failed:
			"Ce fichier n'a pas pu être lu. Son format n'est probablement pas pris en charge par la v1.",

		// Le serveur envoie un code, jamais une phrase (décision 25). Cette table
		// est le seul endroit où un code de M1 devient du français.
		codes: {
			invalid_movie_id: "Ce film n'est pas identifiable.",
			invalid_file_id: "Ce fichier n'est pas identifiable.",
			invalid_audio_track_id: "Cette piste audio n'est pas identifiable.",
			audio_selection_requires_remux:
				'Cette piste ne peut être garantie que par un réencapsulage.',
			file_outside_library:
				"Ce fichier ne se trouve plus dans un dossier surveillé. Vérifiez les dossiers de la bibliothèque dans les réglages.",
			movie_not_found: 'Ce film est introuvable.',
			file_not_found: "Ce fichier n'appartient plus à ce film.",
			audio_track_not_found: "Cette piste audio n'existe plus dans ce fichier.",
			media_file_unavailable:
				'Ce fichier a disparu du disque. Relancez une analyse depuis les réglages.',
			media_not_inspected: 'Analysez ce fichier avant de choisir une piste audio.',
			media_unreadable:
				"Ce fichier n'a pas pu être décodé. Il est peut-être incomplet ou corrompu.",
			video_transcode_required:
				'Le format vidéo de ce fichier demande une conversion complète, prévue pour un jalon ultérieur.',
			movie_unavailable: 'Le film ne peut pas être lu depuis la base de données.',
			file_unavailable: 'Le fichier ne peut pas être lu depuis la base de données.',
			audio_track_unavailable: 'La piste audio ne peut pas être lue depuis la base de données.',
			media_file_unreadable: "Ce fichier n'a pas pu être ouvert sur le disque.",
			media_inspection_not_saved: "Le résultat de l'analyse n'a pas pu être enregistré.",
			stream_start_failed: "La lecture n'a pas pu démarrer.",
			ffmpeg_unsupported: "Aucune version de ffmpeg n'est disponible pour cette plateforme.",
			ffmpeg_unavailable:
				"ffmpeg n'a pas pu être préparé. Vérifiez la connexion, puis réessayez."
		},

		shortcuts: {
			open: 'Afficher les raccourcis clavier',
			close: 'Fermer l’aide des raccourcis',
			title: 'Raccourcis clavier',
			intro: 'Ces touches fonctionnent pendant la lecture, sans ouvrir les contrôles.',
			items: [
				{ keys: ['Espace'], action: 'Lecture ou pause' },
				{ keys: ['←', '→'], action: 'Reculer ou avancer de 10 secondes' },
				{ keys: ['↓', '↑'], action: 'Baisser ou augmenter le volume' },
				{ keys: ['M'], action: 'Couper ou rétablir le son' },
				{ keys: ['F'], action: 'Entrer ou sortir du plein écran' },
				{ keys: ['Début', 'Fin'], action: 'Aller au début ou à la fin du film' },
				{ keys: ['?'], action: 'Afficher ou masquer cette aide' }
			],
			scrubHint:
				'Sur la barre de lecture, Maj avec ← ou → saute de 60 secondes.'
		}
	},

	film: {
		notFound: 'Ce film est introuvable.',
		overview: 'Synopsis',
		cast: 'Distribution',
		director: 'Réalisation',
		genres: 'Genres',
		runtime: 'Durée',
		year: 'Année',
		progress: 'Progression dans le film',
		files: {
			one: 'Fichier',
			many: 'Fichiers disponibles',
			choose: 'Choisir le fichier à lire',
			chosen: 'Fichier choisi',
			primary: 'Principal',
			pending: 'Caractéristiques non mesurées',
			inspect: 'Analyser',
			inspecting: 'Analyse en cours…',
			retry: "Relancer l'analyse",
			errored: "Ce fichier n'a pas pu être analysé.",
			resolution: (width, height) => `${width} × ${height}`,
			trackCount: (count) => (count > 1 ? `${count} pistes audio` : `${count} piste audio`),
			hint:
				'Theia ne choisit jamais une qualité à votre place : ce film possède plusieurs ' +
				'fichiers et vous décidez lequel lire.'
		},
		audio: {
			title: 'Piste audio',
			choose: 'Choisir la piste audio',
			auto: 'Piste par défaut du fichier',
			isDefault: 'par défaut',
			unnamed: (index) => `Piste ${index}`,
			remuxNote:
				'Choisir une piste explicitement passe par un réencapsulage, seul moyen de garantir ' +
				"celle que le navigateur jouera."
		},
		moreByDirector: (director) => `Voir les autres films de ${director}`,
		noOverview: "Aucun synopsis n'est disponible pour ce film.",
		unmatched:
			"Ce fichier n'a pas été identifié sur TMDB. Il reste listé sous le nom que porte le fichier ; " +
			'renommer celui-ci relance une recherche à la prochaine analyse.'
	},

	profiles: {
		question: 'Qui regarde ?',
		defaultName: 'Profil principal',
		manage: 'Gérer les profils',
		done: 'Terminé',
		switch: 'Changer de profil',
		current: (name) => `Profil actif : ${name}`,
		choose: (name) => `Regarder en tant que ${name}`,
		add: 'Ajouter un profil',
		create: 'Créer le profil',
		cancel: 'Annuler',
		nameLabel: 'Nom du profil',
		namePlaceholder: 'Mimi',
		edit: (name) => `Modifier ${name}`,
		editShort: 'Modifier',
		save: 'Enregistrer',
		picture: 'Photo',
		pictureChange: 'Choisir une photo',
		pictureRemove: 'Retirer la photo',
		pictureHint:
			'La photo reste sur cette machine. Elle est recadrée en carré et ré-enregistrée ; ' +
			'aucune donnée du fichier d’origine n’est conservée.',
		pictureUploading: 'Envoi de la photo…',
		details: 'Détails',
		createdAt: 'Créé le',
		moviesStarted: 'Films commencés',
		moviesFinished: 'Films terminés',
		episodesStarted: 'Épisodes commencés',
		episodesFinished: 'Épisodes terminés',
		lastWatched: 'Dernière lecture',
		never: 'Jamais',
		delete: 'Supprimer ce profil',
		deleteConfirm: (name) =>
			`Supprimer ${name} ? Sa progression sera perdue. Les autres profils ne changent pas.`,
		deleteYes: 'Supprimer',
		empty: 'Aucun profil.',
		loading: 'Chargement des profils…',
		codes: {
			invalid_profile_id: "Ce profil n'est pas identifiable.",
			invalid_profile_payload: "La demande n'a pas pu être lue.",
			invalid_profile_name:
				'Un nom est requis, sans caractère de contrôle, et ne peut pas dépasser 40 caractères.',
			profile_not_found: "Ce profil n'existe plus. Il a peut-être été supprimé ailleurs.",
			profile_image_not_found: "Ce profil n'a pas de photo.",
			profile_limit_reached: 'Le nombre maximal de profils est atteint.',
			profile_last_remaining: 'Le dernier profil ne peut pas être supprimé.',
			profile_image_too_large: 'Cette image est trop lourde.',
			profile_image_unreadable: "Ce fichier n'est pas une image utilisable.",
			profile_unavailable: 'Les profils ne sont pas disponibles pour le moment.'
		}
	},

	settings: {
		heading: 'Réglages',
		interface: 'Interface',
		language: 'Langue',
		languageHint:
			'Ce choix reste dans ce navigateur. Il traduit l’interface sans retélécharger ni retraduire les métadonnées TMDB déjà en cache.',
		server: 'Serveur',
		version: 'Version',
		port: 'Port',
		hostname: 'Nom mDNS',
		dataDir: 'Dossier de données',
		library: 'Bibliothèque',
		paths: 'Dossiers surveillés',
		noPaths: 'Aucun dossier surveillé.',
		films: 'Films',
		scan: 'Analyser les dossiers',
		scanning: 'Analyse en cours…',
		lastScan: 'Dernière analyse',
		found: 'trouvés',
		added: 'ajoutés',
		updated: 'mis à jour',
		removed: 'retirés',
		enriched: 'enrichis',
		notFound: 'non identifiés',
		problems: 'Problèmes rencontrés',
		metadata: 'Métadonnées',
		source: 'Source',
		keySources: {
			settings: 'réglages',
			'built-in': 'clé intégrée',
			'config.local.json': 'config.local.json',
			none: 'aucune',
			unknown: 'inconnue'
		},
		noKeyAdvice:
			'Aucune clé TMDB n’est configurée : les films restent listés d’après leur nom de fichier, sans affiche ni synopsis. Ajoutez une clé personnelle ci-dessous pour activer les métadonnées.',

		edit: 'Modifier',
		cancel: 'Annuler',
		save: 'Enregistrer',
		saving: 'Enregistrement…',
		saved: 'Réglages enregistrés.',
		saveFailed: "Les réglages n'ont pas pu être enregistrés.",
		invalidPort: 'Le port doit être un nombre entre 1 et 65535.',
		addPath: 'Ajouter un dossier',
		removePath: 'Retirer',
		pathPlaceholder: 'C:\\Users\\vous\\Videos',
		portHint: 'Le changement de port ne prendra effet qu’au prochain démarrage de Theia.',
		portChanged:
			'Le nouveau port est enregistré, mais Theia écoute toujours sur l’ancien. Redémarrez-le pour appliquer.',
		missingPaths:
			'Enregistré, mais ces dossiers sont introuvables pour l’instant — normal si le disque n’est pas branché :',
		keyLabel: 'Clé TMDB personnelle',
		keyHint:
			'Facultatif. Laissez vide pour utiliser la clé fournie avec Theia. Une clé saisie ici est prioritaire.',
		keyPlaceholder: 'Laisser vide pour utiliser la clé intégrée',
		milestone: 'Theia v1'
	},

	errors: {
		scanFailed: "L'analyse n'a pas pu être menée à son terme.",
		scanBusy: 'Une analyse est déjà en cours.'
	},

	notFound: {
		eyebrow: 'Page introuvable',
		title: 'Cette adresse ne mène nulle part',
		body:
			"Le lien est peut-être incomplet, ou la page a changé de nom. La bibliothèque, elle, n'a pas bougé.",
		crash: 'Une erreur inattendue',
		crashBody:
			"L'interface s'est arrêtée sur une erreur. Recharger la page suffit presque toujours ; si le problème revient, le serveur est peut-être à redémarrer.",
		home: 'Retour à l’accueil',
		reload: 'Recharger'
	}
};

export function formatUptime(seconds) {
	if (seconds < 60) return `${seconds} s`;
	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return `${minutes} min`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours} h ${minutes % 60} min`;
	const days = Math.floor(hours / 24);
	return `${days} j ${hours % 24} h`;
}

export function formatSize(bytes) {
	if (!bytes) return '—';
	const units = ['o', 'Ko', 'Mo', 'Go', 'To'];
	let value = bytes;
	let unit = 0;
	while (value >= 1024 && unit < units.length - 1) {
		value /= 1024;
		unit++;
	}
	return `${value < 10 && unit > 0 ? formatDecimal(value) : Math.round(value)} ${units[unit]}`;
}

export function formatRuntime(minutes) {
	if (!minutes) return null;
	if (minutes < 60) return `${minutes} min`;
	const hours = Math.floor(minutes / 60);
	const remainingMinutes = minutes % 60;
	return remainingMinutes === 0
		? `${hours} h`
		: `${hours} h ${String(remainingMinutes).padStart(2, '0')}`;
}

const catalog = {
	metadata,
	strings,
	formatDecimal,
	formatUptime,
	formatSize,
	formatRuntime
};

export default catalog;
