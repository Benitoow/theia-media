// The OSD's own sentences.
//
// The Rust side never sends one: it sends a code and a reason, and this file is
// where a code becomes something a person reads. That is decision 25 applied to
// the player -- the same rule that keeps Windows syscall names out of a French
// settings page.
//
// French is the default, English ships complete, and the two catalogues are
// expected to grow together. A parity check belongs here the moment this file
// carries more than a screenful; the web application's own check is
// web/scripts/check-locales.mjs.

export const catalogues = {
	fr: {
		play: 'Lecture',
		pause: 'Pause',
		back10: 'Reculer de 10 secondes',
		forward10: 'Avancer de 10 secondes',
		mute: 'Couper le son',
		unmute: 'Rétablir le son',
		fullscreen: 'Plein écran',
		exitFullscreen: 'Quitter le plein écran',
		close: 'Fermer le lecteur',
		tracks: 'Audio et sous-titres',
		loading: 'Chargement',
		engineUnavailable:
			"Le moteur de lecture n'a pas pu démarrer. Le lecteur ne peut pas ouvrir de film.",
		audioFallbackLabel: 'Son',
		audioFallback:
			"Le convertisseur HDMI a refusé le flux audio brut : le film est lu en PCM décodé. Rien n'est cassé, mais le son n'arrive pas tel quel à l'amplificateur.",
		unknownDuration: '--:--',

		chooseFilm: 'Choisir un film',
		library: 'Bibliothèque',
		address: 'Adresse du serveur',
		connect: 'Se connecter',
		findServers: 'Chercher un serveur',
		searching: 'Recherche…',
		noServer:
			"Aucun serveur détecté sur le réseau. Saisis l'adresse que Theia affiche à son démarrage.",
		connectionFailed: "La connexion a échoué. Vérifie l'adresse, et que Theia est bien démarré.",
		libraryFailed: "La bibliothèque n'a pas pu être chargée.",
		playFailed: "Ce film n'a pas pu être ouvert.",
		emptyLibrary: 'Cette bibliothèque ne contient encore aucun film.',
		resumeAt: 'Reprendre à',
		audioTracks: 'Piste audio',
		subtitleTracks: 'Sous-titres',
		subtitlesOff: 'Aucun',
		externalTrack: 'fichier externe',
		trackNumber: 'Piste',
		trackFailed: "Cette piste n'a pas pu être choisie.",
	},
	en: {
		play: 'Play',
		pause: 'Pause',
		back10: 'Back 10 seconds',
		forward10: 'Forward 10 seconds',
		mute: 'Mute',
		unmute: 'Unmute',
		fullscreen: 'Full screen',
		exitFullscreen: 'Leave full screen',
		close: 'Close the player',
		tracks: 'Audio and subtitles',
		loading: 'Loading',
		engineUnavailable: 'The playback engine could not start, so no film can be opened.',
		audioFallbackLabel: 'Sound',
		audioFallback:
			'The HDMI endpoint refused the raw audio stream, so the film is playing as decoded PCM. Nothing is broken, but the sound is not reaching the amplifier untouched.',
		unknownDuration: '--:--',

		chooseFilm: 'Choose a film',
		library: 'Library',
		address: 'Server address',
		connect: 'Connect',
		findServers: 'Find a server',
		searching: 'Searching…',
		noServer: 'No server answered on the network. Enter the address Theia prints when it starts.',
		connectionFailed: 'The connection failed. Check the address, and that Theia is running.',
		libraryFailed: 'The library could not be loaded.',
		playFailed: 'This film could not be opened.',
		emptyLibrary: 'This library holds no films yet.',
		resumeAt: 'Resume at',
		audioTracks: 'Audio track',
		subtitleTracks: 'Subtitles',
		subtitlesOff: 'None',
		externalTrack: 'external file',
		trackNumber: 'Track',
		trackFailed: 'That track could not be selected.',
	},
};

/** The interface language, from the stored choice or the system, French first. */
export function initialLanguage() {
	try {
		const stored = localStorage.getItem('theia.player.language');
		if (stored && catalogues[stored]) return stored;
	} catch {
		// A WebView with storage disabled still gets a working player.
	}
	const system = (navigator.language || 'fr').toLowerCase();
	return system.startsWith('en') ? 'en' : 'fr';
}
