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

	// Containers write the language as ISO 639, and not always the same variant:
	// "fre" and "fra" both mean French, so does "fr". The server passes on the
	// code it read (decision 25); this is where it becomes a word. An unknown
	// code is shown in capitals, which is legible and invents nothing.
	languages: {
		fra: 'French', fre: 'French', fr: 'French',
		eng: 'English', en: 'English',
		spa: 'Spanish', es: 'Spanish',
		deu: 'German', ger: 'German', de: 'German',
		ita: 'Italian', it: 'Italian',
		por: 'Portuguese', pt: 'Portuguese',
		nld: 'Dutch', dut: 'Dutch', nl: 'Dutch',
		jpn: 'Japanese', ja: 'Japanese',
		zho: 'Chinese', chi: 'Chinese', zh: 'Chinese',
		kor: 'Korean', ko: 'Korean',
		rus: 'Russian', ru: 'Russian',
		ara: 'Arabic', ar: 'Arabic',
		pol: 'Polish', pl: 'Polish',
		swe: 'Swedish', sv: 'Swedish',
		dan: 'Danish', da: 'Danish',
		nor: 'Norwegian', no: 'Norwegian',
		fin: 'Finnish', fi: 'Finnish',
		tur: 'Turkish', tr: 'Turkish',
		ces: 'Czech', cze: 'Czech', cs: 'Czech',
		ell: 'Greek', gre: 'Greek', el: 'Greek',
		heb: 'Hebrew', he: 'Hebrew',
		hin: 'Hindi', hi: 'Hindi',
		und: 'Unspecified language', mul: 'Multilingual'
	},

	nav: {
		home: 'Home',
		library: 'Movies',
		settings: 'Settings',
		back: 'Back'
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
		series_continue: {
			title: 'Continue a series',
			href: '/series'
		},
		series_recent: {
			title: 'Recently added series',
			href: '/series'
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

		// The server sends a code, never a sentence (decision 25). This table is
		// the only place where an M1 code becomes English.
		codes: {
			browser_cannot_decode_video:
				'Your browser cannot decode this file\u2019s video: the sound would run on a frozen picture. Open it in another browser, or choose another file on the movie page.',
			invalid_movie_id: 'This movie cannot be identified.',
			invalid_file_id: 'This file cannot be identified.',
			invalid_audio_track_id: 'This audio track cannot be identified.',
			audio_selection_requires_remux: 'This track can only be guaranteed by remuxing.',
			file_outside_library:
				'This file is no longer inside a watched folder. Check the library folders in settings.',
			movie_not_found: 'This movie could not be found.',
			file_not_found: 'This file no longer belongs to this movie.',
			audio_track_not_found: 'This audio track no longer exists in this file.',
			media_file_unavailable:
				'This file has disappeared from disk. Run another scan from settings.',
			media_not_inspected: 'Analyse this file before choosing an audio track.',
			media_unreadable:
				'This file could not be decoded. It may be incomplete or corrupted.',
			transcode_busy:
				'Another playback is already re-encoding video on this machine. Wait for it to ' +
				'finish, or choose the file as it is, which needs no computation.',
			video_transcode_required:
				'This file uses a video format that needs full conversion, planned for a later milestone.',
			movie_unavailable: 'The movie could not be read from the database.',
			file_unavailable: 'The file could not be read from the database.',
			audio_track_unavailable: 'The audio track could not be read from the database.',
			media_file_unreadable: 'This file could not be opened on disk.',
			media_inspection_not_saved: 'The result of the analysis could not be saved.',
			stream_start_failed: 'Playback could not be started.',
			ffmpeg_unsupported: 'No ffmpeg build is available for this platform.',
			ffmpeg_unavailable: 'ffmpeg could not be prepared. Check the connection, then try again.'
		},

		tracks: {
			open: 'Audio and subtitles',
			title: 'Audio and subtitles',
			quality: 'Quality',
			original: 'As the file is',
			height: (h) => h + 'p',
			reencoded: 're-encoded',
			kinds: {
				hardware: 'graphics card',
				software: 'processor'
			},
			subtitles: 'Subtitles',
			noSubtitles: 'None',
			external: 'separate file',
			forced: 'forced',
			unnamedSubtitle: (index) => `Subtitles ${index}`,
			// Decision 3: an image track can only be shown by burning it into
			// the picture, which is the full transcode v1 refuses. Listed all
			// the same, or somebody goes looking for it.
			imageBased: 'image — cannot be shown'
		},

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
		progress: 'Movie progress',
		files: {
			one: 'File',
			many: 'Available files',
			choose: 'Choose which file to play',
			chosen: 'Chosen file',
			primary: 'Primary',
			pending: 'Characteristics not measured',
			inspect: 'Analyse',
			inspecting: 'Analysing…',
			retry: 'Run the analysis again',
			errored: 'This file could not be analysed.',
			resolution: (width, height) => `${width} × ${height}`,
			trackCount: (count) => (count > 1 ? `${count} audio tracks` : `${count} audio track`),
			hint:
				'Theia never picks a quality for you: several files exist and you decide which ' +
				'one to play.',
			tracksInPlayer: 'Audio tracks and subtitles are chosen while the film plays.'
		},
		audio: {
			title: 'Audio track',
			choose: 'Choose the audio track',
			auto: "The file's default track",
			isDefault: 'default',
			unnamed: (index) => `Track ${index}`,
			remuxNote:
				'Choosing a track explicitly goes through a remux, the only way to guarantee which ' +
				'one the browser will play.'
		},
		moreByDirector: (director) => `See more movies by ${director}`,
		noOverview: 'No overview is available for this movie.',
		unmatched:
			'This file could not be identified on TMDB. It remains listed under its filename; ' +
			'renaming the file will trigger another search during the next scan.'
	},

	remote: {
		heading: 'Remote access',
		intro:
			'Theia can be reached from outside the local network through an embedded WireGuard tunnel. No account, no relay, no outside service: every device proves a key created here.',
		opening: 'Opening the port…',
		publicAddress: 'Tunnel entry point',
		tunnelAddress: 'Address to open once connected',
		howItWorks:
			'The entry point is not a web address: nothing answers there in a browser, and that ' +
			'is correct. It is for WireGuard. Install WireGuard on the device, import the ' +
			'configuration created below, turn the tunnel on, then open the address above.',
		step1: 'Install WireGuard on the device (App Store, Google Play, wireguard.com).',
		step2: 'Import this configuration: scan the code, or open the downloaded .conf file.',
		step3: 'Turn the tunnel on, then open this address in the browser:',
		opened: 'Port opened by',
		methods: {
			upnp: 'UPnP',
			natpmp: 'NAT-PMP'
		},
		addressChanged:
			'Your public address has changed since remote access was switched on. Devices created ' +
			'before that point at an address that is no longer yours: create them again.',
		manual: 'Set the port and address by hand',
		retryAutomatic: 'Ask the router again',
		// The server sends a code; these sentences belong to the interface.
		discovery: {
			remote_router_silent:
				'Your router did not answer. Automatic port opening (UPnP or NAT-PMP) is probably ' +
				'switched off in its settings — which is the default on some ISP boxes. Turn it on, ' +
				'or open the port by hand below.',
			remote_router_refused:
				'Your router refused to open this port. It may already be forwarded to another ' +
				'machine, or forwarding may be locked. Try another port, or open it by hand below.',
			remote_carrier_nat:
				'Your provider places you behind carrier-grade NAT: your connection has no public ' +
				'address of its own, and no port forwarding can reach through it. Ask them for a ' +
				'public IP address — it is usually free and immediate.'
		},
		router:
			'Your router must forward the UDP port below to this machine. Never forward Theia’s TCP port: it has no authentication at all.',
		cgnat:
			'If your provider puts you behind CGNAT, no forwarding is possible and Theia cannot work around it.',
		state: 'State',
		stateDisabled: 'Disabled',
		stateRunning: 'Running',
		stateError: 'Error',
		enable: 'Enable remote access',
		disable: 'Disable',
		port: 'Listening UDP port',
		portHelp: 'The port your router forwards to this machine.',
		portChange: 'Changing the port restarts the tunnel and briefly cuts connected devices.',
		endpoint: 'Public address',
		endpointPlaceholder: 'media.example.net:51820',
		endpointHelp:
			'As host:port, without http://. The public port may differ from the listening port.',
		endpointExamples:
			'Examples: media.example.net:51820, 203.0.113.10:51820, [2001:db8::10]:51820',
		endpointChange:
			'Changing this address does not update devices already created: edit their WireGuard configuration, or revoke and recreate them.',
		reachability: 'Reachability',
		unverified: 'Never confirmed',
		unverifiedHelp:
			'No device has proven the path yet. This is not an error: Theia does not test your router from outside.',
		confirmed: 'Confirmed',
		confirmedHelp:
			'At least one device has completed a handshake since this start. It is not a permanent guarantee.',
		save: 'Save',
		saving: 'Saving…',
		devices: 'Devices',
		noDevices: 'No device has been created yet.',
		addDevice: 'Add a device',
		deviceName: 'Device name',
		devicePlaceholder: 'Living room television',
		create: 'Create',
		creating: 'Creating…',
		enableFirst: 'Enable remote access before creating a device.',
		address: 'Address inside the tunnel',
		lastHandshake: 'Last handshake',
		neverConnected: 'Never connected',
		traffic: 'Traffic',
		trafficHelp: 'Tunnel network counters, not viewing statistics.',
		revoke: 'Revoke',
		revokeConfirm: (name) =>
			'Revoke ' + name + '? This device loses access immediately and its key can never be reactivated.',
		revokeYes: 'Revoke this device',
		cancel: 'Cancel',

		provisionTitle: (name) => name + ' is ready',
		provisionWarning:
			'This QR code and this file contain a private key. They will never be shown again.',
		provisionScan: 'Scan this code with the device’s WireGuard app.',
		copyConfig: 'Copy the configuration',
		copied: 'Copied',
		downloadConfig: 'Download the .conf',
		done: 'I have kept the configuration',
		closeWarning:
			'Close without keeping the configuration? It cannot be shown again: you would have to revoke this device and create another.',
		closeAnyway: 'Close anyway',
		lost: 'Lost the configuration? Revoke this device and create a new one.',

		remoteBadge: 'Remote access',
		remoteContext: (name) => 'Connected from ' + name,
		remoteRestricted:
			'Settings, scanning and updates are only available from the local network.',

		codes: {
			invalid_remote_access_payload: 'The request could not be read.',
			invalid_remote_listen_port: 'This port is not valid. Choose a port between 1 and 65535.',
			invalid_remote_endpoint:
				'This address is not valid. Expected host:port, without http:// or a path.',
			invalid_remote_peer_payload: 'The request could not be read.',
			invalid_remote_peer_name: 'A name is required, from 1 to 64 characters.',
			invalid_remote_peer_id: 'This device cannot be identified.',
			remote_peer_limit_reached: 'The limit of 32 active devices has been reached.',
			remote_peer_not_found: 'This device no longer exists.',
			remote_access_disabled: 'Enable remote access before this operation.',
			remote_access_not_ready: 'The tunnel is not ready. Try again in a moment.',
			remote_access_unavailable: 'Remote access is unavailable.'
		},

		reasons: {
			remote_config_invalid: 'The saved configuration is not usable.',
			remote_key_unavailable:
				'The server key cannot be read. If this data directory came from another machine or another Windows account, disable remote access, remove remote-access.key, enable it again and recreate the devices.',
			remote_listen_failed: 'The UDP port could not be opened. It may already be in use.',
			remote_listener_stopped: 'The tunnel stopped on its own.',
			remote_peer_reload_failed:
				'Devices could not be reloaded; the tunnel was closed as a precaution.',
			remote_restore_failed: 'The previous configuration could not be restored.'
		}
	},

	series: {
		title: 'Series',
		loading: 'Loading series…',
		countAll: (n) => (n === 1 ? '1 series' : `${n} series`),
		emptyTitle: 'No series',
		emptyBody:
			'Arrange episodes in series and season folders, named SxxExx, then run another scan from settings.',
		notFound: 'This series could not be found.',
		episodeNotFound: 'This episode could not be found.',
		seasons: 'Seasons',
		specials: 'Specials',
		specialsHelp:
			'Season 0 holds everything outside the numbering: making-of features, ' +
			'Christmas episodes, recaps. It only appears when a "Specials" or ' +
			'"Season 00" folder exists in your library, and it never takes part in ' +
			'autoplay.',
		season: (number) => `Season ${number}`,
		ownedEpisodes: (n) => (n === 1 ? '1 episode owned' : `${n} episodes owned`),
		episodeLabel: (number) => `Episode ${number}`,
		episodeRange: (first, last) => `Episodes ${first} to ${last}`,
		combined: 'Combined episode',
		combinedHint:
			'These episodes are in one file: they play in a single run and share one resume position.',
		gap: 'An episode is missing before the next one',
		next: 'Next episode',
		lastOwned: 'Last episode owned',
		unmatched:
			'This series was not identified on TMDB. It remains listed under its folder name.',
		noOverview: 'No overview is available.',
		play: 'Play episode',
		resumeAtMinutes: (minutes) => `Resume at ${minutes} min`
	},

	profiles: {
		question: "Who's watching?",
		defaultName: 'Main profile',
		manage: 'Manage profiles',
		done: 'Done',
		switch: 'Switch profile',
		current: (name) => `Active profile: ${name}`,
		choose: (name) => `Watch as ${name}`,
		add: 'Add a profile',
		create: 'Create profile',
		cancel: 'Cancel',
		nameLabel: 'Profile name',
		namePlaceholder: 'Mimi',
		edit: (name) => `Edit ${name}`,
		editShort: 'Edit',
		save: 'Save',
		picture: 'Picture',
		pictureChange: 'Choose a picture',
		pictureRemove: 'Remove the picture',
		pictureHint:
			'The picture stays on this machine. It is cropped square and re-encoded; ' +
			'nothing from the original file is kept.',
		pictureUploading: 'Uploading the picture…',
		details: 'Details',
		createdAt: 'Created',
		moviesStarted: 'Movies started',
		moviesFinished: 'Movies finished',
		episodesStarted: 'Episodes started',
		episodesFinished: 'Episodes finished',
		lastWatched: 'Last watched',
		never: 'Never',
		delete: 'Delete this profile',
		deleteConfirm: (name) =>
			`Delete ${name}? Their progress will be lost. Other profiles are unaffected.`,
		deleteYes: 'Delete',
		empty: 'No profiles.',
		loading: 'Loading profiles…',
		codes: {
			invalid_profile_id: 'This profile cannot be identified.',
			invalid_profile_payload: 'The request could not be read.',
			invalid_profile_name:
				'A name is required, without control characters, and at most 40 characters.',
			profile_not_found: 'This profile no longer exists. It may have been deleted elsewhere.',
			profile_image_not_found: 'This profile has no picture.',
			profile_limit_reached: 'The maximum number of profiles has been reached.',
			profile_last_remaining: 'The last profile cannot be deleted.',
			profile_image_too_large: 'This image is too large.',
			profile_image_unreadable: 'This file is not a usable image.',
			profile_unavailable: 'Profiles are unavailable at the moment.'
		}
	},

	settings: {
		heading: 'Settings',
		interface: 'Interface',
		language: 'Language',
		languageHint:
			'This choice stays in this browser. It translates the interface without downloading or retranslating cached TMDB metadata.',
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
