import adapter from '@sveltejs/adapter-static';
import { readFileSync } from 'node:fs';

// SvelteKit's default version name is the build's own timestamp, and it lands
// in `web-dist/_app/version.json` *and* inside one of the client chunks, so the
// bytes of the built interface depend on when it was built. Measured on the two
// v3.3.3 tags: nothing in the Go source had moved and eight of the ten published
// assets were still not byte-identical (`{"version":"1789993574098"}` then
// `1790005895723`), which makes a digest comparison between two builds of one
// tree meaningless. The name is the package's own version instead, which a
// release bumps on purpose - so two builds of one tree agree to the byte.
const { version } = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8'));

/** @type {import('@sveltejs/kit').Config} */
export default {
	kit: {
		version: { name: version },
		// The build lands in web-dist/ at the repository root, where the Go
		// binary picks it up with //go:embed. Nothing else reads this directory.
		adapter: adapter({
			pages: '../web-dist',
			assets: '../web-dist',
			// Single-page app: every unknown path is served index.html and the
			// client-side router takes over. The Go server mirrors this rule.
			fallback: 'index.html',
			// Nothing is prerendered, which is exactly what we want and what
			// strict mode would otherwise complain about.
			strict: false
		})
	}
};
