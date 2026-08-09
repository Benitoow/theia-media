// English catalogue. Keep it strictly shape-compatible with fr.js: the site
// build walks both catalogues recursively and refuses a partial translation.
export default {
	lang: 'en',
	dir: 'en/',
	canonical: 'https://benitoow.github.io/theia-media/en/',
	ogLocale: 'en_US',
	other: { code: 'fr', label: 'FR', href: '../' },

	meta: {
		title: 'Theia — Your film. Not the platform.',
		description:
			'Theia turns folders on your device into a personal web media library, with no account, subscription or mandatory cloud.',
		ogAlt:
			'Theia personal media server: a real player capture and an explicit download choice by system and architecture.'
	},

	skip: 'Skip to content',
	nav: {
		label: 'Primary navigation',
		moments: 'Experience',
		difference: 'Difference',
		download: 'Download'
	},

	hero: {
		eyebrow: 'One media server. One binary.',
		title: 'Your film. Not the platform.',
		lead:
			'Choose your folders. Theia organises your library and plays your films on devices around the home — without an account, subscription or mandatory cloud.'
	},

	playerDemo: {
		status: 'Real capture · demonstration controls',
		title: 'Theia Demo',
		alt: 'Real Theia player capture showing an original black-and-gold abstract landscape.',
		caption:
			'Real Theia player capture using original media created for this page. The controls reproduce the player’s interactive chrome.',
		play: 'Play',
		pause: 'Pause',
		back10: 'Go back 10 seconds',
		forward10: 'Go forward 10 seconds',
		mute: 'Mute',
		unmute: 'Unmute',
		tracksOpen: 'Audio, subtitles and quality',
		tracksTitle: 'Audio, subtitles and quality',
		audio: 'Audio track',
		quality: 'Quality',
		subtitles: 'Subtitles',
		loading: 'Preparing playback…',
		position: 'Position in the film',
		volume: 'Volume',
		noJs:
			'Without JavaScript, the capture and all information remain visible; only the demonstration controls are static.',
		trackOptions: {
			audio: [
				['Automatic', ''],
				['English', '5.1 · DTS']
			],
			quality: [
				['1080p', 'Original file'],
				['720p', 'Re-encoded']
			],
			subtitles: [
				['None', ''],
				['English', 'External SRT']
			]
		}
	},

	downloads: {
		eyebrow: 'Try it now',
		title: 'Your system first. The architecture second.',
		lead: 'Theia never starts a guessed file. Choose both explicitly.',
		statusIdle: 'No system selected.',
		statusSelected: 'Selected system:',
		systems: { windows: 'Windows', macos: 'macOS', linux: 'Linux' },
		chooseArch: 'Choose an architecture',
		download: 'Download Theia for',
		releaseNotes: 'Release notes',
		shaLabel: 'SHA-256 checksums',
		noJs: 'JavaScript is disabled: all six files remain available below.',
		warnings: {
			windows:
				'SmartScreen may warn that the publisher is unknown: Theia is not signed. Check the digest, then choose “More info” and “Run anyway”.',
			macos:
				'macOS may block the app: Theia is not notarised. After checking the digest, use System Settings → Privacy & Security → Open Anyway.',
			linux: 'Make the binary executable with chmod +x, then launch it from your terminal.'
		}
	},

	moments: {
		eyebrow: 'Three moments, not an inventory',
		title: 'From your folders to the sofa. Then further.',
		lead: 'Theia makes sense through what it lets you do, not through a procession of screenshots.',
		items: [
			{
				label: 'DISCOVER',
				title: 'Your folders become a library.',
				body:
					'On first launch, you point Theia to your films and series. It scans, organises and presents what is already on this device.',
				alt: 'Real Theia library screenshot, with films presented as landscape cards.'
			},
			{
				label: 'WATCH',
				title: 'The player handles the difficult parts.',
				body:
					'Direct play when possible, adaptation when needed. Audio tracks, subtitles and quality stay understandable.',
				alt: ''
			},
			{
				label: 'ACCESS REMOTELY',
				title: 'An authorised device. Not an external account.',
				body:
					'Theia prepares WireGuard access so you can reach your library away from home without hosting your media on a Theia service.',
				alt: 'Real screenshot of WireGuard remote-access settings in Theia.'
			}
		],
		watchFacts: [
			['Audio', 'EN · DTS 5.1'],
			['Subtitles', 'English · SRT'],
			['Quality', 'Original · 1080p']
		]
	},

	difference: {
		eyebrow: 'Why Theia',
		title: 'Less ecosystem. More control.',
		items: [
			['One binary', 'The server, interface, database and player travel together. No Docker stack to assemble.'],
			['No Theia account', 'Your library stays on your machine. Local access does not depend on a remote service.'],
			['A browser is enough', 'TV, phone or computer: open Theia, choose who is watching, start the film.']
		],
		limitsTitle: 'The limits, before you download',
		limits: [
			'Theia is younger and less extensible than Plex or Jellyfin.',
			'Transcoding depends on your machine’s codecs and processing power.',
			'Image-based subtitles such as PGS or VobSub are identified but not rendered.'
		],
		compare: 'Read the detailed comparison on GitHub',
		safetyTitle: 'Safe by default, provided you do not improvise the network.',
		safetyBody:
			'Theia is designed for your local network. Do not publish its administration port on the Internet: use the built-in WireGuard remote access. The code is GPL-3.0 and every published binary has a SHA-256 digest.'
	},

	closing: {
		eyebrow: 'Ready to try',
		title: 'Your library is already there.',
		body: 'All that remains is choosing the binary that actually matches your machine.',
		cta: 'Choose my system'
	},

	faq: {
		eyebrow: 'Three useful answers',
		title: 'Before you run the binary',
		items: [
			[
				'Do my files leave my machine?',
				'Not during normal operation: Theia reads your folders and serves the interface from this device. Film and series metadata may be looked up through TMDB.'
			],
			[
				'Why does Windows or macOS show a warning?',
				'Published binaries are not yet signed for SmartScreen or notarised by Apple. The warning is expected; verify the SHA-256 digest before continuing.'
			],
			[
				'Is it a complete replacement for Plex or Jellyfin?',
				'Not for everyone. Theia aims for a more direct setup and clear domestic use. It accepts a smaller ecosystem and fewer extensions.'
			]
		]
	},

	footer: {
		body: 'Theia is free software under GPL-3.0. Playback and transcoding: FFmpeg, under its own licence.',
		tmdb: 'This product uses the TMDB API but is not endorsed or certified by TMDB.',
		navLabel: 'Legal and project links',
		source: 'Source code',
		license: 'GPL-3.0 licence'
	}
};
