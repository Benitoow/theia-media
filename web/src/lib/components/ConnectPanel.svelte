<script>
	import { strings as t } from '$lib/strings.js';

	// The welcome screen is the one a camera is actually pointed at, so it gets
	// a bigger symbol; the settings page only needs it legible.
	let { info, size = 'normal' } = $props();
	let copied = $state(false);

	async function copy() {
		try {
			await navigator.clipboard.writeText(info.primary_url);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			// Clipboard access needs a secure context, which plain HTTP on a LAN
			// is not. The address is written out in full right beside this, so
			// there is nothing to recover from.
		}
	}
</script>

<div class="flex flex-col items-center gap-8 lg:flex-row lg:items-center lg:gap-14">
	<!--
		The QR is served as SVG by the Go server, generated from an IP address
		this process discovered itself. Nothing user-supplied reaches it, which
		is what makes rendering it as markup acceptable here.
	-->
	<!--
		The code is capped by the screen, not only by the size asked for. It used
		to be a flat w-80 with shrink-0, which on the first-launch screen came to
		320px of code plus 32px of padding inside a 312px content box on a 360px
		phone: the address and the code -- the only two things that screen exists
		to show -- were cut off on the right. min() keeps the intended size
		wherever there is room for it and yields where there is not.
	-->
	<div
		class="shrink-0 rounded-xl bg-bone p-4 shadow-[0_1.5rem_4rem_rgba(0,0,0,0.32)]"
		class:w-[min(14rem,100%)]={size === 'normal'}
		class:w-[min(20rem,100%)]={size === 'large'}
	>
		<div class="[&>svg]:h-auto [&>svg]:w-full">{@html info.qr_code_svg}</div>
	</div>

	<div class="min-w-0 flex-1 text-center lg:text-left">
		<span class="label">{t.connect.address}</span>

		<!-- The plain URL matters as much as the code. Cameras fail, and this is
		     the line somebody types into a TV browser by hand. -->
		<p
			class="mt-4 mb-4 text-[clamp(1.125rem,1.7vw,1.75rem)] leading-tight font-semibold
			       break-all text-bone xl:whitespace-nowrap"
		>
			{info.primary_url}
		</p>

		<button
			type="button"
			onclick={copy}
			class="tv-action cursor-pointer"
		>
			{copied ? t.connect.copied : t.connect.copy}
		</button>

		{#if info.mdns_url}
			<p class="mt-7 text-base leading-relaxed text-muted">
				{t.connect.mdns}
				<span class="text-parchment">{info.mdns_url}</span>
			</p>
			<!-- Said plainly because it was measured on a real phone, not guessed:
			     Android does not resolve .local, so this can never be the only
			     way in. -->
			<p class="mt-3 text-sm leading-relaxed text-muted">{t.connect.mdnsCaveat}</p>
		{/if}

		{#if info.alternatives?.length}
			<details class="mt-6">
				<summary class="label inline-flex cursor-pointer items-center hover:text-bone">
					{t.connect.otherAddresses}
				</summary>
				<ul class="mt-3 space-y-2">
					{#each info.alternatives as alt (alt.url)}
						<li class="text-small">
							<span class="text-parchment">{alt.url}</span>
							<span class="label ml-2">
								{alt.interface}{#if alt.virtual} · {t.connect.virtual}{/if}
							</span>
						</li>
					{/each}
				</ul>
				<p class="mt-3 max-w-prose text-sm leading-relaxed text-muted">
					{t.connect.otherAddressesHint}
				</p>
			</details>
		{/if}
	</div>
</div>
