// The page, as a function of a catalogue. Both languages render from this one
// template, so a change to the markup cannot land in one language and not the
// other.

const REPO = 'https://github.com/Benitoow/theia-media';
const LATEST = `${REPO}/releases/latest/download`;

// Platform names are the same in both languages, so they live here rather than
// in the catalogues. `match` is what the download button uses to guess, and it
// is only ever a guess: the full table is always on the page.
export const PLATFORMS = [
	{ id: 'windows-amd64', os: 'Windows', arch: 'x64', file: 'theia-windows-amd64.exe', steps: 'windows' },
	{ id: 'windows-arm64', os: 'Windows', arch: 'ARM64', file: 'theia-windows-arm64.exe', steps: 'windows' },
	{ id: 'darwin-arm64', os: 'macOS', arch: 'Apple Silicon', file: 'theia-darwin-arm64', steps: 'macos' },
	{ id: 'darwin-amd64', os: 'macOS', arch: 'Intel', file: 'theia-darwin-amd64', steps: 'macos' },
	{ id: 'linux-amd64', os: 'Linux', arch: 'x64', file: 'theia-linux-amd64', steps: 'linux' },
	{ id: 'linux-arm64', os: 'Linux', arch: 'ARM64', file: 'theia-linux-arm64', steps: 'linux' }
];

const escape = (s) =>
	String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// Catalogue strings carry inline <code> and <em> on purpose; escaping them
// would print the tags. Only attribute values and plain text go through
// escape().
const raw = (s) => String(s);

export function render(t) {
	const base = t.dir ? '../' : './';
	const s = (name, key) => `
			<figure class="shot">
				<img src="${base}shots/${name}.webp" alt="${escape(t.shots[key])}" width="1600" height="900" loading="lazy" decoding="async">
				<figcaption>${escape(t.shots[key])}</figcaption>
			</figure>`;

	return `<!doctype html>
<html lang="${t.lang}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${escape(t.title)}</title>
<meta name="description" content="${escape(t.description)}">
<meta name="color-scheme" content="dark">
<meta name="theme-color" content="#0b0a09">
<link rel="icon" href="${base}favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="${base}apple-touch-icon.png">
<link rel="canonical" href="https://benitoow.github.io/theia-media/${t.dir}">
<link rel="alternate" hreflang="fr" href="https://benitoow.github.io/theia-media/">
<link rel="alternate" hreflang="en" href="https://benitoow.github.io/theia-media/en/">
<meta property="og:type" content="website">
<meta property="og:title" content="${escape(t.title)}">
<meta property="og:description" content="${escape(t.description)}">
<meta property="og:image" content="https://benitoow.github.io/theia-media/social-preview.png">
<meta name="twitter:card" content="summary_large_image">
<link rel="preload" href="${base}fonts/playfair-display.woff2" as="font" type="font/woff2" crossorigin>
<link rel="preload" href="${base}fonts/inter.woff2" as="font" type="font/woff2" crossorigin>
<link rel="stylesheet" href="${base}styles.css">
</head>
<body>
<a class="skip" href="#main">${t.lang === 'fr' ? 'Aller au contenu' : 'Skip to content'}</a>

<header class="nav">
	<div class="nav-inner">
		<a class="wordmark" href="${base}${t.dir}">Theia</a>
		<nav aria-label="${escape(t.nav.features)}">
			<a href="#what">${escape(t.nav.features)}</a>
			<a href="#shots">${escape(t.nav.shots)}</a>
			<a href="#download">${escape(t.nav.download)}</a>
			<a href="${REPO}" rel="noopener">${escape(t.nav.github)}</a>
			<a class="lang" href="${t.other.href}" hreflang="${t.other.code}" lang="${t.other.code}">${escape(t.other.label)}</a>
		</nav>
	</div>
</header>

<main id="main">

<section class="hero">
	<p class="kicker">${escape(t.hero.kicker)}</p>
	<h1>${escape(t.hero.title).replace(/\n/g, '<br>')}</h1>
	<p class="lede">${escape(t.hero.lede)}</p>
	<div class="actions">
		<a class="button primary" id="primary-download" href="#download" data-fallback-label="${escape(t.hero.cta)}">${escape(t.hero.cta)}</a>
		<a class="button" href="${REPO}" rel="noopener">${escape(t.hero.source)}</a>
	</div>
	<p class="meta">${escape(t.hero.meta)}</p>
</section>

<section class="band" id="what">
	<h2 class="section-title">${escape(t.claims.title)}</h2>
	<div class="claims">
		${t.claims.items
			.map(
				(c) => `<article class="claim">
			<p class="kicker">${escape(c.k)}</p>
			<h3>${escape(c.t)}</h3>
			<p>${escape(c.d)}</p>
		</article>`
			)
			.join('\n\t\t')}
	</div>
</section>

<section class="band" id="shots">
	<h2 class="section-title">${escape(t.shots.title)}</h2>
	<p class="section-lede">${escape(t.shots.lede)}</p>
	${s('home', 'home')}
	<div class="shot-pair">
		${s('library', 'library')}
		${s('series', 'series')}
	</div>
	<div class="shot-pair">
		${s('serie', 'serie')}
		${s('settings', 'settings')}
	</div>
	<div class="shot-pair">
		${s('onboarding', 'onboarding')}
		${s('profiles', 'profiles')}
	</div>
</section>

<section class="band split">
	<div>
		<h2 class="section-title">${escape(t.does.title)}</h2>
		${t.does.groups
			.map(
				(g) => `<div class="group">
			<h3>${escape(g.t)}</h3>
			<ul>${g.items.map((i) => `<li>${escape(i)}</li>`).join('')}</ul>
		</div>`
			)
			.join('\n\t\t')}
	</div>
	<div>
		<h2 class="section-title">${escape(t.refuses.title)}</h2>
		<p class="section-lede">${escape(t.refuses.lede)}</p>
		<dl class="refuses">
			${t.refuses.items.map(([k, v]) => `<dt>${escape(k)}</dt><dd>${escape(v)}</dd>`).join('\n\t\t\t')}
		</dl>
	</div>
</section>

<section class="band">
	<h2 class="section-title">${escape(t.numbers.title)}</h2>
	<p class="section-lede">${escape(t.numbers.lede)}</p>
	<div class="numbers">
		${t.numbers.items
			.map(([n, l]) => `<div class="number"><strong>${escape(n)}</strong><span>${escape(l)}</span></div>`)
			.join('\n\t\t')}
	</div>
</section>

<section class="band" id="download">
	<h2 class="section-title">${escape(t.download.title)}</h2>
	<p class="section-lede">${escape(t.download.lede)}</p>

	<ul class="downloads">
		${PLATFORMS.map(
			(p) => `<li data-platform="${p.id}">
			<a href="${LATEST}/${p.file}" download>
				<span class="os">${p.os}</span>
				<span class="arch">${p.arch}</span>
				<span class="file">${p.file}</span>
				<span class="badge" hidden>${escape(t.download.detected)}</span>
			</a>
		</li>`
		).join('\n\t\t')}
	</ul>

	<p class="fineprint">${escape(t.download.digest)}</p>

	<h3 class="steps-title">${escape(t.download.steps.title)}</h3>
	<div class="steps">
		<div><h4>Windows</h4><ol>${t.download.steps.windows.map((x) => `<li>${raw(x)}</li>`).join('')}</ol></div>
		<div><h4>macOS</h4><ol>${t.download.steps.macos.map((x) => `<li>${raw(x)}</li>`).join('')}</ol></div>
		<div><h4>Linux</h4><ol>${t.download.steps.linux.map((x) => `<li>${raw(x)}</li>`).join('')}</ol></div>
	</div>

	<aside class="warning">
		<h3>${escape(t.download.warning.title)}</h3>
		<p>${escape(t.download.warning.body)}</p>
	</aside>
</section>

<section class="band">
	<h2 class="section-title">${escape(t.compare.title)}</h2>
	<p class="section-lede">${escape(t.compare.lede)}</p>
	<div class="table-scroll">
		<table>
			<thead><tr>${t.compare.head.map((h, i) => `<th${i === 0 ? ' scope="col"' : ' scope="col"'}>${escape(h)}</th>`).join('')}</tr></thead>
			<tbody>
				${t.compare.rows
					.map(
						(r) =>
							`<tr><th scope="row">${escape(r[0])}</th>${r
								.slice(1)
								.map((c, i) => `<td${i === 0 ? ' class="ours"' : ''}>${escape(c)}</td>`)
								.join('')}</tr>`
					)
					.join('\n\t\t\t\t')}
			</tbody>
		</table>
	</div>
	<p class="fineprint">${escape(t.compare.note)}</p>
</section>

</main>

<footer class="footer">
	<p class="wordmark-small">Theia</p>
	<nav aria-label="${escape(t.nav.github)}">
		<a href="${REPO}" rel="noopener">${escape(t.footer.links.repo)}</a>
		<a href="${REPO}/releases" rel="noopener">${escape(t.footer.links.releases)}</a>
		<a href="${REPO}/issues/new/choose" rel="noopener">${escape(t.footer.links.issues)}</a>
		<a href="${REPO}/blob/main/.github/SECURITY.md" rel="noopener">${escape(t.footer.links.security)}</a>
	</nav>
	<p>${escape(t.footer.licence)}</p>
	<p>${escape(t.footer.tmdb)}</p>
	<p class="fineprint">${escape(t.footer.credits)}</p>
</footer>

<script>
// Progressive enhancement, nothing more: the six downloads are in the markup
// and work without this. All it does is promote the likely one.
(function () {
	var ua = navigator.userAgent;
	var p = navigator.userAgentData && navigator.userAgentData.platform;
	var id = null;
	if (/Win/.test(ua) || p === 'Windows') id = /ARM|aarch64/i.test(ua) ? 'windows-arm64' : 'windows-amd64';
	else if (/Mac/.test(ua) || p === 'macOS') id = 'darwin-arm64';
	else if (/Linux|X11/.test(ua) || p === 'Linux') id = /aarch64|arm64/i.test(ua) ? 'linux-arm64' : 'linux-amd64';
	if (!id) return;
	var row = document.querySelector('[data-platform="' + id + '"]');
	if (!row) return;
	row.classList.add('detected');
	var badge = row.querySelector('.badge');
	if (badge) badge.hidden = false;
	var link = row.querySelector('a');
	var cta = document.getElementById('primary-download');
	if (cta && link) {
		cta.href = link.href;
		cta.setAttribute('download', '');
		cta.textContent = cta.dataset.fallbackLabel + ' · ' + row.querySelector('.os').textContent;
	}
})();
</script>
</body>
</html>
`;
}
