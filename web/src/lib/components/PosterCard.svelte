<script>
	import { imageURL, imageSrcSet, displayTitle, displayYear } from '$lib/api.js';
	import Icon from '$lib/components/Icon.svelte';

	// fluid lets the card fill a grid cell instead of carrying its own width.
	// Rows want a fixed size so a strip reads evenly; the library grid wants the
	// column to decide.
	// href lets the same card stand for a series: everything else about it --
	// artwork, title, legend, the progress rule -- is identical.
	// playable says whether the thing behind this card is watched by pressing it.
	// A film card leads to a page whose first control is Play, so the mark tells
	// the truth. A series card leads to a list of seasons, where nothing plays
	// until an episode is chosen, and a play triangle there would promise
	// something the click does not deliver.
	let {
		movie,
		fluid = false,
		legend = null,
		href = null,
		art = null,
		title: given = null,
		playable = true,
		// A card on the first screen is not "below the fold", and telling the
		// browser to defer it means the page paints its artwork late for no
		// reason. Every card was lazy, including the ones already visible.
		priority = false
	} = $props();

	// Landscape artwork, which is what a 16/9 card is for. TMDB's backdrop is
	// already that shape, so nothing is cropped to fit.
	//
	// art is a TMDB path rather than a finished URL, so that a caller which
	// supplies its own picture -- an episode still, on the home page -- gets the
	// same choice of sizes as everything else here.
	const backdropPath = $derived(art ?? movie.metadata?.backdrop_path);
	const backdrop = $derived(imageURL(backdropPath, 'w780'));
	const backdropSet = $derived(imageSrcSet(backdropPath));

	// What the card is actually drawn at, so the browser can pick a width
	// instead of always taking the largest. Two columns on a phone, then the
	// grid's own column, then the fixed row card at its 21rem ceiling. Slightly
	// generous at every step: a picture a little too large is invisible, and one
	// a little too small is soft on the biggest screen in the house.
	const cardSizes = '(max-width: 36rem) 47vw, (max-width: 64rem) 30vw, 21rem';

	// Only reached by a film that has a poster and no backdrop. On the 274-film
	// library that is nobody -- artwork arrives in pairs or not at all -- but a
	// portrait poster stretched to fill a landscape box would lose its title to
	// the crop, so it is contained rather than covered when it does happen.
	const poster = $derived(backdrop ? null : imageURL(movie.metadata?.poster_path, 'w342'));

	const title = $derived(given ?? displayTitle(movie));
	const year = $derived(displayYear(movie));
	// An em dash under every unmatched film read as a placeholder somebody had
	// forgotten to fill in. The line still exists so the cards keep one height;
	// it simply says nothing when there is nothing to say.
	const secondary = $derived(legend ?? year ?? '');
	// First letter of what the card is called, for the artwork stand-in.
	const initial = $derived((title.trim()[0] ?? '?').toUpperCase());

	// Only drawn for a film actually part-watched. A bar at zero on every card
	// would be noise, and one on a finished film would be a lie.
	const watched = $derived(
		movie.progress?.position_seconds > 0 &&
			!movie.progress?.finished &&
			movie.progress?.duration_seconds > 0
	);
	const percent = $derived(
		watched
			? Math.min(100, (movie.progress.position_seconds / movie.progress.duration_seconds) * 100)
			: 0
	);
</script>

<!--
	Wide landscape cards, per §6 of the design system as rewritten. The card is
	still restrained -- no display serif, no gold at rest beyond the progress
	rule -- but it is no longer a bare 2:3 poster: it shows the film's own
	backdrop at 16/9, with corners from the same family as every other panel.
-->
<a
	href={href ?? `/film/${movie.id}`}
	class="poster-card group"
	class:poster-card--fluid={fluid}
	title={title}
	data-poster-card
>
	<div class="poster-frame">
		{#if backdrop}
			<img
				src={backdrop}
				srcset={backdropSet}
				sizes={cardSizes}
				alt=""
				loading={priority ? 'eager' : 'lazy'}
				fetchpriority={priority ? 'high' : 'auto'}
				decoding="async"
				class="poster-art"
			/>
		{:else if poster}
			<img
				src={poster}
				alt=""
				loading={priority ? 'eager' : 'lazy'}
				fetchpriority={priority ? 'high' : 'auto'}
				decoding="async"
				class="poster-art poster-art--fit"
			/>
		{:else}
			<!-- No artwork is a normal state: TMDB did not recognise the file, or
			     has none of its own.
			     This used to print the title inside the frame, which meant every
			     unmatched film showed its name twice -- grey in the box, then
			     again in white directly underneath. The frame's job is to stand
			     in for the artwork, and the title below it was already doing the
			     reading. So it holds a quiet initial instead: decorative, hidden
			     from assistive technology, and different from film to film, so a
			     row of unmatched files does not read as a row of identical empty
			     boxes. -->
			<div class="poster-fallback" aria-hidden="true">
				<span class="poster-fallback-initial">{initial}</span>
			</div>
		{/if}

		<!-- What the card is for.
		     A grid of artwork with a hairline that turns gold says "this one is
		     selected". It never says "this one plays", which is the only thing
		     any of them do. The affordance appears on hover and on keyboard
		     focus, sits in bone rather than gold so the accent budget is
		     untouched, and is hidden from assistive technology because the link
		     around it already says where it goes. -->
		{#if playable}
			<div class="poster-hover" aria-hidden="true">
				<span class="poster-play"><Icon name="play" size={20} /></span>
			</div>
		{/if}

		{#if watched}
			<!-- The one accent the grid carries at rest, allowed by §6 because it
			     is information rather than decoration: it says how far in you are,
			     which is the whole point of the row it appears in. -->
			<div class="poster-progress">
				<div class="poster-progress-played" style="width: {percent}%"></div>
			</div>
		{/if}
	</div>

	<p class="poster-title">{title}</p>
	<p class="poster-legend label">{secondary}&nbsp;</p>
</a>
