<script>
	// rows is [{ label, value }], built by the page: which credits exist differs
	// between a film and a series, and the page is the only thing that knows.
	// Empty values are dropped here rather than at every call site.
	let { rows = [], heading } = $props();

	const shown = $derived(rows.filter((row) => row.value));
</script>

{#if shown.length}
	<!--
		A short table of facts, in the label register for the roles and the reading
		face for the names. It sits under the synopsis and above the cast, which is
		the order a film's own credits run in.
	-->
	<section class="credits">
		<h2 class="label mb-4">{heading}</h2>
		<dl class="credits-list">
			{#each shown as row (row.label)}
				<div class="credits-row">
					<dt class="label credits-role">{row.label}</dt>
					<dd class="credits-value">{row.value}</dd>
				</div>
			{/each}
		</dl>
	</section>
{/if}
