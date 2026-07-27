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
		settings: 'Réglages',
		back: 'Retour'
	},

	hero: {
		details: 'Voir la fiche'
	},

	home: {
		loading: 'Chargement de la bibliothèque…',
		emptyTitle: 'La bibliothèque est vide',
		emptyBody:
			"Indiquez à Theia où sont vos films dans config.json, puis lancez une analyse depuis les réglages.",
		unreachable:
			"La page est chargée mais l'API ne répond pas. Le serveur a probablement été arrêté."
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
		milestone: 'Jalon M3 — interface de navigation. La lecture arrive au jalon suivant.'
	},

	errors: {
		scanFailed: "L'analyse n'a pas pu être menée à son terme.",
		scanBusy: 'Une analyse est déjà en cours.'
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
