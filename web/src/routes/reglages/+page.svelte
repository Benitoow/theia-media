<script>
	import { onMount } from 'svelte';
	import { getJSON } from '$lib/api.js';
	import { i18n } from '$lib/i18n/index.svelte.js';
	import { strings as t, formatUptime } from '$lib/strings.js';
	import { profileSession } from '$lib/profiles.svelte.js';
	import ConnectPanel from '$lib/components/ConnectPanel.svelte';
	import ProfileAvatar from '$lib/components/ProfileAvatar.svelte';

	let health = $state(null);
	let stats = $state(null);
	let settings = $state(null);
	let connect = $state(null);
	let update = $state(null);
	let updateBusy = $state(false);

	let editing = $state(false);
	let saving = $state(false);
	let saveNotice = $state(null);
	let draft = $state({ library_paths: [], port: 8383, tmdb_api_key: '' });

	function startEditing() {
		// The key is never sent to the browser, so the field starts empty and an
		// empty field means "leave it alone" -- see saveSettings.
		draft = {
			library_paths: [...(settings.library_paths ?? [])],
			port: settings.port,
			tmdb_api_key: ''
		};
		saveNotice = null;
		editing = true;
	}

	async function saveSettings() {
		saving = true;
		saveNotice = null;
		try {
			const body = {
				library_paths: draft.library_paths,
				port: Number(draft.port)
			};
			// Omitted rather than sent empty: sending "" would clear a key the
			// user never intended to touch.
			if (draft.tmdb_api_key.trim() !== '') {
				body.tmdb_api_key = draft.tmdb_api_key.trim();
			}

			const res = await fetch('/api/settings', {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body)
			});
			const result = await res.json();
			if (!res.ok) {
				// Never result.error: the server writes English for its log and
				// for anybody reading the API. The interface owns its language.
				saveNotice = {
					ok: false,
					code: res.status === 400 ? 'invalidPort' : 'saveFailed'
				};
				return;
			}

			saveNotice = {
				ok: true,
				portChanged: Boolean(result.port_changed),
				missingPaths: result.missing_paths ?? []
			};
			editing = false;
			await refresh();
		} catch {
			saveNotice = { ok: false, code: 'saveFailed' };
		} finally {
			saving = false;
		}
	}

	function saveNoticeText(notice) {
		if (!notice) return '';
		if (!notice.ok) return t.settings[notice.code] ?? t.settings.saveFailed;

		const parts = [t.settings.saved];
		if (notice.portChanged) parts.push(t.settings.portChanged);
		if (notice.missingPaths?.length) {
			parts.push(`${t.settings.missingPaths} ${notice.missingPaths.join(', ')}`);
		}
		return parts.join(' ');
	}

	async function checkUpdate() {
		updateBusy = true;
		try {
			update = await getJSON('/api/update/check', { method: 'POST' });
		} catch {
			// The status object carries the explanation; a failed check is not
			// worth an error banner of its own.
		} finally {
			updateBusy = false;
		}
	}

	async function applyUpdate() {
		updateBusy = true;
		try {
			const res = await fetch('/api/update/apply', { method: 'POST' });
			update = await res.json();
		} catch {
			// Losing the response does not mean the update failed; the next
			// poll reports the real state.
		} finally {
			updateBusy = false;
		}
	}
	let scanning = $state(false);
	let scanError = $state(null);

	async function refresh() {
		try {
			[health, stats, settings, connect, update] = await Promise.all([
				getJSON('/api/health'),
				getJSON('/api/library/stats'),
				getJSON('/api/settings'),
				getJSON('/api/onboarding'),
				getJSON('/api/update')
			]);
		} catch {
			// The layout already shows a dead page well enough; a settings screen
			// that cannot reach the server has nothing useful to add.
		}
	}

	async function scan() {
		scanning = true;
		scanError = null;
		try {
			await getJSON('/api/library/scan', { method: 'POST' });
			await refresh();
		} catch (e) {
			scanError = e.status === 409 ? 'scanBusy' : 'scanFailed';
		} finally {
			scanning = false;
		}
	}

	onMount(() => {
		refresh();
		const timer = setInterval(refresh, 10_000);
		return () => clearInterval(timer);
	});

	const busy = $derived(scanning || stats?.scanning);
</script>

<svelte:head>
	<title>{t.settings.heading} — {t.appName}</title>
</svelte:head>

<main class="settings-page page-shell page-body max-w-6xl">
	<h1 class="page-title enter mb-14">{t.settings.heading}</h1>

	{#if settings && stats}
		<!-- A browser preference, deliberately outside the server settings PUT:
		     changing language on the TV must not change somebody else's laptop. -->
		<section class="mb-14 border-b border-line pb-14">
			<h2 class="label mb-5">{t.settings.interface}</h2>
			<p class="text-small mb-5 max-w-prose text-muted">{t.settings.languageHint}</p>
			<div class="flex flex-wrap gap-3" role="group" aria-label={t.settings.language}>
				{#each i18n.available as locale (locale.code)}
					<button
						type="button"
						lang={locale.lang}
						class="tv-action cursor-pointer"
						class:tv-action--primary={i18n.locale === locale.code}
						aria-pressed={i18n.locale === locale.code}
						onclick={() => i18n.setLocale(locale.code)}
					>
						{locale.label}
					</button>
				{/each}
			</div>
		</section>

		<!-- The way back to the chooser, now that the nav no longer carries a
		     profile pill. Settings is where you go to change something, and this
		     is a something you change rarely. -->
		<section class="mb-14 border-b border-line pb-14">
			<h2 class="label mb-5">{t.nav.profiles}</h2>
			<p class="text-small mb-6 max-w-prose text-muted">{t.settings.profilesHint}</p>
			<div class="flex flex-wrap items-center gap-5">
				<span class="flex items-center gap-3">
					<ProfileAvatar profile={profileSession.active} />
					<span class="text-small text-parchment">
						{profileSession.active?.name || t.profiles.defaultName}
					</span>
				</span>
				<a href="/profils?return=%2Freglages" class="tv-action">
					{t.settings.profilesAction}
				</a>
			</div>
		</section>

		<!-- The welcome screen shows this once. Keeping it reachable afterwards
		     is what you want the evening a second device turns up. -->
		{#if connect}
			<section class="mb-14 border-b border-line pb-14">
				<h2 class="label mb-6">{t.connect.heading}</h2>
				<ConnectPanel info={connect} />
			</section>
		{/if}

		<section class="mb-14">
			<h2 class="label mb-5">{t.settings.server}</h2>
			<dl class="grid gap-x-8 gap-y-3 sm:grid-cols-[11rem_1fr]">
				<dt class="label">{t.settings.version}</dt>
				<dd class="text-small text-parchment">{settings.version}</dd>
				<dt class="label">{t.settings.port}</dt>
				<dd class="text-small text-parchment">{settings.port}</dd>
				<dt class="label">{t.settings.hostname}</dt>
				<dd class="text-small text-parchment">{settings.hostname}.local</dd>
				<dt class="label">{t.settings.dataDir}</dt>
				<dd class="text-small break-all text-parchment">{settings.data_dir}</dd>
				{#if health}
					<dt class="label">{t.settings.lastScan}</dt>
					<dd class="text-small text-parchment">{formatUptime(health.uptime_seconds)}</dd>
				{/if}
			</dl>
		</section>

		<section class="mb-14">
			<div class="mb-5 flex flex-wrap items-baseline justify-between gap-4">
				<h2 class="label">{t.settings.library}</h2>
				<button
					type="button"
					onclick={scan}
					disabled={busy}
					class="tv-action cursor-pointer disabled:cursor-not-allowed disabled:text-faint"
				>
					{busy ? t.settings.scanning : t.settings.scan}
				</button>
			</div>

			{#if scanError}
				<p class="mb-5 border-l border-error py-1 pl-5 text-small text-parchment">
					{t.errors[scanError] ?? t.errors.scanFailed}
				</p>
			{/if}

			<dl class="mb-6 grid gap-x-8 gap-y-3 sm:grid-cols-[11rem_1fr]">
				<dt class="label">{t.settings.films}</dt>
				<dd class="text-small text-parchment">{stats.movies}</dd>
			</dl>

			<!-- The three things the spec allows to be configured, and nothing
			     else. Editable here so that nobody has to find config.json. -->
			<div class="border-t border-line pt-6">
				<div class="mb-4 flex flex-wrap items-baseline justify-between gap-4">
					<span class="label">{t.settings.paths}</span>
					{#if !editing}
						<button type="button" onclick={startEditing} class="label hover:text-bone">
							{t.settings.edit}
						</button>
					{/if}
				</div>

				{#if !editing}
					{#if settings.library_paths?.length}
						{#each settings.library_paths as path (path)}
							<p class="text-small break-all text-parchment">{path}</p>
						{/each}
					{:else}
						<p class="text-small text-muted">{t.settings.noPaths}</p>
					{/if}
				{:else}
					<div class="space-y-3">
						{#each draft.library_paths as _, i (i)}
							<div class="flex gap-3">
								<input
									type="text"
									bind:value={draft.library_paths[i]}
									placeholder={t.settings.pathPlaceholder}
									class="min-w-0 flex-1 border border-line bg-surface px-4 py-2.5 text-small
									       text-bone placeholder:text-faint focus:border-muted focus:outline-none"
								/>
								<button
									type="button"
									onclick={() => draft.library_paths.splice(i, 1)}
									class="label shrink-0 hover:text-error"
								>
									{t.settings.removePath}
								</button>
							</div>
						{/each}
						<button
							type="button"
							onclick={() => draft.library_paths.push('')}
							class="label hover:text-bone"
						>
							+ {t.settings.addPath}
						</button>
					</div>

					<div class="mt-8 grid gap-6 sm:grid-cols-2">
						<label class="block">
							<span class="label">{t.settings.port}</span>
							<input
								type="number"
								min="1"
								max="65535"
								bind:value={draft.port}
								class="mt-2 w-full border border-line bg-surface px-4 py-2.5 text-small
								       text-bone focus:border-muted focus:outline-none"
							/>
							<span class="label mt-2 block normal-case tracking-normal">
								{t.settings.portHint}
							</span>
						</label>

						<label class="block">
							<span class="label">{t.settings.keyLabel}</span>
							<input
								type="password"
								autocomplete="off"
								bind:value={draft.tmdb_api_key}
								placeholder={t.settings.keyPlaceholder}
								class="mt-2 w-full border border-line bg-surface px-4 py-2.5 text-small
								       text-bone placeholder:text-faint focus:border-muted focus:outline-none"
							/>
							<span class="label mt-2 block normal-case tracking-normal">
								{t.settings.keyHint}
							</span>
						</label>
					</div>

					<div class="mt-8 flex flex-wrap items-center gap-4">
						<button
							type="button"
							onclick={saveSettings}
							disabled={saving}
							class="tv-action tv-action--primary cursor-pointer disabled:cursor-not-allowed
							       disabled:text-faint"
						>
							{saving ? t.settings.saving : t.settings.save}
						</button>
						<button type="button" onclick={() => (editing = false)} class="label hover:text-bone">
							{t.settings.cancel}
						</button>
					</div>
				{/if}

				{#if saveNotice}
					<p
						class="mt-5 border-l py-1 pl-5 text-small text-parchment"
						class:border-accent={saveNotice.ok}
						class:border-error={!saveNotice.ok}
					>
						{saveNoticeText(saveNotice)}
					</p>
				{/if}
			</div>

			{#if stats.last_scan}
				<dl class="flex flex-wrap gap-x-8 gap-y-2 border-t border-line pt-5">
					{#each [[t.settings.found, stats.last_scan.found], [t.settings.added, stats.last_scan.added], [t.settings.updated, stats.last_scan.updated], [t.settings.removed, stats.last_scan.removed], [t.settings.enriched, stats.last_scan.enriched], [t.settings.notFound, stats.last_scan.not_found]] as [label, value] (label)}
						<div class="flex gap-2">
							<dt class="label">{label}</dt>
							<dd class="text-label text-parchment">{value}</dd>
						</div>
					{/each}
				</dl>

				{#if stats.last_scan.problems?.length}
					<div class="mt-6 border-l border-warning py-1 pl-5">
						<p class="label mb-2">{t.settings.problems}</p>
						<ul class="space-y-3">
							{#each stats.last_scan.problems as problem (problem.kind + (problem.path ?? ''))}
								<li class="text-small text-parchment">
									{t.problems[problem.kind] ?? t.problems.unknown}
									{#if problem.path}
										<span class="label mt-1 block break-all normal-case tracking-normal">
											{problem.path}
										</span>
									{/if}
								</li>
							{/each}
						</ul>
					</div>
				{/if}
			{/if}
		</section>

		<section>
			<h2 class="label mb-5">{t.settings.metadata}</h2>
			{#if settings.tmdb.configured}
				<p class="text-small text-parchment">
					TMDB ·
					<span class="text-muted">
						{t.settings.source}
						{t.settings.keySources[settings.tmdb.source] ?? t.settings.keySources.unknown}
					</span>
				</p>
			{:else}
				<p class="border-l border-warning py-1 pl-5 text-small text-parchment">
					{t.settings.noKeyAdvice}
				</p>
			{/if}
		</section>

		{#if update}
			<section class="mt-14 border-t border-line pt-14">
				<div class="mb-5 flex flex-wrap items-baseline justify-between gap-4">
					<h2 class="label">{t.update.heading}</h2>
					{#if update.state !== 'unsupported'}
						<button
							type="button"
							onclick={update.state === 'available' ? applyUpdate : checkUpdate}
							disabled={updateBusy || update.state === 'ready' || update.state === 'downloading'}
							class="ease-cine cursor-pointer border px-6 py-3 text-label uppercase
							       transition-colors duration-160 disabled:cursor-not-allowed
							       disabled:text-faint"
							class:border-accent={update.state === 'available'}
							class:text-accent={update.state === 'available'}
							class:hover:bg-accent={update.state === 'available'}
							class:hover:text-ink={update.state === 'available'}
							class:border-line={update.state !== 'available'}
							class:hover:border-muted={update.state !== 'available'}
						>
							{#if updateBusy}
								{update.state === 'available' ? t.update.installing : t.update.checking}
							{:else if update.state === 'available'}
								{t.update.install}
							{:else}
								{t.update.check}
							{/if}
						</button>
					{/if}
				</div>

				<dl class="mb-6 grid gap-x-8 gap-y-3 sm:grid-cols-[11rem_1fr]">
					<dt class="label">{t.update.current}</dt>
					<dd class="text-small text-parchment">{update.current_version}</dd>
					{#if update.latest_version}
						<dt class="label">{t.update.latest}</dt>
						<dd class="text-small text-parchment">{update.latest_version}</dd>
					{/if}
				</dl>

				<!-- One line saying plainly where things stand. The failed state
				     says the installed version is untouched, because that is the
				     first thing anybody wants to know. -->
				{#if update.state === 'unsupported'}
					<p class="border-l border-line py-1 pl-5 text-small text-muted">{t.update.unsupported}</p>
				{:else if update.state === 'available'}
					<p class="border-l border-accent py-1 pl-5 text-small text-parchment">{t.update.available}</p>
				{:else if update.state === 'ready'}
					<p class="border-l border-accent py-1 pl-5 text-small text-parchment">{t.update.ready}</p>
				{:else if update.state === 'deferred'}
					<p class="border-l border-warning py-1 pl-5 text-small text-parchment">
						{t.update.deferred}
					</p>
				{:else if update.state === 'failed'}
					<p class="border-l border-error py-1 pl-5 text-small text-parchment">
						{t.updateReasons[update.reason] ?? t.update.failed}
					</p>
				{:else}
					<p class="text-small text-muted">
						{t.updateReasons[update.reason] || t.update.upToDate}
					</p>
				{/if}

				{#if update.release_url && update.state === 'available'}
					<a
						href={update.release_url}
						target="_blank"
						rel="noreferrer"
						class="label ease-cine mt-5 inline-block transition-colors duration-160 hover:text-bone"
					>
						{t.update.notes} ↗
					</a>
				{/if}
			</section>
		{/if}

		<p class="micro mt-16 border-t border-line pt-6">{t.settings.milestone}</p>
	{/if}
</main>
