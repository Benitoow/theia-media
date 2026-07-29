// Compatibility facade for the existing component imports. The live
// catalogue itself lives in i18n/ and changes reactively with the browser's
// selected locale.
export {
	strings,
	formatDecimal,
	formatUptime,
	formatSize,
	formatRuntime
} from '$lib/i18n/index.svelte.js';
