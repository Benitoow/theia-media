// Whether this browser is on the LAN or inside the WireGuard tunnel.
//
// The distinction is not cosmetic. Settings, scanning, onboarding, the updater
// and device management are LAN-only and the remote guard refuses them outright
// (decision 44). A remote session must therefore not *ask*: turning a deliberate
// 403 into a red screen would report a working security boundary as a fault.

import { getJSON } from '$lib/api.js';

class RemoteSession {
	/** @type {'lan' | 'remote'} */
	mode = $state('lan');
	/** @type {{id: number, name: string, address: string} | null} */
	peer = $state(null);
	loaded = $state(false);

	get isRemote() {
		return this.mode === 'remote';
	}

	async load() {
		try {
			const session = await getJSON('/api/remote-access/session');
			this.mode = session.mode === 'remote' ? 'remote' : 'lan';
			this.peer = session.peer ?? null;
		} catch {
			// A server too old to know this route, or one that failed to answer, is
			// treated as the LAN it almost certainly is. Hiding the settings from
			// somebody sitting at home would be the worse failure.
			this.mode = 'lan';
			this.peer = null;
		} finally {
			this.loaded = true;
		}
	}
}

export const remote = new RemoteSession();
