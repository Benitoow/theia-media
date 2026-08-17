<script>
	import { imageURL } from '$lib/api.js';

	// cast is the stored TMDB order, which is billing order: the lead first.
	let { cast = [], heading } = $props();

	// w185 is the smallest TMDB portrait size the image cache whitelists, and it
	// is still twice the 3.5rem the frame draws at -- which is what a phone at
	// three device pixels needs. Nothing here asks for a larger one.
	const portraits = $derived(
		cast.map((person) => ({
			name: person.name,
			character: person.character ?? '',
			src: imageURL(person.profile_path, 'w185'),
			initial: (person.name?.trim()?.[0] ?? '?').toUpperCase()
		}))
	);
</script>

{#if portraits.length}
	<section class="mb-10">
		<h2 class="label mb-5">{heading}</h2>
		<!--
			Two columns, as this list has always had, with a portrait added in
			front of each name. Not a scrolling strip: rows scroll on the home
			screen because they are a catalogue you skim, and this is a fact about
			one film that should be readable in one glance without moving anything.
		-->
		<ul class="cast-list">
			{#each portraits as person (person.name + person.character)}
				<li class="cast-member">
					<div class="cast-portrait">
						{#if person.src}
							<img
								src={person.src}
								alt=""
								loading="lazy"
								decoding="async"
								class="h-full w-full object-cover"
							/>
						{:else}
							<!-- The same composed stand-in a card uses for missing artwork,
							     at this size. An empty frame among nine photographs reads as
							     a picture that failed to load. -->
							<span class="cast-portrait-initial" aria-hidden="true">{person.initial}</span>
						{/if}
					</div>
					<div class="min-w-0">
						<span class="cast-name">{person.name}</span>
						{#if person.character}
							<span class="cast-character label">{person.character}</span>
						{/if}
					</div>
				</li>
			{/each}
		</ul>
	</section>
{/if}
