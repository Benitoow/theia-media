<script>
	// The chooser, the management view and one profile's page, in one route.
	//
	// A full screen with the navigation suppressed, not a pill or a dropdown in
	// the nav bar. Decision 35 measured why that fails here — a 2rem avatar and an
	// 11px name are unreadable from a sofa, four targets overflowed the 320px
	// floor, and the first arrow key landed on navigation rather than on the
	// question the app opens with. Decision 48 kept that finding when the
	// maintainer's reference showed a dropdown.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { profiles } from '$lib/profiles.svelte.js';
	import { strings as t, formatSize } from '$lib/strings.js';
	import { getJSON } from '$lib/api.js';
	import { i18n } from '$lib/i18n/index.svelte.js';
	import Icon from '$lib/components/Icon.svelte';
	import ProfileMark from '$lib/components/ProfileMark.svelte';
	import LoadingSkeleton from '$lib/components/LoadingSkeleton.svelte';

	/** @type {'loading' | 'choosing' | 'managing' | 'detail' | 'failed'} */
	let view = $state('loading');
	let failureCode = $state(null);

	let detail = $state(null);
	let creating = $state(false);
	let draftName = $state('');
	let busy = $state(false);
	let formError = $state(null);
	let confirmingDelete = $state(false);
	/** @type {HTMLInputElement} */
	let fileInput = $state();

	const list = $derived(profiles.list);
	// Leaving is only offered when a profile is already active. Arriving here
	// because the application needs an answer must not offer a way out without
	// giving one.
	const canLeave = $derived(profiles.activeID !== null);
	const nameOf = (profile) => profile?.name || t.profiles.defaultName;
	const message = (code) => t.profiles.codes[code] ?? t.profiles.codes.profile_unavailable;

	onMount(async () => {
		try {
			await profiles.load();
			view = $page.url.searchParams.has('gerer') ? 'managing' : 'choosing';
			// A profile's page is addressable, so reloading it does not throw the
			// viewer back to the row they came from.
			const wanted = Number($page.url.searchParams.get('profil'));
			if (Number.isInteger(wanted) && wanted > 0) {
				await openDetail({ id: wanted });
			}
		} catch (error) {
			failureCode = error?.code ?? 'profile_unavailable';
			view = 'failed';
		}
	});

	function choose(profile) {
		profiles.select(profile.id);
		goto('/');
	}

	async function openDetail(profile) {
		busy = true;
		formError = null;
		try {
			detail = await getJSON(`/api/profiles/${profile.id}`);
			draftName = detail.name ?? '';
			confirmingDelete = false;
			view = 'detail';
		} catch (error) {
			formError = message(error?.code);
		} finally {
			busy = false;
		}
	}

	async function submitCreate(event) {
		event.preventDefault();
		busy = true;
		formError = null;
		try {
			await profiles.create(draftName);
			draftName = '';
			creating = false;
		} catch (error) {
			formError = message(error?.code);
		} finally {
			busy = false;
		}
	}

	async function submitRename(event) {
		event.preventDefault();
		busy = true;
		formError = null;
		try {
			detail = await profiles.rename(detail.id, draftName);
		} catch (error) {
			formError = message(error?.code);
		} finally {
			busy = false;
		}
	}

	async function uploadPicture(event) {
		const file = event.currentTarget.files?.[0];
		if (!file) return;
		busy = true;
		formError = null;
		try {
			detail = await profiles.setAvatar(detail.id, file);
		} catch (error) {
			formError = message(error?.code);
		} finally {
			busy = false;
			if (fileInput) fileInput.value = '';
		}
	}

	async function confirmDelete() {
		busy = true;
		formError = null;
		try {
			await profiles.remove(detail.id);
			detail = null;
			confirmingDelete = false;
			view = profiles.activeID === null ? 'choosing' : 'managing';
		} catch (error) {
			formError = message(error?.code);
		} finally {
			busy = false;
		}
	}

	function formatDate(value) {
		if (!value) return t.profiles.never;
		return new Date(value).toLocaleDateString(i18n.localeTag, {
			year: 'numeric',
			month: 'long',
			day: 'numeric'
		});
	}
</script>

<svelte:head>
	<title>{t.profiles.question} — {t.appName}</title>
</svelte:head>

{#if view === 'loading'}
	<LoadingSkeleton variant="detail" label={t.profiles.loading} />
{:else if view === 'failed'}
	<div class="profile-screen">
		<div class="chrome-panel max-w-xl p-8 text-center sm:p-12">
			<p class="tv-copy border-l border-error py-2 pl-6 text-left">{message(failureCode)}</p>
			<a href="/" class="tv-link label mt-8 justify-center">← {t.nav.back}</a>
		</div>
	</div>
{:else if view === 'detail' && detail}
	<!-- Two stacked panels, as in the reference: identity above, local facts
	     below, the destructive action isolated at the foot. What the reference
	     also carried -- email, role, status, a subscription badge, a logout --
	     does not exist here and is not invented (decision 48). -->
	<div class="profile-screen profile-screen--page">
		<div class="profile-detail">
			<section class="chrome-panel profile-identity">
				<ProfileMark profile={detail} round size="7rem" />
				<div class="min-w-0 flex-1">
					<form class="profile-name-form" onsubmit={submitRename}>
						<label class="label" for="profile-name">{t.profiles.nameLabel}</label>
						<div class="profile-name-row">
							<input
								id="profile-name"
								class="profile-input"
								bind:value={draftName}
								maxlength="40"
								placeholder={t.profiles.namePlaceholder}
								aria-label={t.profiles.edit(nameOf(detail))}
							/>
							<button type="submit" class="tv-action cursor-pointer" disabled={busy}>
								{t.profiles.save}
							</button>
						</div>
					</form>

					<div class="profile-picture-actions">
						<!-- A visible button rather than a bare file input: the native
						     control is small, unlabelled and unreachable at three metres. -->
						<button
							type="button"
							class="tv-action cursor-pointer"
							onclick={() => fileInput?.click()}
							disabled={busy}
						>
							<Icon name="plus" size={16} />
							<span>{busy ? t.profiles.pictureUploading : t.profiles.pictureChange}</span>
						</button>
						<input
							bind:this={fileInput}
							type="file"
							accept="image/*"
							class="sr-only"
							onchange={uploadPicture}
							tabindex="-1"
							aria-hidden="true"
						/>
					</div>
					<p class="profile-hint">{t.profiles.pictureHint}</p>
				</div>
			</section>

			<section class="chrome-panel profile-facts">
				<h2 class="label">{t.profiles.details}</h2>
				<dl>
					<div><dt>{t.profiles.createdAt}</dt><dd>{formatDate(detail.created_at)}</dd></div>
					<div>
						<dt>{t.profiles.moviesStarted}</dt>
						<dd>{detail.stats?.movies_started ?? 0}</dd>
					</div>
					<div>
						<dt>{t.profiles.moviesFinished}</dt>
						<dd>{detail.stats?.movies_finished ?? 0}</dd>
					</div>
					<div>
						<dt>{t.profiles.episodesStarted}</dt>
						<dd>{detail.stats?.episodes_started ?? 0}</dd>
					</div>
					<div>
						<dt>{t.profiles.episodesFinished}</dt>
						<dd>{detail.stats?.episodes_finished ?? 0}</dd>
					</div>
					<div>
						<dt>{t.profiles.lastWatched}</dt>
						<dd>{formatDate(detail.stats?.last_watched_at)}</dd>
					</div>
				</dl>
			</section>

			{#if formError}
				<p class="profile-error" role="status">{formError}</p>
			{/if}

			{#if confirmingDelete}
				<div class="chrome-panel profile-danger">
					<p class="tv-copy">{t.profiles.deleteConfirm(nameOf(detail))}</p>
					<div class="profile-danger-actions">
						<button
							type="button"
							class="tv-action profile-destructive cursor-pointer"
							onclick={confirmDelete}
							disabled={busy}
						>
							{t.profiles.deleteYes}
						</button>
						<button
							type="button"
							class="tv-action cursor-pointer"
							onclick={() => (confirmingDelete = false)}
						>
							{t.profiles.cancel}
						</button>
					</div>
				</div>
			{:else}
				<button
					type="button"
					class="tv-action profile-destructive profile-delete cursor-pointer"
					onclick={() => (confirmingDelete = true)}
				>
					<Icon name="close" size={16} />
					<span>{t.profiles.delete}</span>
				</button>
			{/if}

			<button
				type="button"
				class="tv-link label mt-4"
				onclick={() => {
					detail = null;
					formError = null;
					view = 'managing';
				}}
			>
				← {t.nav.back}
			</button>
		</div>
	</div>
{:else}
	<!-- The chooser. Brand, the question, one row of cards, and a single detached
	     control underneath. Everything else is negative space. -->
	<div class="profile-screen">
		<a href="/" class="profile-brand" aria-label="{t.appName} — {t.nav.home}">
			<span class="brand-wordmark"><span class="brand-word">{t.appName}</span></span>
		</a>

		<h1 class="hero-title profile-question">
			{view === 'managing' ? t.profiles.manage : t.profiles.question}
		</h1>

		{#if list.length === 0}
			<p class="tv-copy">{t.profiles.empty}</p>
		{/if}

		<ul class="profile-row">
			{#each list as profile (profile.id)}
				<li>
					<button
						type="button"
						class="profile-card"
						class:profile-card--active={profile.id === profiles.activeID}
						aria-label={view === 'managing'
							? t.profiles.edit(nameOf(profile))
							: t.profiles.choose(nameOf(profile))}
						onclick={() => (view === 'managing' ? openDetail(profile) : choose(profile))}
					>
						<ProfileMark {profile} />
						<span class="profile-card-name">{nameOf(profile)}</span>
						{#if view === 'managing'}
							<span class="profile-card-edit label" aria-hidden="true">
								{t.profiles.editShort}
							</span>
						{/if}
					</button>
				</li>
			{/each}

			{#if view === 'managing' && list.length < 8}
				<li>
					<button type="button" class="profile-card profile-card--add" onclick={() => (creating = true)}>
						<span class="profile-mark profile-mark--empty" aria-hidden="true">
							<Icon name="plus" size={34} />
						</span>
						<span class="profile-card-name">{t.profiles.add}</span>
					</button>
				</li>
			{/if}
		</ul>

		{#if creating}
			<form class="chrome-panel profile-create" onsubmit={submitCreate}>
				<label class="label" for="new-profile">{t.profiles.nameLabel}</label>
				<div class="profile-name-row">
					<!-- svelte-ignore a11y_autofocus -->
					<input
						id="new-profile"
						class="profile-input"
						bind:value={draftName}
						maxlength="40"
						placeholder={t.profiles.namePlaceholder}
						autofocus
					/>
					<button type="submit" class="tv-action tv-action--primary cursor-pointer" disabled={busy}>
						{t.profiles.create}
					</button>
					<button type="button" class="tv-action cursor-pointer" onclick={() => (creating = false)}>
						{t.profiles.cancel}
					</button>
				</div>
			</form>
		{/if}

		{#if formError}
			<p class="profile-error" role="status">{formError}</p>
		{/if}

		<div class="profile-screen-actions">
			<button
				type="button"
				class="tv-action cursor-pointer"
				onclick={() => {
					creating = false;
					formError = null;
					view = view === 'managing' ? 'choosing' : 'managing';
				}}
			>
				{view === 'managing' ? t.profiles.done : t.profiles.manage}
			</button>

			{#if canLeave}
				<a href="/" class="tv-link label">← {t.nav.back}</a>
			{/if}
		</div>
	</div>
{/if}
