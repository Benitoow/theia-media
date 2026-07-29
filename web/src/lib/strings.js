// Every string the user reads lives here.
//
// The interface is French for v1 because there is exactly one user and he is
// French. It is kept in a separate module anyway so that switching to another
// language later is a matter of adding a file, not of hunting through markup.
// Nothing else in the codebase is French: code, comments and internal error
// messages are English.

export const strings = {
	appName: 'Theia',
	tagline: 'Serveur média personnel',

	nav: {
		home: 'Accueil',
		library: 'Films',
		settings: 'Réglages',
		profiles: 'Profils',
		back: 'Retour'
	},

	profiles: {
		eyebrow: 'Dans ce foyer',
		title: 'Qui regarde ?',
		body:
			'Les profils séparent la reprise de lecture. Ce ne sont pas des comptes : aucun mot de passe, et toute personne sur le réseau local peut choisir ou modifier n’importe quel profil.',
		loading: 'Chargement des profils…',
		listLabel: 'Choisir un profil',
		defaultName: 'Profil principal',
		current: 'Profil actif',
		switchTo: (name) => `Changer de profil — ${name}`,
		manage: 'Gérer les profils',
		manageHint:
			'La photo reste dans la base locale de Theia. Elle est recadrée et allégée avant d’être enregistrée.',
		done: 'Terminer',
		name: 'Nom',
		newName: 'Nouveau profil',
		namePlaceholder: 'Prénom ou surnom',
		add: 'Ajouter',
		adding: 'Ajout…',
		rename: 'Enregistrer le nom',
		addPhoto: 'Ajouter une photo',
		replacePhoto: 'Remplacer la photo',
		removePhoto: 'Retirer la photo',
		delete: 'Supprimer',
		confirmDelete: 'Confirmer la suppression',
		notices: {
			profile_saved: 'Nom enregistré.',
			avatar_saved: 'Photo enregistrée.',
			avatar_removed: 'Photo retirée.',
			profile_deleted: 'Profil supprimé. Les films n’ont pas été touchés.',
			invalid_profile_name: 'Le nom doit contenir entre 1 et 40 caractères.',
			invalid_profile_payload: "La modification envoyée n'est pas valide.",
			profile_not_found: "Ce profil n'existe plus. Rechargez la liste.",
			profile_limit_reached: 'Le nombre maximal de profils est atteint.',
			default_profile: 'Le profil principal ne peut pas être supprimé.',
			last_profile: 'Le dernier profil ne peut pas être supprimé.',
			avatar_too_large: 'Cette image est trop lourde. Choisissez un fichier de moins de 8 Mo.',
			invalid_avatar: 'Utilisez une image JPEG, PNG ou WebP valide.',
			profile_create_failed: "Le profil n'a pas pu être ajouté.",
			profile_update_failed: "Le profil n'a pas pu être modifié.",
			profile_delete_failed: "Le profil n'a pas pu être supprimé.",
			avatar_save_failed: "La photo n'a pas pu être enregistrée.",
			avatar_delete_failed: "La photo n'a pas pu être retirée.",
			unknown: "La modification n'a pas pu être enregistrée."
		}
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
		ratingLegend: (rating) =>
			`${rating.toLocaleString('fr-FR', { minimumFractionDigits: 1, maximumFractionDigits: 1 })} / 10`,
		unwatchedBadge: 'Jamais lu',
		finishedBadge: 'Vu'
	},

	hero: {
		details: 'Voir la fiche',
		resumeEyebrow: 'Vous étiez en train de regarder',
		resume: 'Reprendre',
		remaining: (duration) => `Il reste ${duration}`
	},

	// The server sends a row kind, never a title (decision 25). These are the
	// sentences, and the link each row offers into the library.
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
			// No link: the point of this row is that somebody else chose. A way
			// through to a sorted list would be answering a different question.
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

	// Scan problems arrive as codes, never as text: the server used to send its
	// own error strings and the settings page ended up showing people a syscall
	// name in the middle of a French interface.
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

	// Most of these are accessible names on icon buttons rather than visible
	// text. They still belong here: a screen reader reads them aloud, and a
	// button whose only label is a shape is a button nobody can describe.
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
		file: 'Fichier',
		size: 'Taille',
		progress: 'Progression dans le film',
		moreByDirector: (director) => `Voir les autres films de ${director}`,
		noOverview: "Aucun synopsis n'est disponible pour ce film.",
		unmatched:
			"Ce fichier n'a pas été identifié sur TMDB. Il reste listé sous le nom que porte le fichier ; " +
			'renommer celui-ci relance une recherche à la prochaine analyse.'
	},

	settings: {
		heading: 'Réglages',
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

	// The frontend is a single-page app: the Go server answers every unknown
	// path with index.html and the client router decides. So a mistyped address
	// lands here rather than on a server 404, and without this screen it landed
	// on SvelteKit's own -- untouched, unstyled and in English.
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

// formatUptime turns a duration in seconds into something readable in French.
export function formatUptime(seconds) {
	if (seconds < 60) return `${seconds} s`;
	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return `${minutes} min`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours} h ${minutes % 60} min`;
	const days = Math.floor(hours / 24);
	return `${days} j ${hours % 24} h`;
}

// formatSize renders a file size the way a person would say it out loud.
export function formatSize(bytes) {
	if (!bytes) return '—';
	const units = ['o', 'Ko', 'Mo', 'Go', 'To'];
	let value = bytes;
	let unit = 0;
	while (value >= 1024 && unit < units.length - 1) {
		value /= 1024;
		unit++;
	}
	return `${value < 10 && unit > 0 ? value.toFixed(1) : Math.round(value)} ${units[unit]}`;
}
