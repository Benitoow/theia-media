<script>
	import '../app.css';
	import { strings as t } from '$lib/strings.js';

	let { children } = $props();

	// The pill keeps a light scrim over arbitrary hero artwork, then strengthens
	// once the page moves so it never becomes a hard opaque band.
	let scrolled = $state(false);

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

<div class="site-nav-wrap">
	<nav class="site-nav" data-scrolled={scrolled} aria-label="Navigation principale">
		<a href="/" class="nav-target" aria-label={t.nav.home}>
			<span class="brand-orbit" aria-hidden="true"></span>
			<span class="text-label font-semibold tracking-[0.2em] uppercase">{t.appName}</span>
		</a>
		<a href="/reglages" class="nav-target label">
			{t.nav.settings}
		</a>
	</nav>
</div>

{@render children()}

<!--
	TMDB's terms require this to be visible in the application. Keeping it in the
	layout means it survives every page added from here on without anyone having
	to remember it.
-->
<footer class="mt-16 border-t border-line py-10">
	<div class="page-shell">
		<p class="micro max-w-prose leading-relaxed text-muted">
			This product uses the TMDB API but is not endorsed or certified by TMDB.
		</p>
	</div>
</footer>
