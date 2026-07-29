import { browser } from '$app/environment';

const storageKey = 'theia.active-profile';

async function profileRequest(path, options) {
	const response = await fetch(path, options);
	let payload = null;
	try {
		payload = await response.json();
	} catch {
		// DELETE succeeds without a body, and a broken connection may not have
		// one either. The status still carries the useful part.
	}
	if (!response.ok) {
		const error = new Error(payload?.error || `HTTP ${response.status}`);
		error.status = response.status;
		error.code = payload?.error;
		throw error;
	}
	return payload;
}

class ProfileSession {
	profiles = $state([]);
	activeID = $state(null);
	ready = $state(false);
	unreachable = $state(false);

	get active() {
		const found = this.profiles.find((profile) => profile.id === this.activeID);
		// When the server is down, preserve enough of the cached selection to
		// let the normal offline screen render instead of replacing it with a
		// second, less useful failure state.
		return found ?? (this.unreachable && this.activeID ? { id: this.activeID, name: null } : null);
	}

	get needsSelection() {
		return this.ready && !this.unreachable && this.profiles.length > 1 && !this.active;
	}

	async bootstrap() {
		if (this.ready) return;
		const stored = this.readStoredID();
		try {
			const payload = await profileRequest('/api/profiles');
			this.profiles = payload?.profiles ?? [];
			const selected = this.profiles.find((profile) => profile.id === stored);
			if (selected) {
				this.activeID = selected.id;
			} else if (this.profiles.length === 1) {
				this.remember(this.profiles[0].id);
			} else {
				this.activeID = null;
			}
		} catch {
			this.unreachable = true;
			this.activeID = stored ?? 1;
		} finally {
			this.ready = true;
		}
	}

	select(id) {
		if (!this.profiles.some((profile) => profile.id === id)) return false;
		this.remember(id);
		return true;
	}

	async refresh() {
		const payload = await profileRequest('/api/profiles');
		this.profiles = payload?.profiles ?? [];
		this.unreachable = false;
		if (!this.profiles.some((profile) => profile.id === this.activeID)) {
			this.activeID = null;
			if (this.profiles.length === 1) this.remember(this.profiles[0].id);
		}
		return this.profiles;
	}

	async create(name) {
		const profile = await profileRequest('/api/profiles', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		});
		this.profiles = [...this.profiles, profile];
		return profile;
	}

	async rename(id, name) {
		const updated = await profileRequest(`/api/profiles/${id}`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		});
		this.replace(updated);
		return updated;
	}

	async remove(id) {
		await profileRequest(`/api/profiles/${id}`, { method: 'DELETE' });
		this.profiles = this.profiles.filter((profile) => profile.id !== id);
		if (this.activeID === id) {
			const fallback = this.profiles.find((profile) => profile.is_default) ?? this.profiles[0];
			if (fallback) this.remember(fallback.id);
		}
	}

	async uploadAvatar(id, file) {
		const updated = await profileRequest(`/api/profiles/${id}/avatar`, {
			method: 'PUT',
			headers: { 'Content-Type': file.type || 'application/octet-stream' },
			body: file
		});
		this.replace(updated);
		return updated;
	}

	async removeAvatar(id) {
		const updated = await profileRequest(`/api/profiles/${id}/avatar`, {
			method: 'DELETE'
		});
		this.replace(updated);
		return updated;
	}

	replace(updated) {
		this.profiles = this.profiles.map((profile) => (profile.id === updated.id ? updated : profile));
	}

	remember(id) {
		this.activeID = id;
		if (!browser) return;
		try {
			localStorage.setItem(storageKey, String(id));
		} catch {
			// A privacy mode may deny storage. The profile still works for the
			// current page; only persistence is lost.
		}
	}

	readStoredID() {
		if (!browser) return null;
		try {
			const parsed = Number.parseInt(localStorage.getItem(storageKey) ?? '', 10);
			return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : null;
		} catch {
			return null;
		}
	}
}

export const profileSession = new ProfileSession();

export function activeProfileID() {
	return profileSession.activeID;
}

export function profileAvatarURL(profile) {
	if (!profile?.has_avatar || !profile.avatar_version) return null;
	return `/api/profiles/${profile.id}/avatar/${profile.avatar_version}`;
}
