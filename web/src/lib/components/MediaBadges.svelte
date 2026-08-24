<script>
	// What a file is, in six words or fewer.
	//
	// Purely decorative, and bound by the same rule as the file chooser: only
	// measured facts. Nothing here is read off a filename, however loudly the
	// filename shouts DOLBY VISION -- a file that has not been inspected shows no
	// badges at all rather than repeating what somebody typed when they named it.
	//
	// The audio badges describe the track the file marks as default, falling back
	// to the first. That is a fact about the file, not a judgement about which
	// track is best; ranking tracks is the thing decision 38 refuses.
	import { strings as t } from '$lib/strings.js';

	let { media = null } = $props();

	const badges = $derived(build(media));

	function build(media) {
		if (!media || media.status !== 'ok') return [];
		const b = t.film.badges;
		const out = [];

		const video = media.video;
		if (video?.width > 0) out.push(b[resolution(video.width)]);
		// PQ and HLG are different things and a file knows which it is, so it is
		// not flattened into one word.
		if (video?.color_transfer === 'smpte2084') out.push(b.hdr);
		if (video?.color_transfer === 'arib-std-b67') out.push(b.hlg);
		if (video?.dolby_vision) out.push(b.dolbyVision);

		const track = defaultTrack(media.audio_tracks);
		if (track) {
			const name = audioName(track, b);
			if (name) out.push(name);
			if (/atmos/i.test(track.profile ?? '')) out.push(b.atmos);
			const channels = channelName(track.channels, b);
			if (channels) out.push(channels);
		}
		return out.filter(Boolean);
	}

	// Width, not height. A scope film is letterboxed in the file itself: this
	// one is 3840x1604, so anything keyed on height would call a 4K master
	// "1604p" and a 1920x804 one "804p".
	function resolution(width) {
		if (width >= 3840) return 'uhd';
		if (width >= 2560) return 'qhd';
		if (width >= 1900) return 'fhd';
		if (width >= 1200) return 'hd';
		return 'sd';
	}

	function defaultTrack(tracks) {
		if (!tracks?.length) return null;
		return tracks.find((track) => track.is_default) ?? tracks[0];
	}

	function audioName(track, b) {
		const codec = (track.codec ?? '').toLowerCase();
		const profile = (track.profile ?? '').toLowerCase();
		// DTS is the one codec whose profile changes the name people use for it:
		// a DTS-HD Master Audio track is not "DTS" to anybody who owns one.
		if (codec === 'dts') return profile.includes('ma') ? b.dtshd : b.dts;
		return b[codec] ?? codec.toUpperCase();
	}

	function channelName(channels, b) {
		const value = (channels ?? '').toLowerCase();
		if (!value) return '';
		if (value === 'stereo') return b.stereo;
		if (value === 'mono') return b.mono;
		// "5.1(side)" is a layout, and the parenthesis is for a mixing desk.
		return b.channels(value.replace(/\(.*\)$/, ''));
	}
</script>

{#if badges.length}
	<!-- A list rather than a row of spans: a screen reader announces how many
	     there are and reads them one by one, which is the difference between six
	     facts and one run-on string. None of them is focusable, so the D-pad
	     rules in decision 27 have nothing to do here. -->
	<ul class="badges" aria-label={t.film.badges.title}>
		{#each badges as badge (badge)}
			<li class="badge">{badge}</li>
		{/each}
	</ul>
{/if}
