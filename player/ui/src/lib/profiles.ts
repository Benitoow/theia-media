import type { Profile } from '../types';
export function profileAvatarURL(serverURL: string, profile?: Profile) {
	if (!profile?.has_avatar) return null;
	if (profile.avatar_url) return profile.avatar_url;
	return `${serverURL.replace(/\/$/, '')}/api/profiles/${profile.id}/avatar?v=${profile.avatar_version ?? 0}`;
}
