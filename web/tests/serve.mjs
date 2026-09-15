// Starts the built binary against a throwaway data directory, so the guard has
// something to point a browser at.
//
// It deliberately does not build anything. A guard that rebuilds is a guard
// people stop running; `.\build.ps1` or `make build` comes first, and this
// fails with that instruction if the binary is not there.

import { spawn } from 'node:child_process';
import { existsSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..', '..');
const binary = join(root, process.platform === 'win32' ? 'theia-server.exe' : 'theia-server');
const stale = join(root, process.platform === 'win32' ? 'theia.exe' : 'theia');

if (!existsSync(binary)) {
	console.error(
		`No binary at ${binary}.\n` +
			`Build one first:  .\\build.ps1   (Windows)  or  make build   (macOS, Linux)`
	);
	// A pre-V3.3 binary left at the root is worse than no binary at all: it runs,
	// and the guard then reports on code that is not the code being changed.
	if (existsSync(stale)) {
		console.error(
			`\nThere is a ${stale.split(/[\\/]/).pop()} at the root. That is the name the server ` +
				`had before V3.3 (decision 119) and it is stale by definition: delete it, or it will ` +
				`be run by anything that still looks for it.`
		);
	}
	process.exit(1);
}

// A fresh directory every run: the guard must never read, and can never write,
// somebody's real library.
const dataDir = mkdtempSync(join(tmpdir(), 'theia-guard-'));
const port = process.env.THEIA_TEST_PORT ?? '8396';
writeFileSync(join(dataDir, 'config.json'), JSON.stringify({ hostname: 'theia-layout-guard', library_paths: [] }));

const child = spawn(binary, ['--data-dir', dataDir, '--port', port], {
	stdio: 'inherit', cwd: dataDir
});

child.on('exit', (code) => process.exit(code ?? 0));
for (const signal of ['SIGINT', 'SIGTERM']) {
	process.on(signal, () => child.kill(signal));
}
