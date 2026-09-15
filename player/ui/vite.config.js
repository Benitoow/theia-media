import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// The OSD is not a second interface with its own look: it imports the web
// application's token file and its icon component directly. Vite has to be
// told that reading outside this project's root is deliberate.
//
// The token file is written as a Tailwind `@theme` block, which the web
// application's build compiles into utilities. The OSD has no Tailwind, and a
// browser silently ignores an at-rule it does not recognise - everything
// inside it included. The tokens therefore arrived undefined and the interface
// fell back to browser defaults: no colour, no type scale, no durations, and a
// title bar in the platform's black over a film. Nothing failed. It simply
// looked wrong, which is worse.
//
// This turns the block into a plain `:root` for this build only. It runs as a
// PostCSS plugin rather than a Vite transform because `@import` is resolved by
// Vite after the transform hooks, so a transform never sees the imported file.
// `OnceExit` waits for the tree to be complete.
const tokensAsRoot = {
	postcssPlugin: 'theia-tokens-as-root',
	OnceExit(root, { postcss }) {
		root.walkAtRules('theme', (atRule) => {
			const rule = postcss.rule({ selector: ':root' });
			rule.append(atRule.nodes);
			atRule.replaceWith(rule);
		});
	},
};

export default defineConfig({
	plugins: [svelte()],
	css: {
		postcss: { plugins: [tokensAsRoot] },
	},
	server: {
		fs: { allow: ['../../..'] },
	},
	build: {
		outDir: 'dist',
		emptyOutDir: true,
		// The only engine that will ever load this bundle is WebView2, WKWebView
		// or WebKitGTK. No legacy targets, no polyfills.
		target: 'chrome120',
	},
});
