<script>
	import { profileAvatarURL } from '$lib/profiles.svelte.js';

	let { profile, large = false } = $props();
	const avatar = $derived(profileAvatarURL(profile));
</script>

<span class:profile-avatar--large={large} class="profile-avatar" aria-hidden="true">
	{#if avatar}
		<img src={avatar} alt="" decoding="async" />
	{:else}
		<span class="profile-mark">
			<span></span>
		</span>
	{/if}
</span>

<style>
	.profile-avatar {
		display: inline-grid;
		width: 2rem;
		height: 2rem;
		flex: 0 0 auto;
		place-items: center;
		overflow: hidden;
		border: 1px solid var(--color-line);
		border-radius: 50%;
		background:
			radial-gradient(circle at 50% 15%, rgb(200 162 74 / 0.16), transparent 60%),
			var(--color-raised);
	}

	.profile-avatar--large {
		width: 100%;
		height: auto;
		aspect-ratio: 1;
		border-radius: clamp(1rem, 2vw, 1.5rem);
	}

	img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.profile-mark {
		position: relative;
		display: block;
		width: 42%;
		aspect-ratio: 1;
		border: 2px solid var(--color-accent);
		border-radius: 50%;
	}

	.profile-mark span {
		position: absolute;
		inset: 24%;
		border-radius: 50%;
		background: var(--color-accent);
		box-shadow: 0 0 1.2rem rgb(200 162 74 / 0.45);
	}

	.profile-avatar--large .profile-mark {
		border-width: 3px;
	}
</style>
