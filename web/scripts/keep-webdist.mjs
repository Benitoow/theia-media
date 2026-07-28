// Restores web-dist/.gitkeep after a build.
//
// The static adapter empties web-dist/ every time, and that placeholder is the
// only thing making the directory exist in a fresh clone -- which //go:embed
// requires, or the Go build fails before it starts. Losing it is invisible
// locally, where web-dist is full of a real build, and breaks CI on the next
// clean checkout. That happened once; hence this file.
//
// It runs from npm's postbuild hook rather than from the build scripts, so it
// cannot be bypassed by calling `npm run build` directly.

import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const dir = join(root, 'web-dist');

const contents = `This directory is filled by \`npm run build\` in web/ and embedded into the Go
binary via //go:embed. It is intentionally empty in version control -- the file
you are reading only exists so that the embed directive resolves on a fresh
clone, before the frontend has been built.

The SvelteKit static adapter wipes this directory on every build, so
web/scripts/keep-webdist.mjs recreates this file afterwards from npm's postbuild
hook. If it goes missing, \`go build\` still works locally but fails on a fresh
clone.
`;

mkdirSync(dir, { recursive: true });
writeFileSync(join(dir, '.gitkeep'), contents);
