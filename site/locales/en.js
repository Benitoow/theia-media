// English catalogue. Kept key-for-key with fr.js; site/build.mjs fails the
// build if the two ever drift apart.
export default {
	lang: 'en',
	dir: 'en/',
	other: { code: 'fr', label: 'Français', href: '../' },

	title: 'Theia — your media library, one binary',
	description:
		'A personal media server in a single Go binary. Films and series, TMDB metadata, transcoding, household profiles and WireGuard remote access. No configuration, no account, no subscription.',

	nav: { features: 'What it is', shots: 'On screen', download: 'Download', github: 'GitHub' },

	hero: {
		kicker: 'Personal media server',
		title: 'Your films.\nOne binary.\nYour network.',
		lede: 'Theia turns folders of movie and series files into a library you open from the television, phone and computer already in your home. No account, no subscription, no Docker, no separate interface to install. Run the binary, choose the folders, watch.',
		cta: 'Download',
		ctaOther: 'All platforms',
		source: 'Read the code',
		meta: 'GPL-3.0 · Windows, macOS, Linux · 17 MB'
	},

	claims: {
		title: 'Three things, and they hold',
		items: [
			{
				k: 'One file',
				t: 'Nothing to install around it',
				d: 'The server, the interface, the database and the player are the same executable. The only external dependency is FFmpeg, which Theia downloads itself, pinned and verified against a SHA-256 digest.'
			},
			{
				k: 'No account',
				t: 'Nothing to create, nothing to connect',
				d: 'No sign-up, no cloud library, no paid tier that unlocks the hardware in your own machine. The only outbound connections go to TMDB for metadata and to GitHub for updates.'
			},
			{
				k: 'Your machine',
				t: 'The files do not move',
				d: 'Your films stay where they are, on your disks. Theia reads them, indexes them and streams them on your network. Nothing is uploaded, nothing is analysed elsewhere, nothing is measured.'
			}
		]
	},

	shots: {
		title: 'On screen',
		lede: 'The interface is shown in French, the default; English is complete and switches without a reload. Posters, synopses and stills come from TMDB.',
		home: 'The home screen offers to resume the film you left, then what else is part-watched, recently added, best rated, and one suggestion that changes daily.',
		library: 'The library: search across title, director, genre and year, five sorts, filters by genre and watch state.',
		series: 'Series live beside films, in the same grid.',
		serie: 'A series, season by season, with resume kept per episode.',
		settings: 'Remote access: an embedded WireGuard tunnel, off until you turn it on.',
		onboarding: 'On first launch, an address and a QR code. There is nothing else to configure.',
		profiles: 'Everyone keeps their own place. A name, a photo, no password.'
	},

	does: {
		title: 'What Theia does',
		groups: [
			{
				t: 'Library',
				items: [
					'Scans one or more folders and keeps a local SQLite catalogue.',
					'Reads a title and a year out of ordinary release filenames. A file still appears when parsing fails.',
					'Groups several files under one film: a remux and a 1080p encode are one card, not two.',
					'Handles series: seasons, episodes, per-episode resume.',
					'Fetches titles, synopses, posters, runtimes, ratings, director, genres and cast from TMDB, and caches the images.'
				]
			},
			{
				t: 'Watching',
				items: [
					'A home screen built around what you were watching.',
					'A player that lets you choose the audio track, the subtitles and the quality while the film runs.',
					'Subtitles from the file itself or from the .srt sitting beside it, drawn by Theia so they land on the picture rather than in the letterbox.',
					'Position saved continuously, and seeking that works even on a remuxed stream.'
				]
			},
			{
				t: 'Household',
				items: [
					'Profiles: a name, an optional local photo, a separate resume history. No password, no role.',
					'Embedded WireGuard remote access, with one-time device provisioning and per-device revocation. No relay, no rendezvous server.',
					'Two languages, French by default, the choice belonging to each browser.'
				]
			},
			{
				t: 'Operations',
				items: [
					'An address and a QR code on first launch.',
					'Digest-verified updates, refused while something is playing, and reversible if the new binary fails to start.'
				]
			}
		]
	},

	refuses: {
		title: 'What Theia refuses',
		lede: 'Half the argument. These absences are decisions, written down and dated, not a roadmap.',
		items: [
			['Accounts and permissions', 'A household is not an organisation. The local port has no authentication, and that is deliberate.'],
			['Live TV and recording', 'A different product, with its tuners and its schedules.'],
			['A plugin system', 'A program’s extension surface is also its failure surface.'],
			['Music and photos', 'Two more libraries, twice the work, nothing to do with a film.'],
			['Native applications', 'The browser on your television already exists.'],
			['Any usage measurement', 'Nothing is counted, nothing is sent.']
		]
	},

	numbers: {
		title: 'Measured, not estimated',
		lede: 'Taken on the maintainer’s machine, against a real library. No figure here is an estimate.',
		items: [
			['17 MB', 'the binary, interface included'],
			['18 MB', 'resident memory at idle'],
			['22 MB', 'during a sustained remux'],
			['0.8 MB', 'the SQLite database'],
			['0', 'containers, services, system dependencies'],
			['1', 'external dependency: FFmpeg, downloaded and verified']
		]
	},

	download: {
		title: 'Download',
		lede: 'Pick your platform. The link always points at the latest published release.',
		detected: 'Detected on this device',
		all: 'All platforms',
		digest:
			'GitHub publishes a SHA-256 digest for every file on the release page. It is the same digest Theia’s own updater checks before it will install anything.',
		steps: {
			title: 'After downloading',
			windows: [
				'Run <code>theia.exe</code>. Windows SmartScreen will say it does not recognise the publisher: <em>More info</em>, then <em>Run anyway</em>.',
				'An address and a QR code appear. Open them from any device on the network.'
			],
			macos: [
				'<code>chmod +x theia-darwin-arm64</code>, then run it from the terminal.',
				'macOS blocks an unnotarised binary: <em>System Settings → Privacy &amp; Security → Open Anyway</em>.'
			],
			linux: [
				'<code>chmod +x theia-linux-amd64</code>, then <code>./theia-linux-amd64</code>.',
				'No package, no service to declare. A <code>systemd</code> unit if you want one, never because you must.'
			]
		},
		warning: {
			title: 'Before you expose it',
			body:
				'Theia has no authentication on its local port, and that is a decision. Never forward port 8383 on your router. To reach it from outside, use the built-in remote access: a separate WireGuard tunnel, one key per device, and viewer-only rights.'
		}
	},

	compare: {
		title: 'Against the others',
		lede: 'The rows that go against Theia stay. They are the reason it is small.',
		head: ['', 'Theia', 'Plex', 'Jellyfin', 'Emby'],
		rows: [
			['Single binary, no dependencies', 'Yes', 'No', 'No', 'No'],
			['No account for local use', 'Yes', 'Account required', 'Yes', 'Yes'],
			['No paid features', 'Yes', 'Plex Pass', 'Yes', 'Emby Premiere'],
			['No usage measurement', 'Yes', 'No', 'Yes', 'Partial'],
			['Films and series', 'Yes', 'Yes', 'Yes', 'Yes'],
			['Remote access without a third party', 'Yes', 'Via Plex', 'Manual', 'Via Emby'],
			['Live TV and recording', 'No', 'Yes', 'Yes', 'Yes'],
			['Music, photos, books', 'No', 'Yes', 'Yes', 'Yes'],
			['Native TV and mobile apps', 'No', 'Yes', 'Yes', 'Yes'],
			['Plugins and extensions', 'No', 'Yes', 'Yes', 'Yes'],
			['Multiple users with permissions', 'No', 'Yes', 'Yes', 'Yes']
		],
		note: 'Compiled in August 2026 from each project’s public documentation, and measured on Theia itself.'
	},

	footer: {
		licence: 'Theia is free software under the GNU General Public License v3.0.',
		tmdb: 'This product uses the TMDB API but is not endorsed or certified by TMDB.',
		credits:
			'FFmpeg is downloaded on demand and remains under its own licence. Inter and Playfair Display are served locally under the SIL Open Font License 1.1.',
		links: { repo: 'Source code', releases: 'Releases', issues: 'Report a problem', security: 'Security' }
	}
};
