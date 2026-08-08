// Builds the presentation site into site/dist.
//
// Two pages from one template, so markup cannot land in one language and not
// the other, and a key present in one catalogue and missing from the other
// fails the build rather than printing "undefined" on a public page. That is
// the same guarantee web/scripts/check-locales.mjs gives the application.
//
//   node site/build.mjs
//
// No bundler, no dependency. Everything it copies already lives in the
// repository, licence checked: the wordmark, the screenshots, and the two
// fonts the application already ships under the SIL Open Font License.

import { mkdir, copyFile, writeFile, rm, readdir } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import fr from './locales/fr.js';
import en from './locales/en.js';
import { render } from './page.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');
const dist = join(here, 'dist');

/** Walks two catalogues in parallel and reports every key one of them lacks. */
function compare(a, b, path = '') {
	const problems = [];
	const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
	for (const key of keys) {
		const where = path ? `${path}.${key}` : key;
		if (!(key in a)) problems.push(`fr is missing ${where}`);
		else if (!(key in b)) problems.push(`en is missing ${where}`);
		else if (isPlainObject(a[key]) && isPlainObject(b[key])) {
			problems.push(...compare(a[key], b[key], where));
		} else if (Array.isArray(a[key]) && Array.isArray(b[key]) && a[key].length !== b[key].length) {
			problems.push(`${where} has ${a[key].length} entries in fr and ${b[key].length} in en`);
		}
	}
	return problems;
}

const isPlainObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);

// The two keys that are meant to differ, being what makes a catalogue itself.
const expectedDifferences = ['lang', 'dir', 'other'];
const problems = compare(fr, en).filter((p) => !expectedDifferences.some((k) => p.includes(k)));
if (problems.length) {
	console.error('Catalogues have drifted:\n  ' + problems.join('\n  '));
	process.exit(1);
}

await rm(dist, { recursive: true, force: true });
await mkdir(join(dist, 'en'), { recursive: true });
await mkdir(join(dist, 'shots'), { recursive: true });
await mkdir(join(dist, 'fonts'), { recursive: true });

await writeFile(join(dist, 'index.html'), render(fr));
await writeFile(join(dist, 'en', 'index.html'), render(en));
await copyFile(join(here, 'styles.css'), join(dist, 'styles.css'));

// Screenshots: whatever docs/screenshots holds, so adding one to the README
// and adding one to the site is the same act.
const shots = join(root, 'docs', 'screenshots');
for (const name of await readdir(shots)) {
	if (name.endsWith('.webp')) await copyFile(join(shots, name), join(dist, 'shots', name));
}

const assets = [
	[join(root, 'assets', 'social-preview.png'), join(dist, 'social-preview.png')],
	[join(root, 'web', 'static', 'favicon.svg'), join(dist, 'favicon.svg')],
	[join(root, 'web', 'static', 'apple-touch-icon.png'), join(dist, 'apple-touch-icon.png')],
	[
		join(root, 'web', 'node_modules', '@fontsource-variable', 'inter', 'files', 'inter-latin-wght-normal.woff2'),
		join(dist, 'fonts', 'inter.woff2')
	],
	[
		join(
			root,
			'web',
			'node_modules',
			'@fontsource-variable',
			'playfair-display',
			'files',
			'playfair-display-latin-wght-normal.woff2'
		),
		join(dist, 'fonts', 'playfair-display.woff2')
	]
];

for (const [from, to] of assets) {
	if (!existsSync(from)) {
		// The fonts come from web/node_modules. A site build without `npm ci` in
		// web/ would otherwise ship a page that silently falls back to Georgia.
		console.error(`Missing asset: ${from}`);
		process.exit(1);
	}
	await copyFile(from, to);
}

// GitHub Pages would otherwise run the output through Jekyll, which ignores
// directories beginning with an underscore and is pure overhead here.
await writeFile(join(dist, '.nojekyll'), '');

console.log(`Site built into site/dist (fr + en, ${(await readdir(join(dist, 'shots'))).length} screenshots).`);
