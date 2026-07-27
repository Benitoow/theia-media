// Every string the user reads lives here.
//
// The interface is French for v1 because there is exactly one user and he is
// French. It is kept in a separate module anyway so that switching to another
// language later is a matter of adding a file, not of hunting through markup.
// Nothing else in the codebase is French: code, comments and internal error
// messages are English.

export const strings = {
	appName: 'Theia',
	tagline: 'Votre serveur média personnel',

	status: {
		checking: 'Connexion au serveur…',
		online: 'Serveur en ligne',
		offline: 'Serveur injoignable',
		version: 'Version',
		uptime: 'En ligne depuis'
	},

	library: {
		emptyTitle: 'Aucune bibliothèque configurée',
		emptyBody:
			"Theia n'a pas encore de dossier à surveiller. L'analyse des dossiers et l'affichage des films arrivent au jalon suivant.",
		emptyHint: 'Jalon M0 — le serveur démarre, se fait connaître sur le réseau et sert cette page.'
	},

	errors: {
		unreachable:
			"La page est chargée mais l'API ne répond pas. Le serveur a probablement été arrêté."
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
