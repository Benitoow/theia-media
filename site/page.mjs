// Shared FR/EN template for the public site. Essential content, anchors and
// all six downloads exist in the markup before JavaScript runs.

const REPO = 'https://github.com/Benitoow/theia-media';
const SITE = 'https://benitoow.github.io/theia-media';
const LATEST = `${REPO}/releases/latest/download`;

export const PLATFORMS = [
	{
		id: 'windows',
		assets: [
			{ arch: 'x64', file: 'theia-windows-amd64.exe' },
			{ arch: 'ARM64', file: 'theia-windows-arm64.exe' }
		]
	},
	{
		id: 'macos',
		assets: [
			{ arch: 'Apple Silicon', file: 'theia-darwin-arm64' },
			{ arch: 'Intel', file: 'theia-darwin-amd64' }
		]
	},
	{
		id: 'linux',
		assets: [
			{ arch: 'x64', file: 'theia-linux-amd64' },
			{ arch: 'ARM64', file: 'theia-linux-arm64' }
		]
	}
];

const MOMENT_ASSETS = ['library.webp', null, 'settings.webp'];

const escape = (value) =>
	String(value)
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;');

const scriptJSON = (value) => JSON.stringify(value).replace(/</g, '\\u003c');

function icon(name) {
	return {
		play: `<svg class="icon-play" viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M8 5v14l11-7z"/></svg><svg class="icon-pause" viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M7 5h4v14H7zm6 0h4v14h-4z"/></svg>`,
		settings: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 8.6a3.4 3.4 0 1 0 0 6.8 3.4 3.4 0 0 0 0-6.8Z" fill="none" stroke="currentColor" stroke-width="1.7"/><path d="m19 13.2 1.2 1-.7 1.8-1.6-.1-1.2 1.2.1 1.6-1.8.7-1-1.2h-1.8l-1 1.2-1.8-.7.1-1.6-1.2-1.2-1.6.1-.7-1.8 1.2-1v-1.8L6 10.2l.7-1.8 1.6.1 1.2-1.2-.1-1.6 1.8-.7 1 1.2H14l1-1.2 1.8.7-.1 1.6 1.2 1.2 1.6-.1.7 1.8-1.2 1v1.8Z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/></svg>`,
		volume: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 10v4h3l4 3V7l-4 3H5Zm10-1.5c1.3.8 2 2 2 3.5s-.7 2.7-2 3.5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/></svg>`
	}[name];
}

function formatSize(bytes, lang) {
	if (!Number.isFinite(bytes) || bytes <= 0) return '';
	const value = new Intl.NumberFormat(lang, {
		minimumFractionDigits: 1,
		maximumFractionDigits: 1
	}).format(bytes / 1_000_000);
	return `${value}${lang === 'fr' ? ' Mo' : ' MB'}`;
}

function formatDate(value, lang) {
	if (!value) return '';
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return '';
	return new Intl.DateTimeFormat(lang, {
		day: 'numeric',
		month: 'long',
		year: 'numeric',
		timeZone: 'UTC'
	}).format(date);
}

function releaseAsset(release, file) {
	return release?.assets?.find((asset) => asset.file === file);
}

function releaseMeta(release, asset, lang) {
	return [release?.version, formatSize(asset?.size, lang), formatDate(release?.publishedAt, lang)]
		.filter(Boolean)
		.join(' · ');
}

function downloadPanel(t, release, platform) {
	const system = t.downloads.systems[platform.id];
	const links = platform.assets
		.map((declared) => {
			const asset = releaseAsset(release, declared.file);
			const meta = releaseMeta(release, asset, t.lang);
			return `<a class="arch-download" href="${LATEST}/${declared.file}" aria-label="${escape(`${t.downloads.download} ${system} ${declared.arch}`)}">
			<span><strong>${escape(declared.arch)}</strong>${meta ? `<small>${escape(meta)}</small>` : ''}</span><span aria-hidden="true">↓</span>
		</a>`;
		})
		.join('');

	const digests = platform.assets
		.map((declared) => {
			const asset = releaseAsset(release, declared.file);
			if (!asset?.sha256) return '';
			return `<span><span class="checksum-name">${escape(declared.file)}</span><code>${escape(asset.sha256)}</code></span>`;
		})
		.filter(Boolean)
		.join('');

	const releaseURL = release?.url || `${REPO}/releases/latest`;
	return `<section class="download-panel" id="${platform.id}-download" data-download-panel="${platform.id}" aria-labelledby="${platform.id}-download-title">
		<h3 id="${platform.id}-download-title">${escape(system)} · ${escape(t.downloads.chooseArch)}</h3>
		<p class="platform-warning">${escape(t.downloads.warnings[platform.id])}</p>
		<div class="arch-list">${links}</div>
		<div class="download-meta-row">
			<a class="text-link" href="${escape(releaseURL)}">${escape(t.downloads.releaseNotes)} <span aria-hidden="true">↗</span></a>
			${digests ? `<details class="checksums"><summary>${escape(t.downloads.shaLabel)}</summary><div class="checksum-grid">${digests}</div></details>` : ''}
		</div>
	</section>`;
}

function trackOptions(t) {
	return [
		[t.playerDemo.audio, 'audio'],
		[t.playerDemo.quality, 'quality'],
		[t.playerDemo.subtitles, 'subtitles']
	]
		.map(
			([heading, type]) => `<p class="track-heading">${escape(heading)}</p>${t.playerDemo.trackOptions[type]
				.map(
					([primary, detail], index) => `<button class="track-option" type="button" data-track="${type}" aria-pressed="${index === 0}">
					<span class="track-copy"><strong>${escape(primary)}</strong>${detail ? `<small>${escape(detail)}</small>` : ''}</span><span class="track-tick" aria-hidden="true">✓</span>
				</button>`
				)
				.join('')}`
		)
		.join('');
}

function moments(t, base) {
	return t.moments.items
		.map((moment, index) => {
			const asset = MOMENT_ASSETS[index];
			const media = asset
				? `<figure class="moment-media"><img src="${base}shots/${asset}" alt="${escape(moment.alt)}" width="1600" height="900" loading="lazy" decoding="async"></figure>`
				: `<div class="moment-media watch-panel" role="group" aria-label="${escape(t.playerDemo.tracksTitle)}">${t.moments.watchFacts
						.map(
							([label, value]) => `<div class="watch-fact"><span>${escape(label)}</span><strong>${escape(value)}</strong></div>`
						)
						.join('')}</div>`;
			return `<li class="moment"><span class="moment-number">${String(index + 1).padStart(2, '0')}</span><div class="moment-copy"><b>${escape(moment.label)}</b><h3>${escape(moment.title)}</h3><p>${escape(moment.body)}</p></div>${media}</li>`;
		})
		.join('');
}

function structuredData(t, release) {
	return {
		'@context': 'https://schema.org',
		'@graph': [
			{
				'@type': 'SoftwareApplication',
				name: 'Theia',
				url: t.canonical,
				applicationCategory: 'MultimediaApplication',
				operatingSystem: 'Windows, macOS, Linux',
				softwareVersion: release?.version || undefined,
				datePublished: release?.publishedAt || undefined,
				license: 'https://www.gnu.org/licenses/gpl-3.0.html',
				downloadUrl: `${REPO}/releases/latest`,
				description: t.meta.description
			},
			{
				'@type': 'FAQPage',
				mainEntity: t.faq.items.map(([question, answer]) => ({
					'@type': 'Question',
					name: question,
					acceptedAnswer: { '@type': 'Answer', text: answer }
				}))
			}
		]
	};
}

export function render(t, release = {}, build = {}) {
	const base = t.dir ? '../' : './';
	const styleQuery = build.styleVersion ? `?v=${encodeURIComponent(build.styleVersion)}` : '';
	const alternateLocale = t.lang === 'fr' ? 'en_US' : 'fr_FR';
	const clientLabels = scriptJSON({
		statusSelected: t.downloads.statusSelected,
		play: t.playerDemo.play,
		pause: t.playerDemo.pause,
		mute: t.playerDemo.mute,
		unmute: t.playerDemo.unmute
	});
	const jsonLD = scriptJSON(structuredData(t, release));

	return `<!doctype html>
<html lang="${t.lang}">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<meta name="color-scheme" content="dark">
	<meta name="theme-color" content="#0b0a09">
	<title>${escape(t.meta.title)}</title>
	<meta name="description" content="${escape(t.meta.description)}">
	<link rel="icon" href="${base}favicon.svg" type="image/svg+xml">
	<link rel="apple-touch-icon" href="${base}apple-touch-icon.png">
	<link rel="canonical" href="${t.canonical}">
	<link rel="alternate" hreflang="fr" href="${SITE}/">
	<link rel="alternate" hreflang="en" href="${SITE}/en/">
	<link rel="alternate" hreflang="x-default" href="${SITE}/">
	<meta property="og:type" content="website">
	<meta property="og:site_name" content="Theia">
	<meta property="og:url" content="${t.canonical}">
	<meta property="og:locale" content="${t.ogLocale}">
	<meta property="og:locale:alternate" content="${alternateLocale}">
	<meta property="og:title" content="${escape(t.meta.title)}">
	<meta property="og:description" content="${escape(t.meta.description)}">
	<meta property="og:image" content="${SITE}/social-preview.png">
	<meta property="og:image:width" content="1200">
	<meta property="og:image:height" content="630">
	<meta property="og:image:alt" content="${escape(t.meta.ogAlt)}">
	<meta name="twitter:card" content="summary_large_image">
	<meta name="twitter:title" content="${escape(t.meta.title)}">
	<meta name="twitter:description" content="${escape(t.meta.description)}">
	<meta name="twitter:image" content="${SITE}/social-preview.png">
	<link rel="preload" href="${base}fonts/playfair-display.woff2" as="font" type="font/woff2" crossorigin>
	<link rel="preload" href="${base}fonts/inter.woff2" as="font" type="font/woff2" crossorigin>
	<link rel="preload" href="${base}shots/player.webp" as="image" type="image/webp" fetchpriority="high">
	<link rel="stylesheet" href="${base}styles.css${styleQuery}">
	<script type="application/ld+json">${jsonLD}</script>
	<script>document.documentElement.classList.add('has-js')</script>
</head>
<body>
	<a class="skip-link" href="#content">${escape(t.skip)}</a>
	<header class="site-header">
		<nav class="shell topnav" aria-label="${escape(t.nav.label)}">
			<a class="brand" href="#content">THEIA</a>
			<div class="nav-links">
				<a href="#moments">${escape(t.nav.moments)}</a>
				<a href="#difference">${escape(t.nav.difference)}</a>
				<a class="nav-download" href="#try">${escape(t.nav.download)}</a>
				<a class="language" href="${t.other.href}" lang="${t.other.code}" hreflang="${t.other.code}">${escape(t.other.label)}</a>
			</div>
		</nav>
	</header>

	<main id="content" tabindex="-1">
		<section class="shell hero">
			<div class="hero-copy">
				<p class="eyebrow">${escape(t.hero.eyebrow)}</p>
				<h1>${escape(t.hero.title)}</h1>
				<p class="hero-lead">${escape(t.hero.lead)}</p>
			</div>

			<figure class="player-proof">
				<div class="player-demo" data-player-demo>
					<img class="player-frame" src="${base}shots/player.webp" alt="${escape(t.playerDemo.alt)}" width="1280" height="720" decoding="async" fetchpriority="high">
					<span class="demo-chip">${escape(t.playerDemo.status)}</span>
					<header class="player-top"><span class="player-heading">${escape(t.playerDemo.title)}</span></header>
					<div class="player-busy" data-player-busy hidden aria-live="polite"><span class="spinner" aria-hidden="true"></span><span>${escape(t.playerDemo.loading)}</span></div>
					<div class="player-controls">
						<input class="scrub" type="range" min="0" max="7268" value="2538" style="--progress:34.92%" aria-label="${escape(t.playerDemo.position)}" data-player-progress>
						<div class="player-buttons">
							<button class="icon-button icon-button--primary" type="button" aria-label="${escape(t.playerDemo.play)}" aria-pressed="false" data-play-toggle>${icon('play')}</button>
							<button class="icon-button" type="button" aria-label="${escape(t.playerDemo.back10)}" data-seek="-10"><span aria-hidden="true">−10</span></button>
							<button class="icon-button" type="button" aria-label="${escape(t.playerDemo.forward10)}" data-seek="10"><span aria-hidden="true">+10</span></button>
							<span class="player-time">42:18 <i aria-hidden="true"></i> 2:01:08</span>
							<span class="player-spacer"></span>
							<button class="icon-button volume-control" type="button" aria-label="${escape(t.playerDemo.mute)}" aria-pressed="false" data-mute-toggle>${icon('volume')}</button>
							<div class="tracks-anchor">
								<button class="icon-button tracks-toggle" type="button" aria-label="${escape(t.playerDemo.tracksOpen)}" aria-expanded="true" aria-controls="tracks-panel" data-tracks-toggle>${icon('settings')}</button>
								<section class="tracks-panel" id="tracks-panel" aria-label="${escape(t.playerDemo.tracksTitle)}" data-tracks-panel>${trackOptions(t)}</section>
							</div>
						</div>
					</div>
				</div>
				<figcaption class="proof-caption">${escape(t.playerDemo.caption)}</figcaption>
				<noscript><p class="proof-caption">${escape(t.playerDemo.noJs)}</p></noscript>
			</figure>

			<section class="download-station" id="try" aria-labelledby="download-title">
				<div class="download-head">
					<div><p class="eyebrow">${escape(t.downloads.eyebrow)}</p><h2 id="download-title">${escape(t.downloads.title)}</h2></div>
					<p>${escape(t.downloads.lead)}</p>
				</div>
				<div class="os-switch" role="group" aria-label="${escape(t.downloads.title)}">
					${PLATFORMS.map((platform) => `<a class="os-control" href="#${platform.id}-download" role="button" aria-controls="${platform.id}-download" aria-pressed="false" data-os-control="${platform.id}">${escape(t.downloads.systems[platform.id])}</a>`).join('')}
				</div>
				<p class="os-status" data-os-status aria-live="polite">${escape(t.downloads.statusIdle)}</p>
				<noscript><p class="os-status">${escape(t.downloads.noJs)}</p></noscript>
				<div class="download-panels">${PLATFORMS.map((platform) => downloadPanel(t, release, platform)).join('')}</div>
			</section>
		</section>

		<section class="story" id="moments">
			<div class="shell">
				<header class="story-head"><div><p class="eyebrow">${escape(t.moments.eyebrow)}</p><h2 class="section-title">${escape(t.moments.title)}</h2></div><p class="section-lead">${escape(t.moments.lead)}</p></header>
				<ol class="moment-list">${moments(t, base)}</ol>
			</div>
		</section>

		<section class="difference" id="difference">
			<div class="shell difference-grid">
				<div class="difference-copy"><p class="eyebrow">${escape(t.difference.eyebrow)}</p><h2 class="section-title">${escape(t.difference.title)}</h2></div>
				<div class="difference-columns">
					<ul class="plain-list">${t.difference.items.map(([title, body]) => `<li><strong>${escape(title)}</strong><span>${escape(body)}</span></li>`).join('')}</ul>
					<div class="limits"><h3>${escape(t.difference.limitsTitle)}</h3><ul>${t.difference.limits.map((item) => `<li>${escape(item)}</li>`).join('')}</ul><a class="text-link" href="${REPO}#readme">${escape(t.difference.compare)} <span aria-hidden="true">↗</span></a></div>
					<article class="safety"><h3>${escape(t.difference.safetyTitle)}</h3><p>${escape(t.difference.safetyBody)}</p></article>
				</div>
			</div>
		</section>

		<section class="shell closing">
			<div><p class="eyebrow">${escape(t.closing.eyebrow)}</p><h2>${escape(t.closing.title)}</h2><p>${escape(t.closing.body)}</p></div>
			<a class="secondary-cta" href="#try">${escape(t.closing.cta)} <span aria-hidden="true">↑</span></a>
		</section>

		<section class="faq">
			<div class="shell faq-grid">
				<header><p class="eyebrow">${escape(t.faq.eyebrow)}</p><h2 class="section-title">${escape(t.faq.title)}</h2></header>
				<div class="faq-list">${t.faq.items.map(([question, answer]) => `<details data-faq><summary>${escape(question)}</summary><p>${escape(answer)}</p></details>`).join('')}</div>
			</div>
		</section>
	</main>

	<footer class="site-footer">
		<div class="shell footer-grid">
			<div><p>${escape(t.footer.body)}</p><p>${escape(t.footer.tmdb)}</p></div>
			<nav class="footer-links" aria-label="${escape(t.footer.navLabel)}"><a href="${REPO}">${escape(t.footer.source)} <span aria-hidden="true">↗</span></a><a href="${REPO}/blob/main/LICENSE">${escape(t.footer.license)} <span aria-hidden="true">↗</span></a></nav>
		</div>
	</footer>

	<script>
	(() => {
		const labels = ${clientLabels};
		const panels = [...document.querySelectorAll('[data-download-panel]')];
		const controls = [...document.querySelectorAll('[data-os-control]')];
		const status = document.querySelector('[data-os-status]');
		panels.forEach((panel) => { panel.hidden = true; });
		const selectSystem = (control) => {
			const id = control.dataset.osControl;
			controls.forEach((item) => item.setAttribute('aria-pressed', String(item === control)));
			panels.forEach((panel) => { panel.hidden = panel.dataset.downloadPanel !== id; });
			status.textContent = labels.statusSelected + ' ' + control.textContent.trim() + '.';
		};
		controls.forEach((control) => {
			control.addEventListener('click', (event) => { event.preventDefault(); selectSystem(control); });
			control.addEventListener('keydown', (event) => {
				if (event.key === ' ') { event.preventDefault(); selectSystem(control); }
			});
		});

		const trackToggle = document.querySelector('[data-tracks-toggle]');
		const trackPanel = document.querySelector('[data-tracks-panel]');
		const trackAnchor = trackToggle.closest('.tracks-anchor');
		const closeTracks = (restoreFocus) => {
			trackPanel.hidden = true;
			trackToggle.setAttribute('aria-expanded', 'false');
			if (restoreFocus) trackToggle.focus();
		};
		trackToggle.addEventListener('click', () => {
			const open = trackToggle.getAttribute('aria-expanded') === 'true';
			trackPanel.hidden = open;
			trackToggle.setAttribute('aria-expanded', String(!open));
		});
		trackPanel.addEventListener('keydown', (event) => {
			if (event.key === 'Escape') { event.preventDefault(); closeTracks(true); }
		});
		document.addEventListener('click', (event) => {
			if (!trackPanel.hidden && !trackAnchor.contains(event.target)) closeTracks(false);
		});

		const playToggle = document.querySelector('[data-play-toggle]');
		playToggle.addEventListener('click', () => {
			const playing = playToggle.getAttribute('aria-pressed') !== 'true';
			playToggle.setAttribute('aria-pressed', String(playing));
			playToggle.setAttribute('aria-label', playing ? labels.pause : labels.play);
		});

		const progress = document.querySelector('[data-player-progress]');
		const updateProgress = () => {
			const ratio = ((Number(progress.value) - Number(progress.min)) / (Number(progress.max) - Number(progress.min))) * 100;
			progress.style.setProperty('--progress', ratio + '%');
		};
		progress.addEventListener('input', updateProgress);
		document.querySelectorAll('[data-seek]').forEach((button) => {
			button.addEventListener('click', () => {
				progress.value = Math.max(Number(progress.min), Math.min(Number(progress.max), Number(progress.value) + Number(button.dataset.seek)));
				updateProgress();
			});
		});

		const muteToggle = document.querySelector('[data-mute-toggle]');
		muteToggle.addEventListener('click', () => {
			const muted = muteToggle.getAttribute('aria-pressed') !== 'true';
			muteToggle.setAttribute('aria-pressed', String(muted));
			muteToggle.setAttribute('aria-label', muted ? labels.unmute : labels.mute);
		});

		let busyTimer;
		document.querySelectorAll('[data-track]').forEach((option) => {
			option.addEventListener('click', () => {
				const type = option.dataset.track;
				document.querySelectorAll('[data-track="' + type + '"]').forEach((item) => item.setAttribute('aria-pressed', String(item === option)));
				if (type === 'quality') {
					const busy = document.querySelector('[data-player-busy]');
					window.clearTimeout(busyTimer);
					busy.hidden = false;
					busyTimer = window.setTimeout(() => { busy.hidden = true; }, 650);
				}
			});
		});
	})();
	</script>
</body>
</html>
`;
}
