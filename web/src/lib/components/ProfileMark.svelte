<script>
	// A profile's picture, or the mark that stands in for one.
	//
	// Never a broken-image icon and never an empty square: a profile without a
	// picture gets an initial on a colour derived from its own name, so the card
	// stays recognisable from three metres before anybody uploads anything.
	import { avatarURL, avatarHue, avatarInitial } from '$lib/profiles.svelte.js';
	import { strings as t } from '$lib/strings.js';

	let { profile, round = false, size = null } = $props();

	const url = $derived(avatarURL(profile));
	const hue = $derived(avatarHue(profile));
	const name = $derived(profile?.name || t.profiles.defaultName);
	const initial = $derived(avatarInitial(profile, name.slice(0, 1).toUpperCase()));
</script>

<span
	class="profile-mark"
	class:profile-mark--round={round}
	style="--profile-hue: {hue}{size ? `; --profile-mark-size: ${size}` : ''}"
>
	{#if url}
		<img src={url} alt="" decoding="async" />
	{:else}
		<span aria-hidden="true">{initial}</span>
	{/if}
</span>
