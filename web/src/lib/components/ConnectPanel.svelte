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

<div class="flex flex-col items-center gap-8 sm:flex-row sm:items-start sm:gap-12">
	<!--
		The QR is served as SVG by the Go server, generated from an IP address
		this process discovered itself. Nothing user-supplied reaches it, which
		is what makes rendering it as markup acceptable here.
	-->
	<div
		class="shrink-0 rounded-sm bg-bone p-4"
		class:w-52={size === 'normal'}
		class:w-72={size === 'large'}
	>
		{@html info.qr_code_svg}
	</div>

	<div class="min-w-0 flex-1 text-center sm:text-left">
		<span class="label">{t.connect.address}</span>

		<!-- The plain URL matters as much as the code. Cameras fail, and this is
		     the line somebody types into a TV browser by hand. -->
		<p class="mt-3 mb-1 font-display text-title break-all">{info.primary_url}</p>

		<button
			type="button"
			onclick={copy}
			class="ease-cine cursor-pointer text-label uppercase tracking-[0.18em] text-muted
			       transition-colors duration-160 hover:text-bone"
		>
			{copied ? t.connect.copied : t.connect.copy}
		</button>

		{#if info.mdns_url}
			<p class="mt-6 text-small text-muted">
				{t.connect.mdns}
				<span class="text-parchment">{info.mdns_url}</span>
			</p>
			<!-- Said plainly because it was measured on a real phone, not guessed:
			     Android does not resolve .local, so this can never be the only
			     way in. -->
			<p class="label mt-2">{t.connect.mdnsCaveat}</p>
		{/if}

		{#if info.alternatives?.length}
			<details class="mt-6">
				<summary class="label cursor-pointer hover:text-bone">
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
				<p class="label mt-3 max-w-prose normal-case tracking-normal">
					{t.connect.otherAddressesHint}
				</p>
			</details>
		{/if}
	</div>
</div>
