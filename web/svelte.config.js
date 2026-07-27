import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
export default {
	kit: {
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
