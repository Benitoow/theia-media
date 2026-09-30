import { useEffect, useRef, useState } from 'react';
import { invoke } from './bridge';
import type { MediaFile } from '../types';

/** An inspection response belongs to its detail, even after navigation. */
export function useFileInspection(kind: 'movie' | 'episode', id: number, files: MediaFile[]) {
 const generation = useRef(0);
 const inFlight = useRef(false);
 const [measured, setMeasured] = useState<Record<number, MediaFile>>({});
 const [busy, setBusy] = useState(false);
 const [error, setError] = useState(false);
 useEffect(() => {
  generation.current++; inFlight.current = false;
  setMeasured({}); setBusy(false); setError(false);
  return () => { generation.current++; };
 }, [kind, id]);
 const inspect = async (fileId: number) => {
  if (inFlight.current) return;
  const request = generation.current;
  inFlight.current = true; setBusy(true); setError(false);
  try {
   const file = JSON.parse(await invoke<string>('player_inspect_file', { kind, id, fileId })) as MediaFile;
   if (file.id !== fileId || file.media?.status !== 'ok') throw new Error('invalid inspection response');
   if (generation.current === request) setMeasured((previous) => ({ ...previous, [fileId]: file }));
  } catch {
   if (generation.current === request) setError(true);
  } finally {
   if (generation.current === request) { inFlight.current = false; setBusy(false); }
  }
 };
 return { files: files.map((file) => measured[file.id] ?? file), inspect, busy, error };
}
