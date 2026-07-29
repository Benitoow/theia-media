<script>
	// Every path the Go server does not recognise is answered with index.html,
	// so the client router is what decides an address is wrong -- and this is
	// where it says so. Without this file the answer was SvelteKit's built-in
	// error page: unstyled, and in English in the middle of a French interface.
	import { page } from '$app/stores';
	import { strings as t } from '$lib/strings.js';
	import ChromeScene from '$lib/components/ChromeScene.svelte';

	const missing = $derived($page.status === 404);

	// No photograph here, unlike the other full-bleed screens. Someone on this
	// page is trying to work out what went wrong, and scenery is in the way of
	// that. ChromeScene already treats the image as optional.
</script>

<svelte:head>
	<title>{missing ? t.notFound.eyebrow : t.notFound.crash} — {t.appName}</title>
</svelte:head>

<ChromeScene
	eyebrow={missing ? t.notFound.eyebrow : t.notFound.crash}
	title={missing ? t.notFound.title : t.notFound.crash}
	body={missing ? t.notFound.body : t.notFound.crashBody}
	tone={missing ? 'neutral' : 'error'}
>
	<div class="flex flex-wrap items-center gap-4">
		<a href="/" class="tv-action tv-action--primary" data-remote-default>
			{t.notFound.home}
		</a>
		{#if !missing}
			<button type="button" onclick={() => location.reload()} class="tv-action cursor-pointer">
				{t.notFound.reload}
			</button>
		{/if}
	</div>
</ChromeScene>
