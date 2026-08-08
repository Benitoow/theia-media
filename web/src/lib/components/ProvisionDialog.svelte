<script>
	// The one time a device's private key exists in the interface.
	//
	// The server returns it once and cannot reproduce it (decision 45). So it
	// lives in this component's memory and nowhere else: not localStorage, not
	// IndexedDB, not a log, not a URL, and not a service worker's cache. Closing
	// clears it, and losing it means revoking the device rather than asking for
	// it again -- a server that could show it twice would not have thrown it away.
	import { onDestroy, tick } from 'svelte';
	import { strings as t } from '$lib/strings.js';
	import Icon from './Icon.svelte';

	let { provision, onclose } = $props();

	let copied = $state(false);
	let confirmingClose = $state(false);
	let kept = $state(false);
	/** @type {HTMLElement} */
	let panel = $state();

	const name = $derived(provision?.peer?.name ?? '');

	async function copyConfig() {
		try {
			await navigator.clipboard.writeText(provision.client_config);
			copied = true;
			kept = true;
			setTimeout(() => (copied = false), 2500);
		} catch {
			// A browser without clipboard permission still has the download.
		}
	}

	function downloadConfig() {
		// Built and revoked in the same breath: an object URL left alive is a copy
		// of the key sitting in the document.
		const blob = new Blob([provision.client_config], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.href = url;
		link.download = `theia-${name.replace(/[^\p{L}\p{N}]+/gu, '-').toLowerCase() || 'appareil'}.conf`;
		link.click();
		URL.revokeObjectURL(url);
		kept = true;
	}

	function attemptClose() {
		if (kept) return onclose();
		confirmingClose = true;
	}

	function onKeydown(event) {
		if (event.key === 'Escape') {
			event.preventDefault();
			attemptClose();
			return;
		}
		if (event.key !== 'Tab' || !panel) return;

		const focusable = [...panel.querySelectorAll('a[href], button:not(:disabled)')].filter(
			(element) => element.offsetParent !== null
		);
		if (!focusable.length) return;
		const first = focusable[0];
		const last = focusable[focusable.length - 1];
		if (event.shiftKey && document.activeElement === first) {
			event.preventDefault();
			last.focus();
		} else if (!event.shiftKey && document.activeElement === last) {
			event.preventDefault();
			first.focus();
		}
	}

	$effect(() => {
		if (!panel) return;
		tick().then(() => panel.querySelector('[data-remote-default]')?.focus());
	});

	onDestroy(() => {
		// Nothing here outlives the dialog.
		copied = false;
		kept = false;
	});
</script>

<svelte:window onkeydown={onKeydown} />

<div class="provision-backdrop" role="dialog" aria-modal="true" aria-label={t.remote.provisionTitle(name)}>
	<div bind:this={panel} class="chrome-panel provision-panel">
		<h2 class="page-title">{t.remote.provisionTitle(name)}</h2>

		<p class="provision-warning">
			<Icon name="info" size={18} />
			<span>{t.remote.provisionWarning}</span>
		</p>

		{#if confirmingClose}
			<p class="provision-warning provision-warning--strong">{t.remote.closeWarning}</p>
			<div class="provision-actions">
				<button type="button" class="tv-action cursor-pointer" onclick={() => (confirmingClose = false)}>
					{t.remote.cancel}
				</button>
				<button
					type="button"
					class="tv-action provision-destructive cursor-pointer"
					onclick={onclose}
				>
					{t.remote.closeAnyway}
				</button>
			</div>
		{:else}
			<!-- The QR comes from the local server as an SVG. It stays here: it is a
			     secret, not a thumbnail. -->
			<div class="provision-qr">{@html provision.qr_svg}</div>
			<p class="provision-scan">{t.remote.provisionScan}</p>

			<!--
				What to do with it, numbered.

				The dialog used to hand over a QR code, a tunnel address and three
				buttons, and never said that WireGuard has to be installed, nor
				which address to open once it is connected. The public endpoint on
				the settings page looks like a web address; somebody typed it into a
				browser and got a connection refused, which is exactly what a UDP
				endpoint does when asked for HTTP. `tunnel_url` was in the payload
				from the start and shown nowhere.
			-->
			<ol class="provision-steps">
				<li>{t.remote.step1}</li>
				<li>{t.remote.step2}</li>
				<li>
					{t.remote.step3}
					<span class="provision-url">{provision.tunnel_url}</span>
				</li>
			</ol>

			<dl class="provision-facts">
				<div>
					<dt>{t.remote.address}</dt>
					<dd>{provision.peer?.address}</dd>
				</div>
			</dl>

			<div class="provision-actions">
				<button type="button" class="tv-action cursor-pointer" onclick={copyConfig}>
					<Icon name={copied ? 'check' : 'plus'} size={16} />
					<span>{copied ? t.remote.copied : t.remote.copyConfig}</span>
				</button>
				<button type="button" class="tv-action cursor-pointer" onclick={downloadConfig}>
					{t.remote.downloadConfig}
				</button>
				<button
					type="button"
					class="tv-action tv-action--primary cursor-pointer"
					onclick={attemptClose}
					data-remote-default
				>
					{t.remote.done}
				</button>
			</div>

			<p class="provision-lost">{t.remote.lost}</p>
		{/if}
	</div>
</div>
