#!/usr/bin/env node
//
// Every backdrop the navigation bar floats over declares where it is framed.
//
//   node scripts/check-backdrops.mjs
//
// The fault this exists for was reported twice, on two different components,
// and fixed once. A full-bleed picture with no object-position defaults to
// centring, which on a 16/9 still inside a much wider header buries the subject
// behind the navigation pill: measured at 169px of headroom hidden on the film
// header and 223px on the series one, under a bar 128px deep.
//
// Writing the rule down did not stop it happening again -- the second occurrence
// shipped after the first was documented -- so it is checked here instead.
//
// Two idioms are covered, because the application uses both:
//   - Tailwind utilities on the element, which is how the three detail and hero
//     backdrops are written.
//   - A class in app.css setting object-fit: cover, which is how .chrome-scene
//     is written.

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative, resolve, sep } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const source = join(root, 'src');

// Exempt elements, each with the reason it is exempt. An entry here is a
// decision, not a silencer: if you add one, say why.
const exempt = [
	{
		match: /decoding="async"/,
		why: 'posters and portraits are not full-bleed and nothing floats over them'
	}
];

/** Utilities that answer "where is this picture framed". */
const positioned =
	/\bobject-(top|bottom|left|right|center|left-top|left-bottom|right-top|right-bottom|\[[^\]]+\])/;

function walk(dir) {
	const out = [];
	for (const entry of readdirSync(dir)) {
		const path = join(dir, entry);
		if (statSync(path).isDirectory()) out.push(...walk(path));
		else if (entry.endsWith('.svelte')) out.push(path);
	}
	return out;
}

const failures = [];
const checked = [];

// --- the markup half -------------------------------------------------------
for (const file of walk(source)) {
	const text = readFileSync(file, 'utf8');
	const lines = text.split('\n');

	// Every class attribute in the file, with the line it starts on.
	const pattern = /class="([^"]*)"/g;
	let match;
	while ((match = pattern.exec(text)) !== null) {
		const classes = match[1];
		// Full-bleed means pinned to every edge and covering. Anything else is a
		// picture inside a box somebody sized, and its framing is its own affair.
		const fullBleed =
			/\babsolute\b/.test(classes) && /\binset-0\b/.test(classes) && /\bobject-cover\b/.test(classes);
		if (!fullBleed) continue;

		const line = text.slice(0, match.index).split('\n').length;
		const context = lines.slice(Math.max(0, line - 6), line).join('\n');
		const excused = exempt.find((rule) => rule.match.test(context) || rule.match.test(classes));
		const where = `${relative(root, file).split(sep).join('/')}:${line}`;

		if (excused) {
			checked.push([where, 'exempt', excused.why]);
			continue;
		}
		if (positioned.test(classes)) {
			checked.push([where, 'framed', classes.match(positioned)[0]]);
			continue;
		}
		checked.push([where, 'CENTRED', 'no object-position: the bar will cover the subject']);
		failures.push(where);
	}
}

// --- the stylesheet half ---------------------------------------------------
const css = readFileSync(join(source, 'app.css'), 'utf8');
for (const block of css.split('}')) {
	if (!/object-fit:\s*cover/.test(block)) continue;
	if (!/position:\s*absolute/.test(block) && !/inset:\s*0/.test(block)) continue;

	const selector = (block.match(/([.#][\w-]+)\s*\{/) ?? [, '(unnamed rule)'])[1];
	const where = `web/src/app.css  ${selector}`;
	if (/object-position:/.test(block)) {
		checked.push([where, 'framed', block.match(/object-position:\s*([^;]+)/)[1].trim()]);
	} else {
		checked.push([where, 'CENTRED', 'no object-position: the bar will cover the subject']);
		failures.push(where);
	}
}

console.log('\nfull-bleed pictures and where they are framed\n');
for (const [where, state, note] of checked) {
	console.log(`  ${state.padEnd(8)} ${where.padEnd(46)} ${note}`);
}

if (failures.length > 0) {
	console.error(
		`\n${failures.length} full-bleed picture(s) with no framing.\n` +
			`Add an object-position utility -- object-top for anything under the ` +
			`navigation bar -- or\nexempt it in scripts/check-backdrops.mjs with the reason.\n`
	);
	process.exit(1);
}
console.log(`\n${checked.length} full-bleed pictures, all framed.\n`);
