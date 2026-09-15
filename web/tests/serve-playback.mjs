import { spawn, spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
const root = resolve(import.meta.dirname, '../..');
const data = mkdtempSync(join(tmpdir(), 'theia-playback-guard-'));
const binary = join(root, process.platform === 'win32' ? 'theia-server.exe' : 'theia-server');
// The same trap serve.mjs documents: a pre-V3.3 binary at the root is not this
// tree, and a guard that runs it reports on code nobody is changing.
if (!existsSync(binary)) {
	console.error(`No binary at ${binary}. Build one first: .\\build.ps1  or  make build`);
	process.exit(1);
}
const args = ['run', './internal/testfixture', '--data-dir', data];
if (process.env.THEIA_TEST_FFMPEG) args.push('--ffmpeg', process.env.THEIA_TEST_FFMPEG);
const seed = spawnSync(process.env.GO_BINARY || 'go', args, { cwd: root, stdio: 'inherit', timeout: 600_000 });
if (seed.status !== 0) process.exit(seed.status || 1);
const server = spawn(binary, ['--data-dir', data, '--port', '8397'], { cwd: data, stdio: 'inherit' });
server.on('exit', code => process.exit(code || 0));
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => server.kill(signal));
