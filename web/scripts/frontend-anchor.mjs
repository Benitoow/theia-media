// Prints the aggregate SHA-256 of everything under web/src and web/tests:
// the fingerprint that proves a backend or tooling chantier left the
// frontend exactly as it found it.
//
// The method is this file rather than a sentence in a document: walk both
// trees, sort every file by its web-relative path, hash each file, then hash
// the concatenation of (path, in web-relative forward-slash form, followed by
// that file's digest). One command, one line out, no prose to drift:
//
//   node web/scripts/frontend-anchor.mjs
//
// The value it replaced (recorded in docs/plan-refonte-lecture.md for the
// 2026-09-10 modernisation) could not be reproduced from its written
// description -- the aggregation was under-specified. This script is the
// specification now.
import { createHash } from 'node:crypto';
import { readdirSync, readFileSync } from 'node:fs';
import { dirname, join, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const webRoot = join(dirname(fileURLToPath(import.meta.url)), '..');

function walk(dir, acc = []) {
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const path = join(dir, entry.name);
		if (entry.isDirectory()) walk(path, acc);
		else acc.push(path);
	}
	return acc;
}

const files = [...walk(join(webRoot, 'src')), ...walk(join(webRoot, 'tests'))]
	.map((path) => relative(webRoot, path).split(sep).join('/'))
	.sort();

const blob = Buffer.concat(
	files.flatMap((path) => [
		Buffer.from(path, 'utf8'),
		createHash('sha256').update(readFileSync(join(webRoot, path))).digest(),
	]),
);

console.log(files.length, 'files');
console.log(createHash('sha256').update(blob).digest('hex').toUpperCase());
