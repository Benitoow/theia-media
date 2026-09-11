import react from '@astrojs/react';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'astro/config';

// The site is published at a repository subpath on GitHub Pages. Asset
// references in the templates stay relative, so they resolve identically
// under the subpath, from a local preview, and from a Pages preview.
export default defineConfig({
	site: 'https://benitoow.github.io',
	base: '/theia-media',
	trailingSlash: 'ignore',
	integrations: [react()],
	vite: {
		plugins: [tailwindcss()]
	}
});
