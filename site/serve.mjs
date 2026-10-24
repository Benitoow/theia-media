#!/usr/bin/env node
//
// A static server for looking at site/dist. No dependencies.
//
//	node site/serve.mjs [port] [directory]  default 4180, site/dist
//
// It exists because the usual one-liners do not answer Range requests, and
// without them a browser cannot seek in the recording. It listens on the
// loopback address only. Port 8383 is the maintainer's own Theia and 8395 is the
// test port for the application; this page uses neither.

import { createReadStream, existsSync, readFileSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { dirname, extname, join, normalize, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const root = process.argv[3] ? resolve(process.argv[3]) : join(dirname(fileURLToPath(import.meta.url)), 'dist');
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
	'.webm': 'video/webm',
	'.vtt': 'text/vtt; charset=utf-8',
	'.woff2': 'font/woff2',
	'.txt': 'text/plain; charset=utf-8',
	'.xml': 'application/xml'
};

const compressed = new Map();
createServer((request, response) => {
	if (!['GET', 'HEAD'].includes(request.method)) {
		response.writeHead(405, { Allow: 'GET, HEAD' });
		return response.end();
	}
	let path;
	try { path = decodeURIComponent(new URL(request.url, 'http://localhost').pathname); }
	catch (_) { response.writeHead(400); return response.end(); }
	if (path.endsWith('/')) path += 'index.html';
	const file = join(root, normalize(path));
	if (!file.startsWith(root + sep) || !existsSync(file) || !statSync(file).isFile()) {
		response.writeHead(404, { 'Content-Type': 'text/plain' });
		return response.end('not found');
	}
	const size = statSync(file).size;
	const type = types[extname(file)] || 'application/octet-stream';
	response.setHeader('Cache-Control', 'no-cache');
	const range = request.headers.range && /^bytes=(\d*)-(\d*)$/.exec(request.headers.range);
	if (range) {
		const start = range[1] ? parseInt(range[1], 10) : Math.max(0, size - parseInt(range[2], 10));
		const end = range[2] ? Math.min(parseInt(range[2], 10), size - 1) : size - 1;
		const last = range[1] ? end : size - 1;
		if (!Number.isFinite(start) || start > last || start >= size) {
			response.writeHead(416, { 'Content-Range': `bytes */${size}` });
			return response.end();
		}
		response.writeHead(206, { 'Content-Type': type, 'Content-Range': `bytes ${start}-${last}/${size}`, 'Accept-Ranges': 'bytes', 'Content-Length': last - start + 1 });
		if (request.method === 'HEAD') return response.end();
		return createReadStream(file, { start, end: last }).pipe(response);
	}
	// Preview the same compressed text transfer that a static production host
	// can serve. Never compress ranges: video seeking keeps the original bytes.
	if (/\btext\/|javascript|json|xml/.test(type) && /\bgzip\b/.test(request.headers['accept-encoding'] || '')) {
		const stamp = `${statSync(file).mtimeMs}:${size}`;
		let cached = compressed.get(file);
		if (!cached || cached.stamp !== stamp) {
			cached = { stamp, bytes: gzipSync(readFileSync(file)) };
			compressed.set(file, cached);
		}
		response.writeHead(200, { 'Content-Type': type, 'Content-Encoding': 'gzip', Vary: 'Accept-Encoding', 'Content-Length': cached.bytes.length });
		return response.end(request.method === 'HEAD' ? undefined : cached.bytes);
	}
	response.writeHead(200, { 'Content-Type': type, 'Content-Length': size, 'Accept-Ranges': 'bytes' });
	if (request.method === 'HEAD') return response.end();
	createReadStream(file).pipe(response);
}).listen(port, '127.0.0.1', function () { console.log(`serve: ${root} on http://127.0.0.1:${this.address().port}/`); });
