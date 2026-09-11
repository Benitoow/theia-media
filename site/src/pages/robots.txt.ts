// Build-time endpoint; see sitemap.xml.ts for why this is not in public/.

import type { APIRoute } from 'astro';

const SITE = 'https://benitoow.github.io/theia-media';

export const GET: APIRoute = () =>
	new Response(`User-agent: *\nAllow: /\nSitemap: ${SITE}/sitemap.xml\n`, {
		headers: { 'Content-Type': 'text/plain; charset=utf-8' }
	});
