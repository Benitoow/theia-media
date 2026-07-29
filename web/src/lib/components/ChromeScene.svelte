<script>
	// A full-bleed message screen: photograph, a line of eyebrow text, a display
	// title, a paragraph, and at most one action.
	//
	// There are three of these -- an empty library, an unreachable server, and
	// the welcome screen -- and they were drifting apart, each with its own
	// gradient stack and its own spacing. One component keeps them a family.
	//
	// This is chrome, so section 6 of the design system does not apply: it may
	// carry a picture and as much room as it likes. The poster grid may not.

	let {
		image = null,
		eyebrow = null,
		title,
		body = null,
		tone = 'neutral', // 'neutral' | 'error'
		children = null
	} = $props();
</script>

<section class="chrome-scene">
	{#if image}
		<img src={image} alt="" class="chrome-scene-image" />
		<!-- Two fades rather than one flat wash: the copy needs a floor on the
		     left, and the frame needs to meet the page colour at the bottom
		     without dimming the whole photograph. -->
		<div class="chrome-scene-veil"></div>
	{/if}

	<div class="page-shell relative w-full py-32 lg:py-40">
		<div class="max-w-2xl">
			{#if eyebrow}
				<span class="label" class:text-error={tone === 'error'}>{eyebrow}</span>
			{/if}

			<h1 class="hero-title mt-5 mb-7">{title}</h1>

			{#if body}
				<p class="tv-copy mb-9 max-w-prose" class:border-l={tone === 'error'}
					class:border-error={tone === 'error'} class:pl-6={tone === 'error'}>
					{body}
				</p>
			{/if}

			{@render children?.()}
		</div>
	</div>
</section>
