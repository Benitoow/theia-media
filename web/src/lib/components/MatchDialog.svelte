<!--
	Correcting what Theia thinks a file is.

	The library is built by searching TMDB for a title parsed out of a filename.
	That is right nearly every time and wrong in a way nobody could fix: the
	answer used to be "rename the file", which is not something anybody can do
	from the television they are holding the remote for.

	It is deliberately not a metadata editor. There are no fields to type into
	and nothing to correct by hand — only the list of records the automatic
	search passed over, in the order it ranked them, and one of them is right.
-->
<script>
	import { getJSON, imageURL } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import Icon from '$lib/components/Icon.svelte';

	let {
		/** "movies" or "series" — the two catalogues this can correct. */
		kind,
		id,
		/** What the item is currently called, so the heading names it. */
		title,
		onapplied,
		onclose
	} = $props();

	/** @type {'loading' | 'ready' | 'failed'} */
	let state = $state('loading');
	let candidates = $state([]);
	let query = $state('');
	let applyingId = $state(null);
	let errorCode = $state(null);

	const basePath = $derived(`/api/library/${kind}/${id}/match`);
	const wrongLabel = $derived(kind === 'series' ? t.match.wrongSeries : t.match.wrongFilm);

	$effect(() => {
		load('');
	});

	function messageFor(code) {
		return t.match.errors[code] ?? t.match.errors.unknown;
	}

	async function load(search) {
		state = 'loading';
		errorCode = null;
		try {
			const body = await getJSON(`${basePath}/candidates?q=${encodeURIComponent(search)}`);
			candidates = body.candidates ?? [];
			state = 'ready';
		} catch (error) {
			errorCode = error.code ?? 'unknown';
			state = 'failed';
		}
	}

	function submitSearch(event) {
		event.preventDefault();
		load(query.trim());
	}

	async function choose(candidate) {
		applyingId = candidate.tmdb_id;
		errorCode = null;
		try {
			const updated = await getJSON(basePath, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ tmdb_id: candidate.tmdb_id })
			});
			onapplied?.(updated);
		} catch (error) {
			errorCode = error.code ?? 'unknown';
		} finally {
			applyingId = null;
		}
	}

	async function revert() {
		applyingId = -1;
		errorCode = null;
		try {
			const res = await fetch(basePath, { method: 'DELETE' });
			if (!res.ok) throw new Error(String(res.status));
			onapplied?.(null);
		} catch {
			errorCode = 'unknown';
		} finally {
			applyingId = null;
		}
	}

	// Escape closes, as it does everywhere else a panel covers the page.
	function onKeydown(event) {
		if (event.key === 'Escape') {
			event.stopPropagation();
			onclose?.();
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

<div class="match-scrim" role="dialog" aria-modal="true" aria-label={wrongLabel}>
	<div class="match-panel chrome-panel">
		<header class="match-head">
			<div class="min-w-0">
				<h2 class="font-display text-title font-normal">{t.match.heading}</h2>
				<p class="mt-2 text-small text-muted">{title}</p>
			</div>
			<button type="button" class="tv-action" onclick={() => onclose?.()}>
				<Icon name="close" size={16} />
				<span>{t.match.close}</span>
			</button>
		</header>

		<p class="max-w-prose text-small text-muted">{t.match.intro}</p>

		<form class="match-search" onsubmit={submitSearch}>
			<label class="label" for="match-query">{t.match.searchLabel}</label>
			<div class="match-search-row">
				<input
					id="match-query"
					type="search"
					bind:value={query}
					placeholder={t.match.searchPlaceholder}
					class="min-w-0 flex-1 border border-line bg-surface px-4 py-2.5 text-small
					       text-bone placeholder:text-faint focus:border-muted focus:outline-none"
					data-remote-default
				/>
				<button type="submit" class="tv-action">
					<Icon name="search" size={16} />
					<span>{t.match.search}</span>
				</button>
			</div>
		</form>

		{#if errorCode}
			<p class="match-error text-small" role="alert">{messageFor(errorCode)}</p>
		{/if}

		{#if state === 'loading'}
			<p class="text-small text-muted">{t.match.searching}</p>
		{:else if state === 'ready' && candidates.length === 0}
			<p class="text-small text-muted">{t.match.none}</p>
		{:else if state === 'ready'}
			<ul class="match-list">
				{#each candidates as candidate (candidate.tmdb_id)}
					<li class="match-item">
						<div class="match-poster">
							{#if imageURL(candidate.poster_path, 'w154')}
								<img src={imageURL(candidate.poster_path, 'w154')} alt="" decoding="async" />
							{/if}
						</div>
						<div class="min-w-0 flex-1">
							<p class="match-title">
								{candidate.title}
								{#if candidate.year}<span class="text-muted"> · {candidate.year}</span>{/if}
							</p>
							{#if candidate.original_title && candidate.original_title !== candidate.title}
								<p class="text-small text-muted">{candidate.original_title}</p>
							{/if}
							<p class="match-overview text-small text-muted">
								{candidate.overview || t.match.noOverview}
							</p>
						</div>
						<button
							type="button"
							class="tv-action tv-action--primary shrink-0"
							disabled={applyingId !== null}
							onclick={() => choose(candidate)}
						>
							{applyingId === candidate.tmdb_id ? t.match.applying : t.match.choose}
						</button>
					</li>
				{/each}
			</ul>
		{/if}

		<footer class="match-foot">
			<p class="max-w-prose text-small text-muted">{t.match.pinnedHint}</p>
			<button type="button" class="tv-action" disabled={applyingId !== null} onclick={revert}>
				{t.match.revert}
			</button>
		</footer>
	</div>
</div>

<style>
	.match-scrim {
		position: fixed;
		inset: 0;
		z-index: 60;
		display: flex;
		align-items: flex-start;
		justify-content: center;
		overflow-y: auto;
		padding: clamp(1rem, 4vw, 3rem);
		background: rgba(11, 10, 9, 0.86);
	}

	.match-panel {
		width: min(56rem, 100%);
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
		padding: clamp(1.25rem, 3vw, 2rem);
	}

	.match-head {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1.5rem;
	}

	.match-search {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.match-search-row {
		display: flex;
		gap: 0.75rem;
	}

	.match-error {
		color: var(--color-accent);
	}

	.match-list {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.match-item {
		display: flex;
		align-items: center;
		gap: 1rem;
		padding: 0.75rem;
		border: 1px solid var(--color-line);
		border-radius: var(--radius-card);
		background: var(--color-surface);
	}

	.match-poster {
		width: 3.5rem;
		flex: 0 0 auto;
		aspect-ratio: 2 / 3;
		overflow: hidden;
		border-radius: calc(var(--radius-card) / 2);
		background: var(--color-ink);
	}

	.match-poster img {
		height: 100%;
		width: 100%;
		object-fit: cover;
	}

	.match-title {
		font-size: 1rem;
		line-height: 1.3;
	}

	/* Two lines of synopsis: enough to tell two films of the same name apart,
	   not enough to turn the list into reading. */
	.match-overview {
		display: -webkit-box;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		overflow: hidden;
		margin-top: 0.35rem;
	}

	.match-foot {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		border-top: 1px solid var(--color-line);
		padding-top: 1rem;
	}

	@media (max-width: 40rem) {
		.match-item {
			flex-wrap: wrap;
		}
	}
</style>
