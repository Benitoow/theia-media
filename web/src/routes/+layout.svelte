<script>
	import '../app.css';
	import { strings as t } from '$lib/strings.js';

	let { children } = $props();

	// The bar is transparent over the hero and gains a background once the page
	// has moved, so it never sits as a hard band across the artwork.
	let scrolled = $state(false);
</script>

<svelte:window onscroll={() => (scrolled = window.scrollY > 24)} />

<nav
	class="ease-cine fixed inset-x-0 top-0 z-40 flex items-center justify-between
	       px-6 py-5 transition-colors duration-320 lg:px-16"
	class:bg-ink={scrolled}
	class:border-b={scrolled}
	class:border-line={scrolled}
>
	<a href="/" class="text-label tracking-[0.18em] uppercase">{t.appName}</a>
	<a
		href="/reglages"
		class="label ease-cine transition-colors duration-160 hover:text-bone"
	>
		{t.nav.settings}
	</a>
</nav>

{@render children()}

<!--
	TMDB's terms require this to be visible in the application. Keeping it in the
	layout means it survives every page added from here on without anyone having
	to remember it.
-->
<footer class="border-t border-line px-6 py-10 lg:px-16">
	<p class="micro leading-relaxed">
		This product uses the TMDB API but is not endorsed or certified by TMDB.
	</p>
</footer>
