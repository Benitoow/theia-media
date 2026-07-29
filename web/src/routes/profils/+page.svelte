<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { strings as t } from '$lib/strings.js';
	import { profileSession } from '$lib/profiles.svelte.js';
	import ProfileAvatar from '$lib/components/ProfileAvatar.svelte';

	let managing = $state(false);
	let newName = $state('');
	let busyID = $state(null);
	let confirmingDelete = $state(null);
	let noticeCode = $state(null);
	let draftNames = $state({});

	const active = $derived(profileSession.active);
	const returnTo = $derived(safeReturn($page.url.searchParams.get('return')));

	onMount(async () => {
		if (!profileSession.ready) {
			await profileSession.bootstrap();
		} else {
			try {
				await profileSession.refresh();
			} catch {
				// Keep the last known list visible. Selecting another profile
				// will fail honestly if the server is still unreachable.
			}
		}
		syncDrafts();
	});

	function safeReturn(value) {
		if (!value || !value.startsWith('/') || value.startsWith('//') || value.startsWith('/profils')) {
			return '/';
		}
		return value;
	}

	function displayName(profile) {
		return profile.name || t.profiles.defaultName;
	}

	function syncDrafts() {
		draftNames = Object.fromEntries(
			profileSession.profiles.map((profile) => [profile.id, profile.name ?? ''])
		);
	}

	function choose(profile) {
		if (!profileSession.select(profile.id)) return;
		goto(returnTo);
	}

	function chooseWithRemote(event, profile) {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		choose(profile);
	}

	async function createProfile(event) {
		event.preventDefault();
		noticeCode = null;
		busyID = 'new';
		try {
			const created = await profileSession.create(newName);
			newName = '';
			draftNames[created.id] = created.name ?? '';
		} catch (error) {
			noticeCode = error.code || 'profile_create_failed';
		} finally {
			busyID = null;
		}
	}

	async function rename(profile) {
		noticeCode = null;
		busyID = profile.id;
		try {
			const updated = await profileSession.rename(profile.id, draftNames[profile.id] ?? '');
			draftNames[profile.id] = updated.name ?? '';
			noticeCode = 'profile_saved';
		} catch (error) {
			noticeCode = error.code || 'profile_update_failed';
		} finally {
			busyID = null;
		}
	}

	async function upload(profile, event) {
		const file = event.currentTarget.files?.[0];
		if (!file) return;
		noticeCode = null;
		busyID = profile.id;
		try {
			await profileSession.uploadAvatar(profile.id, file);
			noticeCode = 'avatar_saved';
		} catch (error) {
			noticeCode = error.code || 'avatar_save_failed';
		} finally {
			event.currentTarget.value = '';
			busyID = null;
		}
	}

	async function removeAvatar(profile) {
		noticeCode = null;
		busyID = profile.id;
		try {
			await profileSession.removeAvatar(profile.id);
			noticeCode = 'avatar_removed';
		} catch (error) {
			noticeCode = error.code || 'avatar_delete_failed';
		} finally {
			busyID = null;
		}
	}

	async function removeProfile(profile) {
		if (confirmingDelete !== profile.id) {
			confirmingDelete = profile.id;
			return;
		}
		noticeCode = null;
		busyID = profile.id;
		try {
			await profileSession.remove(profile.id);
			delete draftNames[profile.id];
			confirmingDelete = null;
			noticeCode = 'profile_deleted';
		} catch (error) {
			noticeCode = error.code || 'profile_delete_failed';
		} finally {
			busyID = null;
		}
	}

	const notice = $derived(
		noticeCode ? (t.profiles.notices[noticeCode] ?? t.profiles.notices.unknown) : null
	);
</script>

<svelte:head>
	<title>{t.profiles.title} — {t.appName}</title>
</svelte:head>

<main class="profiles-page page-shell page-body">
	<header class="profiles-heading enter">
		<span class="label text-accent">{t.profiles.eyebrow}</span>
		<h1 class="page-title mt-4">{t.profiles.title}</h1>
		<p class="tv-copy mt-5 max-w-prose text-muted">{t.profiles.body}</p>
	</header>

	{#if profileSession.ready && profileSession.profiles.length}
		<section class="profile-grid mt-12" aria-label={t.profiles.listLabel}>
			{#each profileSession.profiles as profile (profile.id)}
				<button
					type="button"
					class:profile-card--active={active?.id === profile.id}
					class="profile-card"
					onclick={() => choose(profile)}
					onkeydown={(event) => chooseWithRemote(event, profile)}
					data-remote-default={active?.id === profile.id || (!active && profile.is_default) ? true : undefined}
				>
					<ProfileAvatar {profile} large />
					<span class="profile-card-name">{displayName(profile)}</span>
					{#if active?.id === profile.id}
						<span class="profile-current">{t.profiles.current}</span>
					{/if}
				</button>
			{/each}
		</section>

		<div class="profiles-actions mt-10">
			<button
				type="button"
				class="tv-action cursor-pointer"
				onclick={() => {
					managing = !managing;
					confirmingDelete = null;
					noticeCode = null;
					syncDrafts();
				}}
			>
				{managing ? t.profiles.done : t.profiles.manage}
			</button>
		</div>

		{#if managing}
			<section class="profile-management mt-14 border-t border-line pt-12">
				<div class="management-copy">
					<h2 class="section-title">{t.profiles.manage}</h2>
					<p class="text-small mt-3 text-muted">{t.profiles.manageHint}</p>
				</div>

				<form class="new-profile mt-8" onsubmit={createProfile}>
					<label>
						<span class="label">{t.profiles.newName}</span>
						<input
							type="text"
							bind:value={newName}
							maxlength="40"
							autocomplete="off"
							placeholder={t.profiles.namePlaceholder}
							required
						/>
					</label>
					<button
						type="submit"
						class="tv-action tv-action--primary cursor-pointer"
						disabled={busyID === 'new'}
					>
						{busyID === 'new' ? t.profiles.adding : t.profiles.add}
					</button>
				</form>

				<div class="editor-list mt-10">
					{#each profileSession.profiles as profile (profile.id)}
						<article class="profile-editor">
							<div class="editor-avatar">
								<ProfileAvatar {profile} />
							</div>
							<label class="editor-name">
								<span class="label">{t.profiles.name}</span>
								<input
									type="text"
									bind:value={draftNames[profile.id]}
									maxlength="40"
									autocomplete="off"
									placeholder={profile.is_default
										? t.profiles.defaultName
										: t.profiles.namePlaceholder}
									required
								/>
							</label>
							<div class="editor-buttons">
								<button
									type="button"
									class="label"
									onclick={() => rename(profile)}
									disabled={busyID === profile.id}
								>
									{t.profiles.rename}
								</button>
								<label class="label upload-button">
									{profile.has_avatar ? t.profiles.replacePhoto : t.profiles.addPhoto}
									<input
										type="file"
										accept="image/jpeg,image/png,image/webp"
										onchange={(event) => upload(profile, event)}
										disabled={busyID === profile.id}
									/>
								</label>
								{#if profile.has_avatar}
									<button
										type="button"
										class="label"
										onclick={() => removeAvatar(profile)}
										disabled={busyID === profile.id}
									>
										{t.profiles.removePhoto}
									</button>
								{/if}
								{#if !profile.is_default}
									<button
										type="button"
										class="label delete-button"
										onclick={() => removeProfile(profile)}
										disabled={busyID === profile.id}
									>
										{confirmingDelete === profile.id
											? t.profiles.confirmDelete
											: t.profiles.delete}
									</button>
								{/if}
							</div>
						</article>
					{/each}
				</div>

				{#if notice}
					<p
						class:error-notice={!['profile_saved', 'avatar_saved', 'avatar_removed', 'profile_deleted'].includes(
							noticeCode
						)}
						class="profile-notice mt-7"
						role="status"
					>
						{notice}
					</p>
				{/if}
			</section>
		{/if}
	{/if}
</main>

<style>
	.profiles-page {
		max-width: var(--content-wide);
		min-height: 70vh;
	}

	.profiles-heading {
		max-width: 46rem;
	}

	.profile-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(9.5rem, 13rem));
		gap: clamp(1rem, 2vw, 2rem);
	}

	.profile-card {
		position: relative;
		min-height: 12rem;
		cursor: pointer;
		color: var(--color-bone);
		text-align: left;
	}

	.profile-card :global(.profile-avatar) {
		transition:
			border-color var(--duration-fast) var(--ease-cine),
			transform var(--duration-base) var(--ease-cine),
			box-shadow var(--duration-base) var(--ease-cine);
	}

	.profile-card:hover :global(.profile-avatar),
	.profile-card:focus-visible :global(.profile-avatar),
	.profile-card--active :global(.profile-avatar) {
		border-color: var(--color-accent);
		box-shadow: 0 1.2rem 3rem rgb(0 0 0 / 0.38);
		transform: translateY(-0.3rem);
	}

	.profile-card-name {
		display: block;
		margin-top: 0.9rem;
		overflow: hidden;
		font-size: clamp(1rem, 1.4vw, 1.25rem);
		font-weight: 500;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.profile-current {
		display: block;
		margin-top: 0.35rem;
		color: var(--color-muted);
		font-size: var(--text-label);
		letter-spacing: 0.16em;
		text-transform: uppercase;
	}

	.profiles-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 1rem;
	}

	.profile-management {
		max-width: 64rem;
	}

	.new-profile {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: end;
		gap: 1rem;
		max-width: 46rem;
	}

	input[type='text'] {
		display: block;
		width: 100%;
		min-height: 52px;
		margin-top: 0.55rem;
		border: 1px solid var(--color-line);
		background: var(--color-surface);
		padding: 0.7rem 1rem;
		color: var(--color-bone);
		font: inherit;
	}

	input[type='text']:focus-visible {
		border-color: var(--color-accent);
		outline: 2px solid var(--color-accent);
		outline-offset: 2px;
	}

	.editor-list {
		border-top: 1px solid var(--color-line);
	}

	.profile-editor {
		display: grid;
		grid-template-columns: auto minmax(12rem, 1fr) minmax(15rem, auto);
		align-items: center;
		gap: 1.25rem;
		padding: 1.25rem 0;
		border-bottom: 1px solid var(--color-line);
	}

	.editor-avatar :global(.profile-avatar) {
		width: 4rem;
		height: 4rem;
	}

	.editor-buttons {
		display: flex;
		flex-wrap: wrap;
		justify-content: flex-end;
		gap: 0.4rem 1.1rem;
	}

	.editor-buttons button,
	.upload-button {
		display: inline-flex;
		min-height: 44px;
		cursor: pointer;
		align-items: center;
		color: var(--color-muted);
		transition: color var(--duration-fast) var(--ease-cine);
	}

	.editor-buttons button:hover,
	.editor-buttons button:focus-visible,
	.upload-button:hover,
	.upload-button:focus-within {
		color: var(--color-bone);
	}

	.delete-button:hover,
	.delete-button:focus-visible {
		color: var(--color-error) !important;
	}

	.upload-button {
		position: relative;
	}

	.upload-button input {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		cursor: pointer;
		opacity: 0;
	}

	.profile-notice {
		border-left: 1px solid var(--color-accent);
		padding: 0.3rem 0 0.3rem 1rem;
		color: var(--color-parchment);
		font-size: var(--text-small);
	}

	.profile-notice.error-notice {
		border-color: var(--color-error);
	}

	@media (min-width: 80rem) {
		.profile-grid {
			grid-template-columns: repeat(auto-fit, minmax(12rem, 15rem));
		}
	}

	@media (max-width: 44rem) {
		.new-profile,
		.profile-editor {
			grid-template-columns: 1fr;
		}

		.editor-buttons {
			justify-content: flex-start;
		}
	}
</style>
