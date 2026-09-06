<script>
	import { onMount } from 'svelte';
	import { getJSON, apiFetch } from '$lib/api.js';
	import { profiles } from '$lib/profiles.svelte.js';
	import { strings as t } from '$lib/strings.js';
	import Icon from './Icon.svelte';
	let { movieId } = $props();
	let saved = $state(false);
	let busy = $state(true);
	let failed = $state(false);
	onMount(async () => {
		try {
			await profiles.ready();
			const data = await getJSON(profiles.url('/api/library/watchlist'));
			saved = data.ids.includes(movieId);
		}
		catch { failed = true; }
		finally { busy = false; }
	});
	async function toggle() {
		busy = true; failed = false;
		try {
			const response = await apiFetch(profiles.url(`/api/library/movies/${movieId}/watchlist`), {
				method: saved ? 'DELETE' : 'PUT'
			});
			if (!response.ok) throw new Error('watchlist_save_failed');
			saved = !saved;
		}
		catch { failed = true; }
		finally { busy = false; }
	}
</script>
<div class="watchlist-control">
	<button type="button" class="tv-action" aria-pressed={saved} disabled={busy} onclick={toggle}>
		<Icon name={saved ? 'check' : 'plus'} size={18} />{saved ? t.v3.saved : t.v3.save}
	</button>
	{#if failed}<p class="text-small text-error" role="alert">{t.v3.listFailed}</p>{/if}
</div>
