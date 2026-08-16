// Starts the built binary against a throwaway data directory, so the guard has
// something to point a browser at.
//
// It deliberately does not build anything. A guard that rebuilds is a guard
// people stop running; `.\build.ps1` or `make build` comes first, and this
// fails with that instruction if the binary is not there.

import { spawn } from 'node:child_process';
import { existsSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..');
const binary = join(root, process.platform === 'win32' ? 'theia.exe' : 'theia');

if (!existsSync(binary)) {
	console.error(
		`No binary at ${binary}.\n` +
			`Build one first:  .\\build.ps1   (Windows)  or  make build   (macOS, Linux)`
	);
	process.exit(1);
}

// A fresh directory every run: the guard must never read, and can never write,
// somebody's real library.
const dataDir = mkdtempSync(join(tmpdir(), 'theia-guard-'));
const port = process.env.THEIA_TEST_PORT ?? '8396';

const child = spawn(binary, ['--data-dir', dataDir, '--port', port], {
	stdio: 'inherit'
});

child.on('exit', (code) => process.exit(code ?? 0));
for (const signal of ['SIGINT', 'SIGTERM']) {
	process.on(signal, () => child.kill(signal));
}
