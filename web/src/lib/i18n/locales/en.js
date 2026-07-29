export const metadata = {
	code: 'en',
	label: 'English',
	htmlLang: 'en',
	localeTag: 'en-US'
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
	tagline: 'Personal media server',
	tmdbAttribution: 'This product uses the TMDB API but is not endorsed or certified by TMDB.',

	a11y: {
		mainNavigation: 'Main navigation'
	},

	nav: {
		home: 'Home',
		library: 'Movies',
		settings: 'Settings',
		profiles: 'Profiles',
		back: 'Back'
	},

	profiles: {
		eyebrow: 'In this household',
		title: 'Who’s watching?',
		body:
			'Profiles keep playback progress separate. They are not accounts: there are no passwords, and anyone on the local network can choose or edit any profile.',
		loading: 'Loading profiles…',
		listLabel: 'Choose a profile',
		defaultName: 'Default profile',
		current: 'Active profile',
		switchTo: (name) => `Switch profile — ${name}`,
		manage: 'Manage profiles',
		manageHint:
			'The photo stays in Theia’s local database. It is cropped and compressed before it is saved.',
		done: 'Done',
		name: 'Name',
		newName: 'New profile',
		namePlaceholder: 'First name or nickname',
		add: 'Add',
		adding: 'Adding…',
		rename: 'Save name',
		addPhoto: 'Add a photo',
		replacePhoto: 'Replace photo',
		removePhoto: 'Remove photo',
		delete: 'Delete',
		confirmDelete: 'Confirm deletion',
		notices: {
			profile_saved: 'Name saved.',
			avatar_saved: 'Photo saved.',
			avatar_removed: 'Photo removed.',
			profile_deleted: 'Profile deleted. Your movies were not affected.',
			invalid_profile_name: 'The name must be between 1 and 40 characters.',
			invalid_profile_payload: 'The submitted change is not valid.',
			profile_not_found: 'This profile no longer exists. Reload the list.',
			profile_limit_reached: 'The maximum number of profiles has been reached.',
			default_profile: 'The default profile cannot be deleted.',
			last_profile: 'The last profile cannot be deleted.',
			avatar_too_large: 'This image is too large. Choose a file under 8 MB.',
			invalid_avatar: 'Choose a valid JPEG, PNG, or WebP image.',
			profile_create_failed: 'The profile could not be added.',
			profile_update_failed: 'The profile could not be updated.',
			profile_delete_failed: 'The profile could not be deleted.',
			avatar_save_failed: 'The photo could not be saved.',
			avatar_delete_failed: 'The photo could not be removed.',
			unknown: 'The change could not be saved.'
		}
	},

	library: {
		title: 'All movies',
		loading: 'Loading library…',
		loadingProgress: (n) =>
			n > 0 ? `Loading library… ${n} movies received` : 'Loading library…',
		search: 'Search',
		searchPlaceholder: 'Title, director, genre…',
		clear: 'Clear',
		sort: 'Sort',
		sortTitle: 'Title',
		sortYear: 'Newest first',
		sortRating: 'Highest rated',
		sortAdded: 'Recently added',
		sortRuntime: 'Runtime',
		genre: 'Genre',
		allGenres: 'All genres',
		status: 'Watch status',
		statusAll: 'All',
		statusUnseen: 'Not started',
		statusInProgress: 'In progress',
		statusFinished: 'Finished',
		countAll: (n) => (n === 1 ? '1 movie' : `${n} movies`),
		countFiltered: (shown, total) => `${shown} of ${total} movies`,
		noResults: 'No movies match',
		noResultsBody: 'Try another search or remove a filter.',
		reset: 'Show everything',
		seeAll: 'See all',
		scrollPrevious: 'Scroll the row left',
		scrollNext: 'Scroll the row right',
		ratingLegend: (rating) => `${formatDecimal(rating)} / 10`,
		unwatchedBadge: 'Not watched',
		finishedBadge: 'Watched'
	},

	hero: {
		details: 'View details',
		resumeEyebrow: 'You were watching',
		resume: 'Resume',
		remaining: (duration) => `${duration} left`
	},

	rows: {
		continue: {
			title: 'Continue watching',
			href: '/films?status=progress'
		},
		recent: {
			title: 'Recently added',
			href: '/films?sort=added'
		},
		top_rated: {
			title: 'Top rated',
			href: '/films?sort=rating'
		},
		tonight: {
			title: 'Tonight’s random picks',
			href: null,
			hint: 'A selection that changes every day'
		}
	},

	home: {
		loading: 'Loading library…',
		emptyTitle: 'No movies yet',
		emptyBody:
			'Add one or more folders in Settings, then scan them. Theia will take care of the rest.',
		unreachableTitle: 'Server unavailable',
		unreachable:
			'The page loaded, but the API is not responding. The server has probably been stopped.',
		retry: 'Try again'
	},

	welcome: {
		eyebrow: 'First launch',
		title: 'Theia is ready',
		body:
			'Scan this code with a phone or tablet on the same network, or enter the address manually on a TV. There is nothing else to configure.',
		enter: 'View library'
	},

	problems: {
		directory_unreadable:
			'Folder cannot be read. If it is on an external drive, the drive is probably disconnected.',
		not_a_directory: 'This path points to a file, not a folder.',
		subdirectory_unreadable: 'Subfolder skipped: Theia could not read it.',
		file_unreadable: 'File skipped: Theia could not read its information.',
		save_failed: 'This file could not be saved to the library.',
		metadata_unavailable:
			'Metadata could not be retrieved during this scan. Theia will try again later.',
		metadata_key_rejected:
			'TMDB rejected the API key. Check it in Settings; movies will remain listed without artwork.',
		unknown: 'An unexpected problem occurred during the scan.'
	},

	updateReasons: {
		up_to_date: 'Theia is up to date.',
		no_release: 'No version has been published yet.',
		github_unreachable: 'GitHub is unavailable right now. Try again later.',
		playback_active: 'Playback is currently active.',
		no_binary_for_platform: 'This version has no binary for this system.',
		download_not_verified:
			'The downloaded file does not match the published checksum. Nothing was installed.',
		binary_did_not_run:
			'The downloaded file did not start correctly. Nothing was installed.',
		replace_failed:
			'The binary could not be replaced. The previous version was restored.',
		development_build: ''
	},

	update: {
		heading: 'Update',
		current: 'Installed version',
		latest: 'Latest published version',
		check: 'Check now',
		checking: 'Checking…',
		install: 'Install',
		installing: 'Installing…',
		upToDate: 'Theia is up to date.',
		available: 'A new version is available.',
		ready: 'Update installed. Theia is restarting — reload this page in a few seconds.',
		deferred:
			'Update postponed: playback is active. It will be offered again when playback has stopped.',
		failed:
			'The update failed. The installed version was not changed and is still running.',
		unsupported:
			'This build cannot update itself: it was compiled locally and cannot be compared with a published version.',
		notes: 'Release notes',
		lastChecked: 'Last checked'
	},

	connect: {
		heading: 'Connect a device',
		address: 'Local network address',
		copy: 'Copy',
		copied: 'Copied',
		mdns: 'Also available at',
		mdnsCaveat:
			'This name does not work on Android — use the IP address above instead.',
		otherAddresses: 'Other addresses for this machine',
		otherAddressesHint:
			'If the code leads nowhere, this machine has several network adapters and Theia may have selected the wrong one. Try one of these addresses.',
		virtual: 'virtual adapter'
	},

	player: {
		play: 'Play',
		pause: 'Pause',
		back10: 'Go back 10 seconds',
		forward10: 'Skip forward 10 seconds',
		position: 'Position in movie',
		volume: 'Volume',
		mute: 'Mute',
		unmute: 'Unmute',
		fullscreen: 'Full screen',
		close: 'Close player',

		resume: 'Resume',
		resumeAtMinutes: (minutes) => `Resume at ${minutes} min`,
		resumeAt: 'You stopped at',
		fromStart: 'Start from the beginning',
		continueWatching: 'Continue watching',

		buffering: 'Buffering…',
		seeking: 'Seeking…',
		remuxBadge: 'Remuxed on the fly',
		preparing:
			'Preparing playback — ffmpeg only needs to be downloaded once, so this may take a moment.',

		unavailable: 'Playback could not be prepared.',
		noFfmpeg:
			'This file needs to be remuxed, but ffmpeg is not available for this platform.',
		failed:
			'This file could not be played. Its format is probably not supported by v1.',

		shortcuts: {
			open: 'Show keyboard shortcuts',
			close: 'Close keyboard shortcuts',
			title: 'Keyboard shortcuts',
			intro: 'These keys work during playback without opening the controls.',
			items: [
				{ keys: ['Space'], action: 'Play or pause' },
				{ keys: ['←', '→'], action: 'Go back or forward 10 seconds' },
				{ keys: ['↓', '↑'], action: 'Turn the volume down or up' },
				{ keys: ['M'], action: 'Mute or unmute' },
				{ keys: ['F'], action: 'Enter or exit full screen' },
				{ keys: ['Home', 'End'], action: 'Go to the beginning or end of the movie' },
				{ keys: ['?'], action: 'Show or hide this help' }
			],
			scrubHint:
				'On the timeline, hold Shift with ← or → to skip 60 seconds.'
		}
	},

	film: {
		notFound: 'This movie could not be found.',
		overview: 'Overview',
		cast: 'Cast',
		director: 'Director',
		genres: 'Genres',
		runtime: 'Runtime',
		year: 'Year',
		file: 'File',
		size: 'Size',
		progress: 'Movie progress',
		moreByDirector: (director) => `See more movies by ${director}`,
		noOverview: 'No overview is available for this movie.',
		unmatched:
			'This file could not be identified on TMDB. It remains listed under its filename; ' +
			'renaming the file will trigger another search during the next scan.'
	},

	settings: {
		heading: 'Settings',
		interface: 'Interface',
		language: 'Language',
		languageHint:
			'This choice stays in this browser. It translates the interface without downloading or retranslating cached TMDB metadata.',
		profilesHint:
			'Each profile keeps its own resume position. Like the language, the selected profile belongs to this browser rather than to the server: switching profile on the television changes nothing on a laptop.',
		profilesAction: 'Switch profile',
		server: 'Server',
		version: 'Version',
		port: 'Port',
		hostname: 'mDNS name',
		dataDir: 'Data folder',
		library: 'Library',
		paths: 'Watched folders',
		noPaths: 'No watched folders.',
		films: 'Movies',
		scan: 'Scan folders',
		scanning: 'Scanning…',
		lastScan: 'Last scan',
		found: 'found',
		added: 'added',
		updated: 'updated',
		removed: 'removed',
		enriched: 'enriched',
		notFound: 'not identified',
		problems: 'Problems found',
		metadata: 'Metadata',
		source: 'Source',
		keySources: {
			settings: 'settings',
			'built-in': 'built-in key',
			'config.local.json': 'config.local.json',
			none: 'none',
			unknown: 'unknown'
		},
		noKeyAdvice:
			'No TMDB key is configured: movies remain listed from their filenames, without posters or synopses. Add a personal key below to enable metadata.',

		edit: 'Edit',
		cancel: 'Cancel',
		save: 'Save',
		saving: 'Saving…',
		saved: 'Settings saved.',
		saveFailed: 'Settings could not be saved.',
		invalidPort: 'The port must be a number between 1 and 65535.',
		addPath: 'Add a folder',
		removePath: 'Remove',
		pathPlaceholder: 'C:\\Users\\you\\Videos',
		portHint: 'The port change will take effect the next time Theia starts.',
		portChanged:
			'The new port is saved, but Theia is still listening on the old one. Restart it to apply the change.',
		missingPaths:
			'Saved, but these folders cannot be found right now — this is normal if the drive is disconnected:',
		keyLabel: 'Personal TMDB key',
		keyHint:
			'Optional. Leave this blank to use the key provided with Theia. A key entered here takes priority.',
		keyPlaceholder: 'Leave blank to use the built-in key',
		milestone: 'Theia v1'
	},

	errors: {
		scanFailed: 'The scan could not be completed.',
		scanBusy: 'A scan is already in progress.'
	},

	notFound: {
		eyebrow: 'Page not found',
		title: 'This address leads nowhere',
		body:
			'The link may be incomplete, or the page may have been renamed. Your library has not moved.',
		crash: 'An unexpected error',
		crashBody:
			'The interface stopped because of an error. Reloading the page almost always fixes it; if the problem returns, the server may need to be restarted.',
		home: 'Back to home',
		reload: 'Reload'
	}
};

export function formatUptime(seconds) {
	if (seconds < 60) return `${seconds} sec`;
	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return `${minutes} min`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours} hr ${minutes % 60} min`;
	const days = Math.floor(hours / 24);
	return `${days} d ${hours % 24} hr`;
}

export function formatSize(bytes) {
	if (!bytes) return '—';
	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
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
		? `${hours} hr`
		: `${hours} hr ${String(remainingMinutes).padStart(2, '0')} min`;
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
