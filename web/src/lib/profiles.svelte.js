// Who is watching, as far as this browser is concerned.
//
// The selection is local, exactly like the interface language (decision 32):
// a television and a laptop must be able to disagree about who is watching
// without one changing the other underneath somebody mid-film. The server is
// told which history to use through ?profile=, never through a credential.

import { browser } from '$app/environment';
import { getJSON, apiFetch } from '$lib/api.js';

const storageKey = 'theia.profile';

class Profiles {
	/** @type {Array<{id: number, name?: string, is_default: boolean, has_avatar: boolean, avatar_version?: number}>} */
	list = $state([]);
	/** @type {number | null} */
	activeID = $state(null);
	loaded = $state(false);

	get active() {
		return this.list.find((profile) => profile.id === this.activeID) ?? null;
	}

	// The chooser is owed an answer when nothing has been selected in this
	// browser, or when the selection names a profile that has since been deleted
	// from another device.
	//
	// One profile is not a question. A household that never creates a second one
	// would otherwise meet "who's watching?" on every new device, with a single
	// card to press -- a click whose answer is already known, between the
	// television and a film. The founding criterion is a film in under three
	// clicks; this is the kind of screen that quietly breaks it. The chooser
	// appears the moment there is an actual choice to make.
	get needsSelection() {
		return this.loaded && this.active === null && this.list.length > 1;
	}

	async load() {
		const payload = await getJSON('/api/profiles');
		this.list = payload.profiles ?? [];

		if (browser && this.activeID === null) {
			try {
				const stored = Number(localStorage.getItem(storageKey));
				if (Number.isInteger(stored) && stored > 0) this.activeID = stored;
			} catch {
				// Private modes deny storage. The chooser simply asks every visit.
			}
		}
		// A stale id is dropped rather than sent: the server would refuse it with
		// profile_not_found on every single request.
		if (!this.list.some((profile) => profile.id === this.activeID)) {
			this.activeID = null;
		}
		// With a single profile the answer is not in doubt, so it is adopted
		// rather than asked for. See needsSelection.
		if (this.activeID === null && this.list.length === 1) {
			this.select(this.list[0].id);
		}
		this.loaded = true;
		return this.list;
	}

	select(id) {
		this.activeID = id;
		if (!browser) return;
		try {
			localStorage.setItem(storageKey, String(id));
		} catch {
			// The session still follows even when persistence is blocked.
		}
	}

	/** Appends ?profile= to a path, preserving any query it already carries. */
	url(path) {
		if (this.activeID === null) return path;
		return path + (path.includes('?') ? '&' : '?') + 'profile=' + this.activeID;
	}

	async create(name) {
		const response = await apiFetch('/api/profiles', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		});
		return this.#applied(response);
	}

	async rename(id, name) {
		const response = await apiFetch(`/api/profiles/${id}`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		});
		return this.#applied(response);
	}

	async setAvatar(id, file) {
		const response = await apiFetch(`/api/profiles/${id}/avatar`, {
			method: 'PUT',
			headers: { 'Content-Type': file.type || 'application/octet-stream' },
			body: file
		});
		return this.#applied(response);
	}

	async remove(id) {
		const response = await apiFetch(`/api/profiles/${id}`, { method: 'DELETE' });
		if (!response.ok) throw await errorFrom(response);
		if (this.activeID === id) this.activeID = null;
		await this.load();
	}

	async #applied(response) {
		if (!response.ok) throw await errorFrom(response);
		const profile = await response.json();
		await this.load();
		return profile;
	}
}

async function errorFrom(response) {
	const error = new Error(`HTTP ${response.status}`);
	error.status = response.status;
	try {
		const body = await response.json();
		if (typeof body?.error === 'string') error.code = body.error;
	} catch {
		// A failure without a JSON body still carries its status.
	}
	return error;
}

/** The picture URL, versioned so it can be cached immutably and still change. */
export function avatarURL(profile) {
	if (!profile?.has_avatar) return null;
	return `/api/profiles/${profile.id}/avatar?v=${profile.avatar_version ?? 0}`;
}

/** A stable colour per profile, so a picture-less card is still recognisable. */
export function avatarHue(profile) {
	const source = profile?.name ?? String(profile?.id ?? '');
	let hash = 0;
	for (let i = 0; i < source.length; i++) hash = (hash * 31 + source.charCodeAt(i)) % 360;
	return hash;
}

/** The letter a picture-less card shows. Never a broken-image icon. */
export function avatarInitial(profile, fallback) {
	const name = (profile?.name ?? '').trim();
	if (!name) return fallback;
	return [...name][0].toUpperCase();
}

export const profiles = new Profiles();
