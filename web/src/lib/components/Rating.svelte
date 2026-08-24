<script>
	// The film's rating, with the denominator that makes it a score.
	//
	// It used to be a bare gold number at the end of the metadata row, beside a
	// year and a runtime -- so "7,8" had nothing to say it was out of ten, and
	// sat in the one row on the page where every other figure is a duration or a
	// date. The scale is the fix, and it is deliberately the quiet half: the gold
	// stays on the figure alone, which keeps this to one of the five accent uses
	// section 3 allows per screen.
	import { strings as t, formatDecimal } from '$lib/strings.js';

	let { value = 0 } = $props();
</script>

{#if value}
	<span class="rating" aria-label={t.library.ratingAria(value)}>
		<span class="rating-value">{formatDecimal(value)}</span>
		<!-- Hidden from the accessible name because the label above already says
		     "out of ten" in words; announcing "7,8 slash 10" after it is noise. -->
		<span class="rating-scale" aria-hidden="true">{t.library.ratingScale}</span>
	</span>
{/if}
