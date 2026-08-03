<script>
	// Remote access, administered from the LAN only.
	//
	// The whole point of M4 is that the tunnel is the *only* way in from outside
	// and that the historical HTTP port stays private. So this panel says so
	// plainly: the router forwards UDP, never TCP, and CGNAT is an unsupported
	// case rather than something Theia quietly works around.
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

	const running = $derived(status?.state === 'running');
	const failed = $derived(status?.state === 'error');
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
	}

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

		<!-- Said before anything can be switched on, not after. -->
		<p class="remote-caution">
			<Icon name="info" size={18} />
			<span>{t.remote.router}</span>
		</p>
		<p class="remote-note">{t.remote.cgnat}</p>

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
			{/if}
		</dl>

		{#if running}
			<p class="remote-note">
				{status.reachability === 'confirmed' ? t.remote.confirmedHelp : t.remote.unverifiedHelp}
			</p>
		{/if}

		{#if failed && status.reason}
			<!-- An error state always keeps Disable reachable: it is the documented
			     way back, and the LAN never went away. -->
			<p class="remote-error">{reason(status.reason)}</p>
		{/if}

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

		{#if formError}
			<p class="remote-error" role="status">{formError}</p>
		{/if}

		<div class="remote-actions">
			{#if dirty}
				<button type="button" class="tv-action cursor-pointer" onclick={saveConfig} disabled={busy}>
					{busy ? t.remote.saving : t.remote.save}
				</button>
			{/if}

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
					onclick={() => send({ enabled: true, listen_port: Number(portDraft), endpoint: endpointDraft.trim() })}
					disabled={busy}
				>
					{t.remote.enable}
				</button>
			{/if}
		</div>

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
