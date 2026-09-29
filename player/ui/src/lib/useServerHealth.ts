import { useEffect, useState } from 'react';
import { invoke } from './bridge';

/** One probe at a time with bounded backoff, keeping the current server selected. */
export function useServerHealth(url: string | undefined, recovered: () => Promise<void>, waitingForConnection = false) {
    const [offline, setOffline] = useState(false);
    useEffect(() => {
        setOffline(false);
        if (!url) return;
        let cancelled = false, failures = 0, wasOffline = false;
        let timer: ReturnType<typeof setTimeout>;
        const probe = async () => {
            const reachable = await invoke<boolean>('player_server_reachable', { url }).catch(() => false);
            if (cancelled) return;
            setOffline(!reachable);
            if (reachable && (wasOffline || waitingForConnection)) await recovered();
            wasOffline = !reachable;
            failures = reachable ? 0 : Math.min(failures + 1, 3);
            if (!cancelled) timer = setTimeout(probe, [15000, 5000, 10000, 30000][failures]);
        };
        timer = setTimeout(probe, 15000);
        return () => { cancelled = true; clearTimeout(timer); };
    }, [url, recovered, waitingForConnection]);
    return offline;
}
