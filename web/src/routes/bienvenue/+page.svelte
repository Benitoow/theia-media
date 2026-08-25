<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getJSON } from '$lib/api.js';
	import { strings as t } from '$lib/strings.js';
	import ConnectPanel from '$lib/components/ConnectPanel.svelte';

	let info = $state(null);
	let failure = $state(null);

	onMount(async () => {
		try {
			info = await getJSON('/api/onboarding');
		} catch {
			// No network address means nothing to show and nothing to fix from
			// here. Let the library through rather than trapping the user on a
			// welcome screen.
			goto('/', { replaceState: true });
		}
	});

	async function enter() {
		try {
			await fetch('/api/onboarding/complete', { method: 'POST' });
		} catch {
			// Worst case the screen appears once more. Not worth blocking on.
		}
		goto('/', { replaceState: true });
	}
</script>

<svelte:head>
	<title>{t.welcome.title} — {t.appName}</title>
</svelte:head>

<main class="relative isolate min-h-screen overflow-hidden">
	<!-- Centred on purpose, and the exception to the rule the other backdrops
	     follow. This asset is 1920x1080, so on a 16/9 window it fits exactly and
	     the framing decides nothing at all; only a wider window crops it. Rendered
	     at 21/9 both ways: centred, the winged figure is cropped above the frame,
	     which reads as framing. Framed from the top she appears -- with her face
	     landing inside the navigation bar, which reads as a fault. Top alignment
	     is already the furthest the picture can move down, so there is no third
	     option. A head out of shot beats a head cut in half by a bar. -->
	<img
		src="/chrome/theia-onboarding.webp"
		alt=""
		class="absolute inset-0 -z-20 h-full w-full object-cover object-center opacity-55"
	/>
	<div class="picture-veil picture-veil--welcome"></div>

	{#if info}
		<div class="page-shell grid min-h-screen items-center gap-10 py-32 xl:grid-cols-[0.8fr_1.2fr]">
			<header class="max-w-xl">
				<span class="label">{t.welcome.eyebrow}</span>
				<h1 class="hero-title mt-5 mb-7">{t.welcome.title}</h1>
				<p class="tv-copy max-w-prose">{t.welcome.body}</p>
			</header>

			<section class="chrome-panel p-6 sm:p-10 xl:p-12">
				<ConnectPanel {info} size="large" />
				<div class="mt-9 border-t border-line pt-8">
					<button
						type="button"
						onclick={enter}
						class="tv-action tv-action--primary cursor-pointer"
						data-remote-default
					>
						<span>{t.welcome.enter}</span>
						<span aria-hidden="true">→</span>
					</button>
				</div>
			</section>
		</div>
	{:else if failure}
		<div class="page-shell flex min-h-screen items-center py-32">
			<p class="chrome-panel max-w-prose border-l border-error p-8 text-small text-parchment">
				{failure}
			</p>
		</div>
	{/if}
</main>
