<script>
	import { strings as t } from '$lib/strings.js';
	import { i18n } from '$lib/i18n/index.svelte.js';

	// value is the certificate exactly as its board wrote it -- "12", "R", "TP".
	// country is the ISO code of that board. The server sends both or neither.
	let { value = '', country = '' } = $props();

	// The country is named by Intl rather than by a list in the catalogues: a
	// table of two hundred country names in two languages, to print one of them,
	// is a maintenance burden with no upside. It follows the active locale, so
	// switching language renames it without a reload.
	const countryName = $derived(regionName(country, i18n.localeTag));

	const accessibleName = $derived(
		countryName
			? t.credits.certificationFull(value, countryName)
			: `${t.credits.certification} ${value}`
	);

	function regionName(code, tag) {
		if (!code) return '';
		try {
			return new Intl.DisplayNames([tag], { type: 'region' }).of(code) ?? code;
		} catch {
			// Some television engines ship no Intl.DisplayNames. The code itself
			// is still true, just terser.
			return code;
		}
	}
</script>

{#if value}
	<!-- A boxed number beside the year and the runtime is the convention every
	     poster and every listings page already uses, so it does not need a word
	     in front of it. The word is in the accessible name, where a screen reader
	     needs it and a sofa does not. -->
	<span class="certificate" aria-label={accessibleName}>{value}</span>
{/if}
