#!/usr/bin/env node
//
// A static server for looking at site/dist. No dependencies.
//
//	node site/serve.mjs [port]        default 4180, serves site/dist
//
// It exists because the usual one-liners do not answer Range requests, and
// without them a browser cannot seek in the recording. It listens on the
// loopback address only. Port 8383 is the maintainer's own Theia and 8395 is the
// test port for the application; this page uses neither.

import { createReadStream, existsSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { dirname, extname, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), 'dist');
const port = Number(process.argv[2] || 4180);
if (!existsSync(join(root, 'index.html'))) {
	console.error('serve: site/dist is empty; run node site/build.mjs first.');
	process.exit(1);
}

const types = {
	'.html': 'text/html; charset=utf-8',
	'.css': 'text/css; charset=utf-8',
	'.js': 'text/javascript; charset=utf-8',
	'.json': 'application/json',
	'.webp': 'image/webp',
	'.png': 'image/png',
	'.jpg': 'image/jpeg',
	'.svg': 'image/svg+xml',
	'.mp4': 'video/mp4',
	'.vtt': 'text/vtt; charset=utf-8',
	'.woff2': 'font/woff2',
	'.txt': 'text/plain; charset=utf-8',
	'.xml': 'application/xml'
};

createServer((request, response) => {
	let path = decodeURIComponent(new URL(request.url, 'http://localhost').pathname);
	if (path.endsWith('/')) path += 'index.html';
	const file = join(root, normalize(path));
	if (!file.startsWith(root) || !existsSync(file) || !statSync(file).isFile()) {
		response.writeHead(404, { 'Content-Type': 'text/plain' });
		return response.end('not found');
	}
	const size = statSync(file).size;
	const type = types[extname(file)] || 'application/octet-stream';
	const range = request.headers.range && /bytes=(\d*)-(\d*)/.exec(request.headers.range);
	if (range) {
		const start = range[1] ? parseInt(range[1], 10) : 0;
		const end = range[2] ? Math.min(parseInt(range[2], 10), size - 1) : size - 1;
		response.writeHead(206, { 'Content-Type': type, 'Content-Range': `bytes ${start}-${end}/${size}`, 'Accept-Ranges': 'bytes', 'Content-Length': end - start + 1 });
		return createReadStream(file, { start, end }).pipe(response);
	}
	response.writeHead(200, { 'Content-Type': type, 'Content-Length': size, 'Accept-Ranges': 'bytes' });
	createReadStream(file).pipe(response);
}).listen(port, '127.0.0.1', () => console.log(`serve: ${root} on http://127.0.0.1:${port}/`));
