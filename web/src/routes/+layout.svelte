<script>
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { i18n } from '$lib/i18n/index.svelte.js';
	import { strings as t } from '$lib/strings.js';
	import { profiles } from '$lib/profiles.svelte.js';
	import { remote } from '$lib/remote.svelte.js';
	import ProfileMark from '$lib/components/ProfileMark.svelte';

	let { children } = $props();

	// Which link is lit. `/film/<id>` deliberately lights nothing: it is reached
	// as readily from the home rows as from the library, and claiming one of
	// them would be a guess dressed as a fact.
	const path = $derived($page.url.pathname);
	const atHome = $derived(path === '/');
	const inLibrary = $derived(path === '/films' || path.startsWith('/films/'));
	const inSeries = $derived(
		path === '/series' || path.startsWith('/serie/') || path.startsWith('/episode/')
	);
	const inSettings = $derived(path === '/reglages' || path.startsWith('/reglages/'));
	// The chooser owns the whole viewport: the navigation is suppressed for that
	// route only, so the first arrow key lands on a profile rather than on a link.
	const inProfiles = $derived(path === '/profils' || path.startsWith('/profils/'));

	// The pill keeps a light scrim over arbitrary hero artwork, then strengthens
	// once the page moves so it never becomes a hard opaque band.
	let scrolled = $state(false);

	onMount(async () => {
		i18n.bootstrap();
		// Asked first: what follows depends on which side of the tunnel this is.
		await remote.load();
		try {
			await profiles.load();
			// The application opens by asking who is watching, whenever this
			// browser has no answer -- or has one that was deleted elsewhere.
			if (profiles.needsSelection && !inProfiles) goto('/profils');
		} catch {
			// A server without profiles still serves the library. The chooser is
			// not worth blocking a film over.
		}
	});

	$effect(() => {
		document.documentElement.lang = i18n.htmlLang;
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

{#if !inProfiles}
<div class="site-nav-wrap">
	<nav class="site-nav" data-scrolled={scrolled} aria-label={t.a11y.mainNavigation}>
		<a
			href="/"
			class="nav-target nav-brand"
			aria-label="{t.appName} — {t.nav.home}"
			aria-current={atHome ? 'page' : undefined}
		>
			<!-- Text, not an image: it carries the name to a screen reader
			     without an alt attribute, needs no request, and cannot reflow
			     the nav while it loads. See .brand-wordmark. -->
			<span class="brand-wordmark">{t.appName}</span>
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
				href="/series"
				class="nav-target nav-link label"
				aria-current={inSeries ? 'page' : undefined}
			>
				{t.series.title}
			</a>
			<!-- Settings, scanning, onboarding and the updater are LAN-only, and
			     the remote guard refuses them outright. A link that always 403s is
			     worse than no link (decision 44). -->
			{#if !remote.isRemote}
				<a
					href="/reglages"
					class="nav-target nav-link label"
					aria-current={inSettings ? 'page' : undefined}
				>
					{t.nav.settings}
				</a>
			{/if}

			<!-- A shortcut to the chooser, not a menu that expands here. The
			     reference showed a dropdown; decision 35 had already measured that
			     a profile control inside this pill is unreadable at three metres
			     and steals the first D-pad press. -->
			{#if profiles.active}
				<a
					href="/profils"
					class="nav-target nav-profile"
					aria-label={t.profiles.switch}
					title={t.profiles.current(profiles.active.name || t.profiles.defaultName)}
				>
					<ProfileMark profile={profiles.active} round size="2.25rem" />
				</a>
			{/if}
		</div>
	</nav>
</div>

{#if remote.isRemote}
	<p class="remote-banner">
		<span class="label">{t.remote.remoteBadge}</span>
		{#if remote.peer?.name}
			<span>{t.remote.remoteContext(remote.peer.name)}</span>
		{/if}
	</p>
{/if}
{/if}

{@render children()}

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
