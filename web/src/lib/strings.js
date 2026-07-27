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
	pitch:
		"Un seul binaire. Aucune configuration, aucun compte, aucun paywall. Vous branchez la machine, vous ouvrez un navigateur, un film se lance.",

	status: {
		checking: 'Connexion au serveur…',
		online: 'En ligne',
		offline: 'Serveur injoignable',
		version: 'Version',
		network: 'Réseau',
		films: 'Films'
	},

	library: {
		heading: 'Bibliothèque',
		emptyTitle: 'Aucun dossier configuré',
		emptyBody:
			"Theia n'a pas encore de dossier à surveiller. Ajoutez-en un dans config.json, puis relancez une analyse.",
		scannedTitle: 'Rien trouvé pour l’instant',
		scannedBody:
			"L'analyse s'est terminée sans trouver de fichier vidéo. Vérifiez que les dossiers configurés contiennent bien des films.",
		scan: 'Analyser les dossiers',
		scanning: 'Analyse en cours…',
		lastScan: 'Dernière analyse',
		found: 'trouvés',
		added: 'ajoutés',
		updated: 'mis à jour',
		removed: 'retirés',
		problems: 'Problèmes rencontrés',
		milestone: 'Jalon M1 — analyse des dossiers et lecture des noms de fichiers.'
	},

	errors: {
		unreachable:
			"La page est chargée mais l'API ne répond pas. Le serveur a probablement été arrêté.",
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
