<script>
	import '../app.css';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { i18n } from '$lib/i18n/index.svelte.js';
	import { strings as t } from '$lib/strings.js';
	import { profileSession } from '$lib/profiles.svelte.js';

	let { children } = $props();

	// Which link is lit. `/film/<id>` deliberately lights nothing: it is reached
	// as readily from the home rows as from the library, and claiming one of
	// them would be a guess dressed as a fact.
	const path = $derived($page.url.pathname);
	const atHome = $derived(path === '/');
	const inLibrary = $derived(path === '/films' || path.startsWith('/films/'));
	const inSettings = $derived(path === '/reglages' || path.startsWith('/reglages/'));
	const inProfiles = $derived(path === '/profils' || path.startsWith('/profils/'));

	// The pill keeps a light scrim over arbitrary hero artwork, then strengthens
	// once the page moves so it never becomes a hard opaque band.
	let scrolled = $state(false);

	onMount(async () => {
		i18n.bootstrap();
		await profileSession.bootstrap();
	});

	$effect(() => {
		document.documentElement.lang = i18n.htmlLang;
	});

	// The layout survives client-side navigation. Keep this guard reactive so
	// leaving the chooser without selecting somebody cannot strand another page
	// behind an eternal loading message.
	$effect(() => {
		if (!profileSession.needsSelection || inProfiles) return;
		const destination = `/profils?return=${encodeURIComponent(
			$page.url.pathname + $page.url.search
		)}`;
		goto(destination, { replaceState: true });
	});

	const remoteFocusable = [
		'a[href]',
		'button:not(:disabled)',
		'input:not(:disabled)',
		'select:not(:disabled)',
		'textarea:not(:disabled)',
		'summary',
		'[role="slider"]',
		'[tabindex]:not([tabindex="-1"])'
	].join(',');

	function isRemoteFocusable(element) {
		if (!(element instanceof HTMLElement) || element.tabIndex < 0) return false;
		if (element.closest('[inert]') || element.getAttribute('aria-hidden') === 'true') return false;

		const style = getComputedStyle(element);
		const rect = element.getBoundingClientRect();
		return style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0;
	}

	function focusForRemote(element) {
		element.focus({ preventScroll: true });
		element.scrollIntoView({
			block: 'nearest',
			inline: 'nearest',
			behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
		});
	}

	// Browsers do not agree on spatial navigation, and desktop engines generally
	// do nothing with a TV remote's D-pad. Supply the missing contract here:
	// the first arrow enters the page, then geometry decides up/down/left/right.
	function navigateByRemote(event) {
		if (
			event.defaultPrevented ||
			event.altKey ||
			event.ctrlKey ||
			event.metaKey ||
			event.shiftKey ||
			!['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(event.key)
		) {
			return;
		}

		const active = document.activeElement;
		if (
			active instanceof HTMLInputElement ||
			active instanceof HTMLSelectElement ||
			active instanceof HTMLTextAreaElement ||
			active?.isContentEditable ||
			active?.getAttribute?.('role') === 'slider' ||
			active instanceof HTMLVideoElement
		) {
			return;
		}

		const modal = document.querySelector('[role="dialog"][aria-modal="true"]');
		const scope = modal ?? document;
		const candidates = [...scope.querySelectorAll(remoteFocusable)].filter(isRemoteFocusable);
		if (!candidates.length) return;

		if (!(active instanceof HTMLElement) || !candidates.includes(active)) {
			const preferred = scope.querySelector('[data-remote-default]');
			const target = isRemoteFocusable(preferred) ? preferred : candidates[0];
			event.preventDefault();
			focusForRemote(target);
			return;
		}

		const origin = active.getBoundingClientRect();
		const originX = origin.left + origin.width / 2;
		const originY = origin.top + origin.height / 2;

		const ranked = candidates
			.filter((candidate) => candidate !== active)
			.map((candidate) => {
				const rect = candidate.getBoundingClientRect();
				const dx = rect.left + rect.width / 2 - originX;
				const dy = rect.top + rect.height / 2 - originY;
				const horizontal = event.key === 'ArrowLeft' || event.key === 'ArrowRight';
				const primary = horizontal ? Math.abs(dx) : Math.abs(dy);
				const cross = horizontal ? Math.abs(dy) : Math.abs(dx);
				const aligned = horizontal
					? rect.bottom >= origin.top && rect.top <= origin.bottom
					: rect.right >= origin.left && rect.left <= origin.right;
				const ahead =
					(event.key === 'ArrowRight' && dx > 8) ||
					(event.key === 'ArrowLeft' && dx < -8) ||
					(event.key === 'ArrowDown' && dy > 8) ||
					(event.key === 'ArrowUp' && dy < -8);

				return { candidate, ahead, aligned, score: primary + cross * 2.25 };
			})
			.filter(({ ahead }) => ahead)
			.sort((a, b) => Number(b.aligned) - Number(a.aligned) || a.score - b.score);

		if (!ranked.length) return;
		event.preventDefault();
		focusForRemote(ranked[0].candidate);
	}
</script>

<svelte:window
	onscroll={() => (scrolled = window.scrollY > 24)}
	onkeydown={navigateByRemote}
/>

{#if !profileSession.ready}
	<main class="profile-bootstrap" aria-live="polite">
		<span class="label">{t.profiles.loading}</span>
	</main>
{:else}
	<!--
		No nav on the profile chooser. "Who is watching?" is a question with one
		job, and a D-pad landing on a row of navigation links instead of on a face
		is the whole reason the old pill did not work. Every other screen keeps it.
	-->
	{#if !inProfiles}
		<div class="site-nav-wrap">
			<nav class="site-nav" data-scrolled={scrolled} aria-label={t.a11y.mainNavigation}>
				<a
					href="/"
					class="nav-target nav-brand"
					aria-label={t.nav.home}
					aria-current={atHome ? 'page' : undefined}
				>
					<span class="brand-orbit" aria-hidden="true"></span>
					<span class="text-label font-semibold tracking-[0.2em] uppercase">{t.appName}</span>
				</a>
				<div class="flex items-center">
					<a
						href="/films"
						class="nav-target nav-link label"
						aria-current={inLibrary ? 'page' : undefined}
					>
						{t.nav.library}
					</a>
					<a
						href="/reglages"
						class="nav-target nav-link label"
						aria-current={inSettings ? 'page' : undefined}
					>
						{t.nav.settings}
					</a>
				</div>
			</nav>
		</div>
	{/if}

	{#if profileSession.active || profileSession.unreachable || inProfiles}
		{@render children()}
	{:else}
		<main class="profile-bootstrap" aria-live="polite">
			<span class="label">{t.profiles.loading}</span>
		</main>
	{/if}

	<!--
		TMDB's terms require this to be visible in the application. Keeping it in the
		layout means it survives every page added from here on without anyone having
		to remember it.
	-->
	<footer class="mt-16 border-t border-line py-10">
		<div class="page-shell">
			<p class="micro max-w-prose leading-relaxed text-muted">
				{t.tmdbAttribution}
			</p>
		</div>
	</footer>
{/if}

<style>
	.profile-bootstrap {
		display: grid;
		min-height: 100vh;
		place-items: center;
		color: var(--color-muted);
	}

	/* The profile pill that used to live here is gone: it is a full screen now,
	   reached from settings or shown on arrival. Its small-screen rule went with
	   it, and so did the 320px overflow it caused -- three nav targets fit where
	   four did not. */
</style>
