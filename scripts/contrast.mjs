#!/usr/bin/env node
//
// Checks every colour token in docs/design-system.md against its documented
// contrast ratio, and against the WCAG minimum for the role it plays.
//
//   node scripts/contrast.mjs
//
// Exits non-zero if any token drifts, so a colour change cannot quietly break
// legibility. Run it whenever you touch a value in the palette.

const BACKGROUND = '#0B0A09'; // --ink

// [token, hex, documented ratio, minimum required, why that minimum]
const TOKENS = [
	['bone', '#EDE7DC', 16.07, 4.5, 'primary text'],
	['parchment', '#D6CFC2', 12.78, 4.5, 'body copy'],
	['muted', '#8C857A', 5.42, 4.5, 'labels and metadata'],
	['faint', '#5A544C', 2.64, 0, 'decorative and disabled only'],
	['accent', '#C8A24A', 8.22, 4.5, 'accent, used on text and focus rings'],
	['accent-bright', '#E3C173', 11.44, 4.5, 'accent hover'],
	['accent-dim', '#7A6330', 3.44, 3.0, 'borders and glows, never text'],
	['error', '#D06A5D', 5.56, 4.5, 'error text'],
	['warning', '#C9964A', 7.48, 4.5, 'warning text'],
];


// Text that does not sit on --ink.
//
// The table above measures every token against the page background, which is the
// right default and was quietly wrong for the first thing to put a label on a
// panel. A file badge is --muted on --surface, not on --ink: 5.12:1 rather than
// 5.42:1. Both clear AA, and the point is that nobody had checked -- which is
// the exact failure mode the header of this file describes.
//
// [what, foreground, background, documented ratio, minimum, role]
const PAIRS = [
	['badge label', '#8C857A', '#131211', 5.12, 4.5, 'file badges, set on a panel'],
	['rating scale', '#8C857A', '#0B0A09', 5.42, 4.5, 'the "/ 10" beside a rating'],
];
const channel = (c) => {
	c /= 255;
	return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
};

const luminance = (hex) => {
	const n = parseInt(hex.slice(1), 16);
	return (
		0.2126 * channel((n >> 16) & 255) +
		0.7152 * channel((n >> 8) & 255) +
		0.0722 * channel(n & 255)
	);
};

const contrast = (a, b) => {
	const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
	return (hi + 0.05) / (lo + 0.05);
};

let failed = 0;

console.log(`\ncontrast against ${BACKGROUND} (--ink)\n`);
console.log(`  ${'token'.padEnd(15)} ${'hex'.padEnd(9)} ${'documented'.padEnd(11)} ${'measured'.padEnd(9)} min`);
console.log(`  ${'-'.repeat(58)}`);

for (const [name, hex, documented, minimum, role] of TOKENS) {
	const measured = contrast(hex, BACKGROUND);

	const drifted = Math.abs(measured - documented) >= 0.05;
	const tooLow = minimum > 0 && measured < minimum;
	if (drifted || tooLow) failed++;

	const mark = tooLow ? 'FAIL' : drifted ? 'DRIFT' : 'ok';
	console.log(
		`  ${name.padEnd(15)} ${hex.padEnd(9)} ${documented.toFixed(2).padEnd(11)} ${measured.toFixed(2).padEnd(9)} ${minimum || '-'}\t${mark}`
	);

	if (tooLow) {
		console.log(`     ^ ${role} needs at least ${minimum}:1`);
	} else if (drifted) {
		console.log(`     ^ docs/design-system.md says ${documented}, update one or the other`);
	}
}


console.log(`\ncontrast on surfaces other than --ink\n`);
console.log(`  ${'what'.padEnd(15)} ${'on'.padEnd(9)} ${'documented'.padEnd(11)} ${'measured'.padEnd(9)} min`);
console.log(`  ${'-'.repeat(58)}`);

for (const [name, hex, background, documented, minimum, role] of PAIRS) {
	const measured = contrast(hex, background);

	const drifted = Math.abs(measured - documented) >= 0.05;
	const tooLow = minimum > 0 && measured < minimum;
	if (drifted || tooLow) failed++;

	const mark = tooLow ? 'FAIL' : drifted ? 'DRIFT' : 'ok';
	console.log(
		`  ${name.padEnd(15)} ${background.padEnd(9)} ${documented.toFixed(2).padEnd(11)} ${measured.toFixed(2).padEnd(9)} ${minimum || '-'}\t${mark}`
	);

	if (tooLow) console.log(`     ^ ${role} needs at least ${minimum}:1`);
	else if (drifted) console.log(`     ^ docs/design-system.md says ${documented}, update one or the other`);
}
if (failed > 0) {
	console.error(`\n${failed} token(s) need attention.\n`);
	process.exit(1);
}
console.log('\nall tokens match the documented ratios.\n');
