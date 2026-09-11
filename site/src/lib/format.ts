// Pure formatters, safe to bundle into client islands. The server-only
// release loader stays in release.ts: pulling node:fs into the browser bundle
// is exactly what this split exists to prevent.

export function formatSize(bytes?: number): string {
	if (bytes === undefined || !Number.isFinite(bytes) || bytes <= 0) return '';
	return `${new Intl.NumberFormat('en-US', {
		minimumFractionDigits: 1,
		maximumFractionDigits: 1
	}).format(bytes / 1_000_000)} MB`;
}

export function formatDate(value?: string): string {
	if (!value) return '';
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return '';
	return new Intl.DateTimeFormat('en-US', {
		day: 'numeric',
		month: 'long',
		year: 'numeric',
		timeZone: 'UTC'
	}).format(date);
}
