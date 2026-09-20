export {};

declare global {
	interface Window {
		__TAURI__?: {
			core?: { invoke?: <T = unknown>(command: string, args?: Record<string, unknown>) => Promise<T> };
			event?: { listen?: (event: string, handler: (event: { payload: unknown }) => void) => Promise<() => void> };
			window?: { getCurrentWindow?: () => any };
		};
	}
}
