<script>
	// Remote access, administered from the LAN only.
	//
	// The whole point of M4 is that the tunnel is the *only* way in from outside
	// and that the historical HTTP port stays private.
	//
	// What this panel asks for has changed. It opened with a UDP port field, a
	// public endpoint field and a paragraph of router instructions -- four steps
	// in somebody else's admin interface, three of which Theia could have taken
	// itself. It asks the router now. The instructions have not been deleted,
	// only folded away: a box with UPnP switched off is a real evening, and it
	// is then the only way through. See internal/portmap.
	import { onMount } from 'svelte';
	import { apiFetch, getJSON, formatTime } from '$lib/api.js';
	import { strings as t, formatSize } from '$lib/strings.js';
	import { i18n } from '$lib/i18n/index.svelte.js';
	import Icon from './Icon.svelte';
	import ProvisionDialog from './ProvisionDialog.svelte';

	/** @type {'loading' | 'ready' | 'failed'} */
	let view = $state('loading');
	let status = $state(null);
	let busy = $state(false);
	let formError = $state(null);

	let portDraft = $state('');
	let endpointDraft = $state('');
	let deviceName = $state('');
	let adding = $state(false);
	let provision = $state(null);
	/** @type {number | null} */
	let revoking = $state(null);
	// The port and the address are the router's to answer, so the fields that
	// used to ask for them are folded away. They are still here, because a
	// router that refuses to forward is a real case and a person who has
	// forwarded the port themselves must not be told to undo it.
	let manualOpen = $state(false);

	const running = $derived(status?.state === 'running');
	const failed = $derived(status?.state === 'error');
	const automatic = $derived(status?.automatic !== false);
	const discoveryFailed = $derived(Boolean(status?.discovery_reason));
	const dirty = $derived(
		status !== null &&
			(String(status.listen_port) !== portDraft || (status.endpoint ?? '') !== endpointDraft)
	);
	const portChanged = $derived(status !== null && String(status.listen_port) !== portDraft);
	const endpointChanged = $derived(status !== null && (status.endpoint ?? '') !== endpointDraft);

	const message = (code) => t.remote.codes[code] ?? t.remote.codes.remote_access_unavailable;
	const reason = (code) => t.remote.reasons[code] ?? t.remote.codes.remote_access_unavailable;

	function adopt(next) {
		status = next;
		portDraft = String(next.listen_port ?? '');
		endpointDraft = next.endpoint ?? '';
		// A router that said no is exactly when the manual way is worth seeing.
		if (next.discovery_reason) manualOpen = true;
	}

	// One button. Everything the tunnel needs is asked of the router: which port
	// it will forward, and what address the internet sees. Both are facts the
	// gateway already holds, and UPnP and NAT-PMP are how it says them -- to
	// this machine, on this network, with nothing in between.
	const enable = () => send({ enabled: true, automatic: true });

	onMount(async () => {
		try {
			adopt(await getJSON('/api/remote-access'));
			view = 'ready';
		} catch {
			view = 'failed';
		}
	});

	async function send(body) {
		busy = true;
		formError = null;
		try {
			const response = await apiFetch('/api/remote-access', {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body)
			});
			if (!response.ok) {
				let code = 'remote_access_unavailable';
				try {
					const payload = await response.json();
					if (payload?.error) code = payload.error;
				} catch {
					// A failure without a body still has its status.
				}
				formError = message(code);
				return false;
			}
			adopt(await response.json());
			return true;
		} catch {
			formError = message('remote_access_unavailable');
			return false;
		} finally {
			busy = false;
		}
	}

	// Only what actually changed is sent, so saving an endpoint never restarts a
	// listener the viewer did not ask to restart.
	const saveConfig = () =>
		send({
			...(portChanged ? { listen_port: Number(portDraft) } : {}),
			...(endpointChanged ? { endpoint: endpointDraft.trim() } : {})
		});

	async function createDevice(event) {
		event.preventDefault();
		busy = true;
		formError = null;
		try {
			const response = await apiFetch('/api/remote-access/peers', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name: deviceName })
			});
			if (!response.ok) {
				let code = 'remote_access_unavailable';
				try {
					const payload = await response.json();
					if (payload?.error) code = payload.error;
				} catch {
					// Same as above.
				}
				formError = message(code);
				return;
			}
			provision = await response.json();
			deviceName = '';
			adding = false;
		} catch {
			formError = message('remote_access_unavailable');
		} finally {
			busy = false;
		}
	}

	async function revoke(id) {
		busy = true;
		formError = null;
		try {
			const response = await apiFetch(`/api/remote-access/peers/${id}`, { method: 'DELETE' });
			if (!response.ok && response.status !== 204) {
				let code = 'remote_access_unavailable';
				try {
					const payload = await response.json();
					if (payload?.error) code = payload.error;
				} catch {
					// Same as above.
				}
				formError = message(code);
				return;
			}
			revoking = null;
			adopt(await getJSON('/api/remote-access'));
		} catch {
			formError = message('remote_access_unavailable');
		} finally {
			busy = false;
		}
	}

	async function closeProvision() {
		provision = null;
		try {
			adopt(await getJSON('/api/remote-access'));
		} catch {
			// The device exists regardless; the list refreshes on the next load.
		}
	}

	function handshake(seconds) {
		if (!seconds) return t.remote.neverConnected;
		return new Date(seconds * 1000).toLocaleString(i18n.localeTag);
	}
</script>

<section class="mb-14 border-b border-line pb-14">
	<h2 class="label mb-5">{t.remote.heading}</h2>

	{#if view === 'loading'}
		<p class="label" role="status">{t.remote.saving}</p>
	{:else if view === 'failed'}
		<p class="remote-error">{message('remote_access_unavailable')}</p>
	{:else}
		<p class="tv-copy mb-6 max-w-[46rem]">{t.remote.intro}</p>

		<dl class="remote-status">
			<div>
				<dt>{t.remote.state}</dt>
				<dd class:remote-value--error={failed}>
					{failed
						? t.remote.stateError
						: running
							? t.remote.stateRunning
							: t.remote.stateDisabled}
				</dd>
			</div>
			{#if running}
				<div>
					<dt>{t.remote.reachability}</dt>
					<dd>{status.reachability === 'confirmed' ? t.remote.confirmed : t.remote.unverified}</dd>
				</div>
				{#if status.endpoint}
					<!--
						Shown rather than asked for: this is what the router answered.

						Labelled as the tunnel's endpoint and never as an address to
						open, because host:port in a browser-shaped interface reads as
						a link. Somebody typed it into Chrome and met
						ERR_CONNECTION_REFUSED, which is correct -- it is a WireGuard
						UDP endpoint and nothing listens there for HTTP -- and told
						them nothing. The address to actually open sits beside it.
					-->
					<div>
						<dt>{t.remote.publicAddress}</dt>
						<dd class="remote-value--plain">{status.endpoint}</dd>
					</div>
				{/if}
				{#if status.tunnel_url}
					<div>
						<dt>{t.remote.tunnelAddress}</dt>
						<dd class="remote-value--plain">{status.tunnel_url}</dd>
					</div>
				{/if}
				{#if status.mapped_method}
					<div>
						<dt>{t.remote.opened}</dt>
						<dd>{t.remote.methods[status.mapped_method] ?? status.mapped_method}</dd>
					</div>
				{/if}
			{/if}
		</dl>

		<!-- A public address that moved is worth interrupting for: every
		     configuration handed out before it now points at somebody else. -->
		{#if status.endpoint_changed}
			<p class="remote-note remote-note--warn">{t.remote.addressChanged}</p>
		{/if}

		{#if discoveryFailed}
			<p class="remote-error">{t.remote.discovery[status.discovery_reason] ?? t.remote.discovery.remote_router_silent}</p>
		{/if}

		{#if running}
			<!-- Said before anything else, because it is what the two addresses
			     above are for, and what the panel was otherwise silent on. -->
			<p class="remote-note">{t.remote.howItWorks}</p>
			<p class="remote-note">
				{status.reachability === 'confirmed' ? t.remote.confirmedHelp : t.remote.unverifiedHelp}
			</p>
		{/if}

		{#if failed && status.reason}
			<!-- An error state always keeps Disable reachable: it is the documented
			     way back, and the LAN never went away. -->
			<p class="remote-error">{reason(status.reason)}</p>
		{/if}

		{#if formError}
			<p class="remote-error" role="status">{formError}</p>
		{/if}

		<div class="remote-actions">
			{#if status.enabled}
				<button
					type="button"
					class="tv-action cursor-pointer"
					onclick={() => send({ enabled: false })}
					disabled={busy}
				>
					{t.remote.disable}
				</button>
			{:else}
				<button
					type="button"
					class="tv-action tv-action--primary cursor-pointer"
					onclick={enable}
					disabled={busy}
					data-remote-default
				>
					{busy ? t.remote.opening : t.remote.enable}
				</button>
			{/if}
		</div>

		<!--
			The old panel led with this: a UDP port, a public endpoint, and a
			paragraph about forwarding a port on a router Theia has never seen.
			Both fields are answers the gateway already holds, so it is asked
			instead. They stay, folded away, because a router with UPnP switched
			off is a real evening and somebody who forwarded the port by hand
			must not be told to undo it.
		-->
		<details class="remote-manual" bind:open={manualOpen}>
			<summary class="tv-action cursor-pointer">{t.remote.manual}</summary>

			<p class="remote-caution">
				<Icon name="info" size={18} />
				<span>{t.remote.router}</span>
			</p>
			<p class="remote-note">{t.remote.cgnat}</p>

			<div class="remote-fields">
				<label class="remote-field">
					<span class="label">{t.remote.port}</span>
					<input class="profile-input" type="text" inputmode="numeric" bind:value={portDraft} />
					<span class="remote-help">{t.remote.portHelp}</span>
				</label>

				<label class="remote-field">
					<span class="label">{t.remote.endpoint}</span>
					<input
						class="profile-input"
						type="text"
						bind:value={endpointDraft}
						placeholder={t.remote.endpointPlaceholder}
					/>
					<span class="remote-help">{t.remote.endpointHelp}</span>
					<span class="remote-help">{t.remote.endpointExamples}</span>
				</label>
			</div>

			<!-- Both consequences are stated before the save, not discovered after. -->
			{#if endpointChanged}
				<p class="remote-note remote-note--warn">{t.remote.endpointChange}</p>
			{/if}
			{#if portChanged && running}
				<p class="remote-note remote-note--warn">{t.remote.portChange}</p>
			{/if}

			<div class="remote-actions">
				{#if dirty}
					<button type="button" class="tv-action cursor-pointer" onclick={saveConfig} disabled={busy}>
						{busy ? t.remote.saving : t.remote.save}
					</button>
				{/if}
				{#if !automatic}
					<button
						type="button"
						class="tv-action cursor-pointer"
						onclick={enable}
						disabled={busy}
					>
						{t.remote.retryAutomatic}
					</button>
				{/if}
			</div>
		</details>

		<h3 class="label mt-12 mb-4">{t.remote.devices}</h3>

		{#if !running}
			<p class="remote-note">{t.remote.enableFirst}</p>
		{/if}

		{#if status.peers?.length}
			<ul class="remote-devices">
				{#each status.peers as peer (peer.id)}
					<li class="remote-device">
						<div class="min-w-0 flex-1">
							<p class="remote-device-name">{peer.name}</p>
							<p class="remote-device-facts">
								<span>{peer.address}</span>
								<span>{t.remote.lastHandshake} : {handshake(peer.last_handshake_at)}</span>
							</p>
							<p class="remote-device-facts">
								<span>
									↓ {formatSize(peer.received_bytes ?? 0)} · ↑ {formatSize(peer.transmitted_bytes ?? 0)}
								</span>
							</p>
						</div>

						{#if revoking === peer.id}
							<div class="remote-device-confirm">
								<p class="tv-copy text-small">{t.remote.revokeConfirm(peer.name)}</p>
								<div class="remote-actions">
									<button
										type="button"
										class="tv-action remote-destructive cursor-pointer"
										onclick={() => revoke(peer.id)}
										disabled={busy}
									>
										{t.remote.revokeYes}
									</button>
									<button type="button" class="tv-action cursor-pointer" onclick={() => (revoking = null)}>
										{t.remote.cancel}
									</button>
								</div>
							</div>
						{:else}
							<button
								type="button"
								class="tv-action remote-destructive cursor-pointer"
								onclick={() => (revoking = peer.id)}
							>
								{t.remote.revoke}
							</button>
						{/if}
					</li>
				{/each}
			</ul>
			<p class="remote-note">{t.remote.trafficHelp}</p>
		{:else if running}
			<p class="remote-note">{t.remote.noDevices}</p>
		{/if}

		{#if running}
			{#if adding}
				<form class="remote-add" onsubmit={createDevice}>
					<label class="remote-field">
						<span class="label">{t.remote.deviceName}</span>
						<input
							class="profile-input"
							bind:value={deviceName}
							maxlength="64"
							placeholder={t.remote.devicePlaceholder}
						/>
					</label>
					<div class="remote-actions">
						<button type="submit" class="tv-action tv-action--primary cursor-pointer" disabled={busy}>
							{busy ? t.remote.creating : t.remote.create}
						</button>
						<button type="button" class="tv-action cursor-pointer" onclick={() => (adding = false)}>
							{t.remote.cancel}
						</button>
					</div>
				</form>
			{:else}
				<button type="button" class="tv-action mt-5 cursor-pointer" onclick={() => (adding = true)}>
					<Icon name="plus" size={16} />
					<span>{t.remote.addDevice}</span>
				</button>
			{/if}
		{/if}
	{/if}
</section>

{#if provision}
	<ProvisionDialog {provision} onclose={closeProvision} />
{/if}
