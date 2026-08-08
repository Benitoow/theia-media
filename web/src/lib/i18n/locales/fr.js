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

	// Les conteneurs écrivent la langue en ISO 639, et pas toujours la même
	// variante : « fre » et « fra » désignent le français, « fr » aussi. Le
	// serveur transmet le code tel qu'il l'a lu (décision 25) ; c'est ici qu'il
	// devient un mot. Un code absent s'affiche en majuscules, ce qui reste
	// lisible et n'invente rien.
	languages: {
		fra: 'Français', fre: 'Français', fr: 'Français',
		eng: 'Anglais', en: 'Anglais',
		spa: 'Espagnol', es: 'Espagnol',
		deu: 'Allemand', ger: 'Allemand', de: 'Allemand',
		ita: 'Italien', it: 'Italien',
		por: 'Portugais', pt: 'Portugais',
		nld: 'Néerlandais', dut: 'Néerlandais', nl: 'Néerlandais',
		jpn: 'Japonais', ja: 'Japonais',
		zho: 'Chinois', chi: 'Chinois', zh: 'Chinois',
		kor: 'Coréen', ko: 'Coréen',
		rus: 'Russe', ru: 'Russe',
		ara: 'Arabe', ar: 'Arabe',
		pol: 'Polonais', pl: 'Polonais',
		swe: 'Suédois', sv: 'Suédois',
		dan: 'Danois', da: 'Danois',
		nor: 'Norvégien', no: 'Norvégien',
		fin: 'Finnois', fi: 'Finnois',
		tur: 'Turc', tr: 'Turc',
		ces: 'Tchèque', cze: 'Tchèque', cs: 'Tchèque',
		ell: 'Grec', gre: 'Grec', el: 'Grec',
		heb: 'Hébreu', he: 'Hébreu',
		hin: 'Hindi', hi: 'Hindi',
		und: 'Langue non précisée', mul: 'Multilingue'
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
		series_continue: {
			title: 'Reprendre une série',
			href: '/series'
		},
		series_recent: {
			title: 'Séries récemment ajoutées',
			href: '/series'
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
			browser_cannot_decode_video:
				'Votre navigateur ne sait pas décoder la vidéo de ce fichier : le son avancerait sur une image figée. Ouvrez-le dans un autre navigateur, ou choisissez un autre fichier sur la fiche du film.',
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

		tracks: {
			open: 'Audio et sous-titres',
			title: 'Audio et sous-titres',
			close: 'Fermer',
			subtitles: 'Sous-titres',
			noSubtitles: 'Aucun',
			external: 'fichier joint',
			forced: 'forcés',
			unnamedSubtitle: (index) => `Sous-titres ${index}`,
			// Décision 3 : une piste image ne peut être affichée qu'en la
			// gravant dans l'image, ce qui est le transcodage complet que la v1
			// refuse. Elle est listée quand même, sinon on la cherche.
			imageBased: 'image — non affichable'
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
				'Theia ne choisit jamais une qualité à votre place : plusieurs fichiers existent ' +
				'et vous décidez lequel lire.',
			tracksInPlayer: 'Les pistes audio et les sous-titres se choisissent pendant la lecture.'
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

	remote: {
		heading: 'Accès distant',
		intro:
			'Theia peut être joint hors du réseau local par un tunnel WireGuard embarqué. Aucun compte, aucun relais, aucun service extérieur : chaque appareil prouve une clé créée ici.',
		opening: 'Ouverture du port…',
		publicAddress: 'Point d’entrée du tunnel',
		tunnelAddress: 'Adresse à ouvrir une fois connecté',
		howItWorks:
			'Le point d’entrée n’est pas une adresse web : rien n’y répond dans un navigateur, ' +
			'c’est normal. Il sert à WireGuard. Installez WireGuard sur l’appareil, importez la ' +
			'configuration créée ci-dessous, activez le tunnel, puis ouvrez l’adresse ci-dessus.',
		step1: 'Installez WireGuard sur l’appareil (App Store, Google Play, wireguard.com).',
		step2: 'Importez cette configuration : scannez le code, ou ouvrez le fichier .conf téléchargé.',
		step3: 'Activez le tunnel, puis ouvrez cette adresse dans le navigateur :',
		opened: 'Port ouvert par',
		methods: {
			upnp: 'UPnP',
			natpmp: 'NAT-PMP'
		},
		addressChanged:
			'Votre adresse publique a changé depuis l’activation. Les appareils créés avant ce ' +
			'changement pointent vers une adresse qui n’est plus la vôtre : recréez-les.',
		manual: 'Réglage manuel du port et de l’adresse',
		retryAutomatic: 'Redemander au routeur',
		// Le serveur envoie un code ; ces phrases-là appartiennent à l'interface.
		discovery: {
			remote_router_silent:
				'Votre routeur n’a pas répondu. L’ouverture automatique de ports (UPnP ou NAT-PMP) ' +
				'est sans doute désactivée dans son interface — c’est le cas par défaut sur ' +
				'certaines box. Activez-la, ou ouvrez le port à la main ci-dessous.',
			remote_router_refused:
				'Votre routeur a refusé d’ouvrir ce port. Il est peut-être déjà redirigé vers une ' +
				'autre machine, ou la redirection est verrouillée. Essayez un autre port, ou ' +
				'ouvrez-le à la main ci-dessous.',
			remote_carrier_nat:
				'Votre opérateur vous place derrière un CGNAT : votre connexion n’a pas d’adresse ' +
				'publique à elle, et aucune redirection de port ne peut la traverser. Demandez-lui ' +
				'une adresse IP publique — c’est en général gratuit et immédiat.'
		},
		router:
			'Votre box doit rediriger le port UDP ci-dessous vers cette machine. Ne redirigez jamais le port TCP de Theia : il n’a aucune authentification.',
		cgnat:
			'Si votre opérateur vous place derrière un CGNAT, aucune redirection n’est possible et Theia ne peut rien y faire.',
		state: 'État',
		stateDisabled: 'Désactivé',
		stateRunning: 'Actif',
		stateError: 'En erreur',
		enable: 'Activer l’accès distant',
		disable: 'Désactiver',
		port: 'Port UDP d’écoute',
		portHelp: 'Le port que votre box redirige vers cette machine.',
		portChange: 'Changer le port redémarre le tunnel et coupe brièvement les appareils connectés.',
		endpoint: 'Adresse publique',
		endpointPlaceholder: 'media.exemple.net:51820',
		endpointHelp:
			'Sous la forme hôte:port, sans http://. Le port public peut différer du port d’écoute.',
		endpointExamples:
			'Exemples : media.exemple.net:51820, 203.0.113.10:51820, [2001:db8::10]:51820',
		endpointChange:
			'Changer cette adresse ne met pas à jour les appareils déjà créés : il faut modifier leur configuration WireGuard, ou les révoquer et les recréer.',
		reachability: 'Joignabilité',
		unverified: 'Jamais confirmée',
		unverifiedHelp:
			'Aucun appareil n’a encore prouvé le chemin. Ce n’est pas une erreur : Theia ne teste pas votre box depuis l’extérieur.',
		confirmed: 'Confirmée',
		confirmedHelp:
			'Au moins un appareil a établi une liaison depuis ce démarrage. Ce n’est pas une garantie permanente.',
		save: 'Enregistrer',
		saving: 'Enregistrement…',
		devices: 'Appareils',
		noDevices: 'Aucun appareil n’a encore été créé.',
		addDevice: 'Ajouter un appareil',
		deviceName: 'Nom de l’appareil',
		devicePlaceholder: 'Télévision du salon',
		create: 'Créer',
		creating: 'Création…',
		enableFirst: 'Activez l’accès distant avant de créer un appareil.',
		address: 'Adresse dans le tunnel',
		lastHandshake: 'Dernière liaison',
		neverConnected: 'Jamais connecté',
		traffic: 'Trafic',
		trafficHelp: 'Compteurs réseau du tunnel, pas des statistiques de visionnage.',
		revoke: 'Révoquer',
		revokeConfirm: (name) =>
			'Révoquer ' + name + ' ? Cet appareil perdra l’accès immédiatement et sa clé ne pourra pas être réactivée.',
		revokeYes: 'Révoquer cet appareil',
		cancel: 'Annuler',

		provisionTitle: (name) => name + ' est prêt',
		provisionWarning:
			'Ce QR code et ce fichier contiennent une clé privée. Ils ne seront plus jamais affichés.',
		provisionScan: 'Scannez ce code avec l’application WireGuard de l’appareil.',
		copyConfig: 'Copier la configuration',
		copied: 'Copié',
		downloadConfig: 'Télécharger le .conf',
		done: 'J’ai conservé la configuration',
		closeWarning:
			'Fermer sans conserver la configuration ? Elle ne peut pas être réaffichée : il faudra révoquer cet appareil et en créer un autre.',
		closeAnyway: 'Fermer quand même',
		lost: 'Configuration perdue ? Révoquez cet appareil et créez-en un nouveau.',

		remoteBadge: 'Accès distant',
		remoteContext: (name) => 'Connecté depuis ' + name,
		remoteRestricted:
			'Les réglages, l’analyse et les mises à jour ne sont accessibles que depuis le réseau local.',

		codes: {
			invalid_remote_access_payload: 'La demande n’a pas pu être lue.',
			invalid_remote_listen_port: 'Ce port n’est pas valide. Choisissez un port entre 1 et 65535.',
			invalid_remote_endpoint:
				'Cette adresse n’est pas valide. Attendu : hôte:port, sans http:// ni chemin.',
			invalid_remote_peer_payload: 'La demande n’a pas pu être lue.',
			invalid_remote_peer_name: 'Un nom est requis, de 1 à 64 caractères.',
			invalid_remote_peer_id: 'Cet appareil n’est pas identifiable.',
			remote_peer_limit_reached: 'La limite de 32 appareils actifs est atteinte.',
			remote_peer_not_found: 'Cet appareil n’existe plus.',
			remote_access_disabled: 'Activez l’accès distant avant cette opération.',
			remote_access_not_ready: 'Le tunnel n’est pas prêt. Réessayez dans un instant.',
			remote_access_unavailable: 'L’accès distant est indisponible.'
		},

		reasons: {
			remote_config_invalid: 'La configuration enregistrée n’est pas utilisable.',
			remote_key_unavailable:
				'La clé du serveur est illisible. Si ce dossier de données vient d’une autre machine ou d’un autre compte Windows, désactivez l’accès distant, supprimez remote-access.key, réactivez et recréez les appareils.',
			remote_listen_failed: 'Le port UDP n’a pas pu être ouvert. Il est peut-être déjà utilisé.',
			remote_listener_stopped: 'Le tunnel s’est arrêté de lui-même.',
			remote_peer_reload_failed:
				'Les appareils n’ont pas pu être rechargés ; le tunnel a été fermé par sécurité.',
			remote_restore_failed: 'L’ancienne configuration n’a pas pu être rétablie.'
		}
	},

	series: {
		title: 'Séries',
		loading: 'Chargement des séries…',
		countAll: (n) => (n === 1 ? '1 série' : `${n} séries`),
		emptyTitle: 'Aucune série',
		emptyBody:
			'Rangez vos épisodes en dossiers de série et de saison, nommés SxxExx, puis relancez une analyse depuis les réglages.',
		notFound: 'Cette série est introuvable.',
		episodeNotFound: 'Cet épisode est introuvable.',
		seasons: 'Saisons',
		specials: 'Épisodes spéciaux',
		specialsHelp:
			"La saison 0 rassemble ce qui est hors numérotation : making-of, épisodes de " +
			"Noël, récapitulatifs. Elle n'apparaît que si un dossier « Specials » ou " +
			'« Saison 00 » existe dans votre bibliothèque, et elle ne fait jamais partie ' +
			"de l'enchaînement automatique.",
		season: (number) => `Saison ${number}`,
		ownedEpisodes: (n) => (n === 1 ? '1 épisode possédé' : `${n} épisodes possédés`),
		episodeLabel: (number) => `Épisode ${number}`,
		episodeRange: (first, last) => `Épisodes ${first} à ${last}`,
		combined: 'Épisode combiné',
		combinedHint:
			'Ces épisodes sont dans un seul fichier : ils se lisent d’une traite et partagent une reprise.',
		gap: 'Un épisode manque avant le suivant',
		next: 'Épisode suivant',
		lastOwned: 'Dernier épisode possédé',
		unmatched:
			'Cette série n’a pas été identifiée sur TMDB. Elle reste listée sous le nom de son dossier.',
		noOverview: 'Aucun synopsis n’est disponible.',
		play: 'Lire l’épisode',
		resumeAtMinutes: (minutes) => `Reprendre à ${minutes} min`
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
