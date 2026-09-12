<script>
	import { browser } from '$app/environment';
	import { codecPlayback } from '$lib/codec-playback.svelte.js';
	import {
		compatibilityRows,
		probePlaybackEnvironment
	} from '$lib/playback-compatibility.js';
	import { strings as t } from '$lib/strings.js';

	let { media = null, plan = null, loading = false } = $props();
	let environment = $state({ hdrDisplay: null, codecSupport: 'unknown' });

	$effect(() => {
		const codec = plan?.video_codec || media?.video?.codec || '';
		if (browser) environment = probePlaybackEnvironment(codec, window);
	});

	const rows = $derived(
		compatibilityRows({
			media,
			plan,
			environment,
			browserStruggles: codecPlayback.strugglesWith(plan?.video_codec || media?.video?.codec),
			loading
		})
	);

	function detail(row) {
		const value = t.film.compatibility.details[row.detail];
		return typeof value === 'function' ? value(row.codec || '', row.kind || '') : value;
	}
</script>

<section class="compatibility" aria-labelledby="playback-compatibility-heading">
	<div class="compatibility-heading">
		<div>
			<h2 id="playback-compatibility-heading" class="label">{t.film.compatibility.heading}</h2>
			<p class="compatibility-intro">{t.film.compatibility.intro}</p>
		</div>
		<span class="compatibility-preview">{t.film.compatibility.preview}</span>
	</div>

	<ul class="compatibility-list">
		{#each rows as row (row.feature)}
			<li class="compatibility-row">
				<div class="compatibility-row-heading">
					<span class="compatibility-feature">{t.film.compatibility.features[row.feature]}</span>
					<span class="compatibility-status compatibility-status--{row.status}">
						{t.film.compatibility.statuses[row.status]}
					</span>
				</div>
				<p>{detail(row)}</p>
			</li>
		{/each}
	</ul>

	<p class="compatibility-caveat">{t.film.compatibility.caveat}</p>
</section>

<style>
	.compatibility {
		max-width: 46rem;
		margin-top: 2rem;
		border: 1px solid var(--color-line);
		border-radius: var(--radius-card);
		background:
			linear-gradient(135deg, color-mix(in srgb, var(--color-raised) 58%, transparent), transparent 52%),
			var(--color-surface);
		padding: clamp(1.1rem, 2vw, 1.5rem);
	}

	.compatibility-heading,
	.compatibility-row-heading {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
	}

	.compatibility-intro,
	.compatibility-row p,
	.compatibility-caveat {
		color: var(--color-muted);
		font-size: var(--text-small);
		line-height: 1.55;
	}

	.compatibility-intro {
		margin: 0.55rem 0 0;
	}

	.compatibility-preview,
	.compatibility-status {
		flex: none;
		border: 1px solid var(--color-line);
		border-radius: var(--radius-nested);
		padding: 0.28rem 0.5rem;
		color: var(--color-muted);
		font-family: var(--font-label);
		font-size: var(--text-label);
		letter-spacing: var(--text-label--letter-spacing);
		line-height: var(--text-label--line-height);
		text-transform: uppercase;
	}

	.compatibility-preview {
		color: var(--color-faint);
	}

	.compatibility-list {
		margin: 1.25rem 0 0;
		padding: 0;
		list-style: none;
	}

	.compatibility-row {
		border-top: 1px solid var(--color-line);
		padding: 1rem 0;
	}

	.compatibility-feature {
		color: var(--color-parchment);
		font-weight: 600;
	}

	.compatibility-row p {
		margin: 0.45rem 0 0;
	}

	.compatibility-status--adapted,
	.compatibility-status--reported,
	.compatibility-status--planned {
		border-color: var(--color-muted);
		color: var(--color-parchment);
	}

	.compatibility-status--measured,
	.compatibility-status--unavailable {
		border-color: color-mix(in srgb, var(--color-error) 58%, var(--color-line));
		color: var(--color-error);
	}

	.compatibility-caveat {
		margin: 0;
		border-top: 1px solid var(--color-line);
		padding-top: 1rem;
		color: var(--color-faint);
	}

	@media (max-width: 36rem) {
		.compatibility-heading,
		.compatibility-row-heading {
			flex-direction: column;
		}

		.compatibility-preview,
		.compatibility-status {
			align-self: flex-start;
		}
	}
</style>
