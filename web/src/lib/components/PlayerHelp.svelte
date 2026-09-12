<script>
	import { strings as t } from '$lib/strings.js';
	import Icon from './Icon.svelte';

	let { panel = $bindable(), onclose } = $props();
</script>

<section
	bind:this={panel}
	class="player-shortcuts"
	aria-labelledby="player-shortcuts-title"
>
	<div class="player-shortcuts-header">
		<div>
			<span class="label">{t.appName}</span>
			<h2 id="player-shortcuts-title">{t.player.shortcuts.title}</h2>
		</div>
		<button
			type="button"
			onclick={onclose}
			class="player-icon-button"
			data-shortcuts-close
		>
			<Icon name="close" label={t.player.shortcuts.close} />
		</button>
	</div>

	<p class="tv-copy player-shortcuts-intro">{t.player.shortcuts.intro}</p>

	<dl class="player-shortcuts-list">
		{#each t.player.shortcuts.items as shortcut (shortcut.action)}
			<div>
				<dt>
					{#each shortcut.keys as key (key)}
						<kbd>{key}</kbd>
					{/each}
				</dt>
				<dd>{shortcut.action}</dd>
			</div>
		{/each}
	</dl>

	<p class="player-shortcuts-hint">{t.player.shortcuts.scrubHint}</p>
</section>
