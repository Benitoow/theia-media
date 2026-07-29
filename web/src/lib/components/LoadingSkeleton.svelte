<script>
	let { variant = 'library', label } = $props();

	const rowPosters = Array.from({ length: 8 });
	const gridPosters = Array.from({ length: 18 });
</script>

<div class="page-skeleton page-skeleton--{variant}" role="status" aria-label={label}>
	<span class="sr-only">{label}</span>

	{#if variant === 'home'}
		<div class="skeleton-hero" aria-hidden="true">
			<div class="page-shell skeleton-hero-copy">
				<span class="skeleton-block skeleton-label"></span>
				<span class="skeleton-block skeleton-title"></span>
				<span class="skeleton-block skeleton-title skeleton-title--short"></span>
				<span class="skeleton-block skeleton-copy"></span>
				<span class="skeleton-block skeleton-copy skeleton-copy--short"></span>
				<span class="skeleton-block skeleton-action"></span>
			</div>
		</div>

		<div class="skeleton-home-rows" aria-hidden="true">
			{#each [0, 1] as row}
				<section class="skeleton-row">
					<div class="page-shell">
						<span class="skeleton-block skeleton-row-title"></span>
					</div>
					<div class="skeleton-poster-strip">
						{#each rowPosters as _}
							<div class="skeleton-card">
								<span class="skeleton-block skeleton-poster"></span>
								<span class="skeleton-block skeleton-card-title"></span>
								<span class="skeleton-block skeleton-card-legend"></span>
							</div>
						{/each}
					</div>
				</section>
			{/each}
		</div>
	{:else if variant === 'detail'}
		<div class="skeleton-detail-hero" aria-hidden="true"></div>
		<div class="page-shell skeleton-detail" aria-hidden="true">
			<div class="skeleton-detail-poster skeleton-block"></div>
			<div class="skeleton-detail-copy">
				<span class="skeleton-block skeleton-label"></span>
				<span class="skeleton-block skeleton-detail-title"></span>
				<span class="skeleton-block skeleton-detail-title skeleton-detail-title--short"></span>
				<span class="skeleton-block skeleton-metadata"></span>
				<span class="skeleton-block skeleton-action"></span>
				<span class="skeleton-block skeleton-copy"></span>
				<span class="skeleton-block skeleton-copy"></span>
				<span class="skeleton-block skeleton-copy skeleton-copy--short"></span>
			</div>
		</div>
	{:else}
		<div class="skeleton-library" aria-hidden="true">
			<span class="skeleton-block skeleton-library-title"></span>
			<span class="skeleton-block skeleton-label"></span>
			<span class="skeleton-block skeleton-toolbar"></span>
			<div class="skeleton-grid">
				{#each gridPosters as _}
					<div class="skeleton-card">
						<span class="skeleton-block skeleton-poster"></span>
						<span class="skeleton-block skeleton-card-title"></span>
						<span class="skeleton-block skeleton-card-legend"></span>
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>
