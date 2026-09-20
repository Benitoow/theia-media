import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [react(), tailwindcss()],
	server: {
		fs: { allow: ['../../..'] },
	},
	build: {
		outDir: 'dist',
		emptyOutDir: true,
		target: 'chrome120',
	},
});
