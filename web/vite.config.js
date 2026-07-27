import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		// `npm run dev` serves the UI with hot reload while forwarding API calls
		// to a Theia binary running alongside it, so the frontend can be worked
		// on without rebuilding Go.
		proxy: {
			'/api': 'http://localhost:8383'
		}
	}
});
