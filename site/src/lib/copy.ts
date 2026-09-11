// The page copy, English only, typed. English became the single language when
// the site was rebuilt on React/Tailwind; there is no other catalogue to stay
// shape-compatible with, and check.mjs guards the structure of the output.

export const REPO = 'https://github.com/Benitoow/theia-media';
export const SITE = 'https://benitoow.github.io/theia-media';
export const DISCORD = 'https://discord.gg/p4Rp4zHdHf';
export const LATEST = `${REPO}/releases/latest/download`;

export interface PlatformAsset {
	arch: string;
	file: string;
}

export interface Platform {
	id: 'windows' | 'macos' | 'linux';
	label: string;
	assets: PlatformAsset[];
}

export const PLATFORMS: Platform[] = [
	{ id: 'windows', label: 'Windows', assets: [{ arch: 'x64', file: 'theia-windows-amd64.exe' }, { arch: 'ARM64', file: 'theia-windows-arm64.exe' }] },
	{ id: 'macos', label: 'macOS', assets: [{ arch: 'Apple Silicon', file: 'theia-darwin-arm64' }, { arch: 'Intel', file: 'theia-darwin-amd64' }] },
	{ id: 'linux', label: 'Linux', assets: [{ arch: 'x64', file: 'theia-linux-amd64' }, { arch: 'ARM64', file: 'theia-linux-arm64' }] }
];

const copy = {
	lang: 'en' as const,
	canonical: SITE,
	ogLocale: 'en_US',

	meta: {
		title: 'Theia — Your film. Not the platform.',
		description:
			'Theia turns the folders on your machine into a media library the whole household opens in a browser — one binary, no account, no subscription, no cloud.',
		ogAlt:
			'Theia personal media server: a real player capture and an explicit download choice by system and architecture.'
	},

	skip: 'Skip to content',
	nav: {
		label: 'Primary navigation',
		how: 'How it works',
		moments: 'Experience',
		difference: 'Difference',
		download: 'Download'
	},

	hero: {
		eyebrow: 'Theia V3 · Personal media server',
		titleLead: 'Your film.',
		titleRest: 'Not the platform.',
		lead:
			'One binary turns your folders into a library the whole household opens in a browser — no account, no cloud.',
		primary: 'Download Theia',
		secondary: 'See it run',
		facts: ['One binary', 'No account', 'No cloud', 'GPL-3.0']
	},

	steps: {
		eyebrow: 'How it works',
		title: 'Three steps. No installer.',
		lead: 'A single file that runs from wherever you put it.',
		items: [
			{
				number: '01',
				title: 'Download the binary',
				body: 'One file for your system and architecture. Verify it, drop it anywhere.',
				hint: 'theia-windows-amd64.exe',
				prompt: true
			},
			{
				number: '02',
				title: 'Run it',
				body: 'Keep a terminal open. Theia prints its address the moment it starts.',
				hint: './theia',
				prompt: true
			},
			{
				number: '03',
				title: 'Open the address',
				body: 'Point Theia at your films and series, pick a profile, press play.',
				hint: 'http://localhost:8383',
				prompt: false
			}
		]
	},

	playerDemo: {
		status: 'Real capture · demonstration controls',
		title: 'Theia Demo',
		alt: 'Real Theia player capture showing an original black-and-gold abstract landscape.',
		caption:
			'Real capture · original media authored for this page · the controls are a demonstration.',
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
		noJs: 'Without JavaScript, the capture and all information remain visible; only the demonstration controls are static.',
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
		} as Record<'audio' | 'quality' | 'subtitles', [string, string][]>
	},

	downloads: {
		eyebrow: 'Try it now',
		title: 'Your system first. The architecture second.',
		lead: 'Theia never starts a guessed file. Choose both explicitly.',
		statusIdle: 'No system selected.',
		statusSelected: 'Selected system:',
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
		} as Record<Platform['id'], string>
	},

	moments: {
		eyebrow: 'Three moments, not an inventory',
		title: 'From your folders to the sofa. Then further.',
		items: [
			{
				label: 'DISCOVER',
				title: 'Your folders become a library.',
				body: 'You point Theia to your films and series. It scans, organises and presents what is already on this device.',
				alt: 'Real Theia library screenshot, with films presented as landscape cards.',
				shot: 'library.webp'
			},
			{
				label: 'WATCH',
				title: 'The player handles the difficult parts.',
				body:
					'Direct play when possible, adaptation when needed. Audio tracks, subtitles and quality stay understandable.',
				alt: 'Real Theia film page: play, resume, tracks and file details for one film.',
				shot: 'film.webp',
				facts: true
			},
			{
				label: 'ACCESS REMOTELY',
				title: 'An authorised device. Not an external account.',
				body: 'WireGuard access is prepared for you: your library reaches you away from home, and never touches a Theia service.',
				alt: 'Real screenshot of WireGuard remote-access settings in Theia.',
				shot: 'settings.webp'
			}
		] as { label: string; title: string; body: string; alt: string; shot: string | null; facts?: boolean }[],
		watchFacts: [
			['Audio', 'EN · DTS 5.1'],
			['Subtitles', 'English · SRT'],
			['Quality', 'Original · 1080p']
		] as [string, string][]
	},

	stills: {
		eyebrow: 'The application, in stills',
		items: [
			{
				file: 'home.webp',
				alt: 'Real Theia home screen: continue-watching row and film posters.'
			},
			{
				file: 'series.webp',
				alt: 'Real Theia series screen: seasons and episodes of a show.'
			},
			{
				file: 'search.webp',
				alt: 'Real Theia search screen with results as you type.'
			},
			{
				file: 'profiles.webp',
				alt: 'Real Theia profile chooser: one profile per viewer.'
			},
			{
				file: 'onboarding.webp',
				alt: 'Real Theia onboarding: pointing the library at your folders.'
			}
		] as { file: string; alt: string }[]
	},

	difference: {
		eyebrow: 'Why Theia',
		title: 'What you gain. What you give up.',
		gainsTitle: 'You gain',
		gains: [
			['One binary', 'The server, interface, database and player travel together. No Docker stack to assemble.'],
			['No Theia account', 'Your library stays on your machine. Local access does not depend on a remote service.'],
			['A browser is enough', 'TV, phone or computer: open Theia, choose who is watching, start the film.']
		] as [string, string][],
		givesTitle: 'You give up',
		gives: [
			['A large ecosystem', 'Theia is younger and less extensible than Plex or Jellyfin.'],
			['Automatic quality everywhere', 'Transcoding depends on your machine’s codecs and processing power.'],
			['Image-based subtitles', 'PGS and VobSub tracks are identified but not rendered.']
		] as [string, string][],
		compare: 'Read the detailed comparison on GitHub',
		safetyTitle: 'Safe by default, provided you do not improvise the network.',
		safetyBody:
			'Theia is built for your local network. Don’t publish its port: use the built-in WireGuard remote access. GPL-3.0 code, a SHA-256 digest on every binary.'
	},

	closing: {
		eyebrow: 'Ready to try',
		title: 'Your library is already there.',
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
		] as [string, string][]
	},

	footer: {
		body: 'Theia is free software under GPL-3.0. Playback and transcoding: FFmpeg, under its own licence.',
		tmdb: 'This product uses the TMDB API but is not endorsed or certified by TMDB.',
		navLabel: 'Legal and project links',
		source: 'Source code',
		license: 'GPL-3.0 licence'
	}
};

export default copy;
