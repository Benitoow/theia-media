<script>
	// One film, as the card grid draws it.
	//
	// Design system section 6 is the authority here, and it is the one section
	// that overrides the rest: artwork arrives with its own typography and
	// colour, so a card is density and speed rather than chrome. 6.1 fixes the
	// frame - a 16/9 backdrop at the width a card can show, `--radius-card`, a
	// 1px hairline at rest, gold on hover and focus, the title underneath in the
	// UI face rather than the display serif - and 6.2 the behaviour.
	//
	// The three fallbacks 6.1 orders are all drawn here: the backdrop, then the
	// poster *contained* in the frame rather than covering it (a portrait image
	// cropped to a landscape box loses its title off the top and bottom), then
	// the title as text on a surface. Never a broken-image icon - which is why a
	// dead source is a state here rather than something the browser decides. An
	// `<img>` that fails paints the platform's own glyph over the film, and a
	// server whose image cache has not caught up yet is a normal state, not a
	// fault.

	import Icon from '../../../../web/src/lib/components/Icon.svelte';

	/**
	 * @type {{
	 *   movie: Record<string, any>,
	 *   onplay: (id: number) => void,
	 *   t: (key: string) => string
	 * }}
	 */
	let { movie, onplay, t } = $props();

	// Which URL failed, rather than a boolean: a card whose artwork arrives
	// later - a rescan, a cache that filled in the meantime - then draws again
	// instead of keeping a verdict about a URL nobody is asking for any more.
	let failedUrl = $state('');

	const backdrop = $derived(
		movie.backdrop_url && movie.backdrop_url !== failedUrl ? movie.backdrop_url : ''
	);
	const poster = $derived(
		!backdrop && movie.poster_url && movie.poster_url !== failedUrl ? movie.poster_url : ''
	);
	const title = $derived(movie.title || '');

	const position = $derived(movie.progress?.position_seconds ?? 0);
	const duration = $derived(movie.progress?.duration_seconds ?? 0);
	const finished = $derived(movie.progress?.finished ?? false);

	// Part-watched only: never at zero, never on a finished film, and never
	// without a duration to divide by - the bar is the grid's one exemption from
	// 6.2's rules, and a bar invented from a missing duration would be a lie
	// about how much of the film is left.
	const watched = $derived(
		!finished && position > 0 && duration > 0
			? Math.min(100, (position / duration) * 100)
			: 0
	);

	// The same thirty seconds the server uses before it remembers a position at
	// all (`minimumRememberedSeconds`): offering to resume at 4 seconds is
	// offering to start again.
	const resumable = $derived(!finished && position >= 30);
</script>

<li>
	<button class="film" onclick={() => onplay(movie.id)}>
		<span class="film-art" class:film-art--empty={!backdrop && !poster}>
			{#if backdrop}
				<img src={backdrop} alt="" loading="lazy" decoding="async" onerror={() => (failedUrl = backdrop)} />
			{:else if poster}
				<img
					class="film-art--poster"
					src={poster}
					alt=""
					loading="lazy"
					decoding="async"
					onerror={() => (failedUrl = poster)}
				/>
			{:else}
				<span class="film-art-title">{title}</span>
			{/if}
			<!-- Bone, not gold: the accent still has to mean "look here"
			     everywhere else, exactly as it does for the player's one filled
			     control (6b). aria-hidden because the button around it already
			     says where it goes. -->
			<span class="film-mark" aria-hidden="true"><Icon name="play" size={28} /></span>
			{#if watched > 0}
				<span class="film-watched" style="width: {watched}%"></span>
			{/if}
		</span>
		<span class="film-name">{title}</span>
		<span class="label film-legend">
			{#if movie.year}{movie.year}{/if}
			{#if resumable}
				· {t('resumeAt')} {Math.max(1, Math.floor(position / 60))} min
			{/if}
		</span>
	</button>
</li>
