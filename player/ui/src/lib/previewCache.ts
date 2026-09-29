import { invoke } from './bridge';

const MAX_BYTES = 24 * 1024 * 1024;
const MAX_CLIPS = 12;
const cache = new Map<string, { url: string; size: number }>();
const pending = new Map<string, Promise<Answer>>();
const waiting: Array<() => void> = [];
let active = 0, bytes = 0, generation = 0;
type Answer = { state?: string; url?: string };
async function permit() { if (active < 2) { active++; return; } await new Promise<void>((resolve) => waiting.push(resolve)); }
function release() { const next = waiting.shift(); if (next) next(); else active--; }
export function clearPreviews() { generation++; for (const clip of cache.values()) URL.revokeObjectURL(clip.url); cache.clear(); bytes = 0; pending.clear(); }
export async function preview(kind: string, id: number): Promise<Answer> {
    const key = `${kind}:${id}`, found = cache.get(key);
    if (found) { cache.delete(key); cache.set(key, found); return { state: 'ready', url: found.url }; }
    const existing = pending.get(key); if (existing) return existing;
    const scope = generation;
    const request = (async () => {
        await permit();
        try {
            if (scope !== generation) return {};
            const answer = JSON.parse(await invoke<string>('player_preview', { kind, id })) as { state?: string; data_url?: string };
            if (scope !== generation) return {};
            if (answer.state !== 'ready' || !answer.data_url) return { state: answer.state };
            const match = /^data:video\/mp4;base64,([A-Za-z0-9+/=]+)$/.exec(answer.data_url);
            if (!match || match[1].length > 12 * 1024 * 1024) throw new Error('invalid preview');
            const binary = atob(match[1]), data = Uint8Array.from(binary, (character) => character.charCodeAt(0));
            const size = data.byteLength, url = URL.createObjectURL(new Blob([data], { type: 'video/mp4' }));
            cache.set(key, { url, size }); bytes += size;
            while (bytes > MAX_BYTES || cache.size > MAX_CLIPS) {
                const oldest = cache.entries().next().value; if (!oldest) break;
                cache.delete(oldest[0]); bytes -= oldest[1].size; URL.revokeObjectURL(oldest[1].url);
            }
            return { state: 'ready', url };
        } finally { release(); }
    })();
    pending.set(key, request);
    try { return await request; } finally { if (pending.get(key) === request) pending.delete(key); }
}
export function cachedPreview(kind: string, id: number) { return cache.get(`${kind}:${id}`)?.url; }
export function previewUsage() { return { clips: cache.size, bytes, active, waiting: waiting.length }; }
