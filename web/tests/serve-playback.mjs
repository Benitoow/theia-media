import { spawn, spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readdirSync, statSync } from 'node:fs';
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
// The suite drives the *built* interface, not the sources: the server serves
// web-dist. Editing web/src and running this without a build tests the previous
// frontend and reports on code nobody is changing - which is exactly what
// happened on 20 September 2026, when a fixed write path was measured as still
// broken because the bundle predated it. Cheap to check, expensive to miss.
{
	const built = join(root, 'web-dist', 'index.html');
	const sources = join(root, 'web', 'src');
	const newest = (dir) => {
		let latest = 0;
		const walk = (path) => {
			for (const entry of readdirSync(path, { withFileTypes: true })) {
				const child = join(path, entry.name);
				if (entry.isDirectory()) walk(child);
				else latest = Math.max(latest, statSync(child).mtimeMs);
			}
		};
		walk(dir);
		return latest;
	};
	if (!existsSync(built)) {
		console.error(`No built interface at ${built}. Build one first: .\\build.ps1`);
		process.exit(1);
	}
	if (existsSync(sources) && newest(sources) > statSync(built).mtimeMs) {
		console.error(
			`web/src is newer than ${built}: this suite would test the interface as it was before ` +
				`those edits. Run .\\build.ps1 first.`
		);
		process.exit(1);
	}
}

const args = ['run', './internal/testfixture', '--data-dir', data];
if (process.env.THEIA_TEST_FFMPEG) args.push('--ffmpeg', process.env.THEIA_TEST_FFMPEG);
const seed = spawnSync(process.env.GO_BINARY || 'go', args, { cwd: root, stdio: 'inherit', timeout: 600_000 });
// A missing `go` is not a non-zero status: spawnSync returns `status: null` with
// `error` set, and the bare `status !== 0` test below then exited 1 saying
// nothing at all - a guard that fails silently is a guard nobody can act on.
if (seed.error) {
	console.error(
		`Could not run ${process.env.GO_BINARY || 'go'}: ${seed.error.message}\n` +
			`The playback fixtures are seeded with Go. Put it on PATH, or set GO_BINARY to its full path.`
	);
	process.exit(1);
}
if (seed.status !== 0) process.exit(seed.status || 1);
const server = spawn(binary, ['--data-dir', data, '--port', '8397'], { cwd: data, stdio: 'inherit' });
server.on('exit', code => process.exit(code || 0));
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => server.kill(signal));
