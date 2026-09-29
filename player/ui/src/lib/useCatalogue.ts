import { useCallback, useRef, useState } from 'react';
import { invoke } from './bridge';
import type { Home, Movie, Series, SeriesHome } from '../types';

/** Independent endpoints preserve the usable half of a partially offline library. */
export function useCatalogue(onError: (key: string) => void) {
    const generation = useRef(0);
    const libraryRequest = useRef(0);
    const homeRequest = useRef(0);
    const [movies, setMovies] = useState<Movie[]>([]);
    const [series, setSeries] = useState<Series[]>([]);
    const [home, setHome] = useState<Home | null>(null);
    const [seriesHome, setSeriesHome] = useState<SeriesHome | null>(null);
    const [homeError, setHomeError] = useState(false);
    const invalidate = useCallback(() => { generation.current++; }, []);
    const clear = useCallback(() => { generation.current++; setMovies([]); setSeries([]); setHome(null); setSeriesHome(null); setHomeError(false); }, []);
    const loadLibrary = useCallback(async () => {
        const scope = generation.current, request = ++libraryRequest.current;
        const responses = await Promise.allSettled([invoke<string>('player_library'), invoke<string>('player_series')]);
        if (scope !== generation.current || request !== libraryRequest.current) return;
        try { if (responses[0].status === 'fulfilled') setMovies(JSON.parse(responses[0].value)); } catch { onError('libraryFailed'); }
        try { if (responses[1].status === 'fulfilled') setSeries(JSON.parse(responses[1].value)); } catch { onError('libraryFailed'); }
        if (responses.some((response) => response.status === 'rejected')) onError('libraryFailed');
    }, [onError]);
    const loadHome = useCallback(async () => {
        const scope = generation.current, request = ++homeRequest.current;
        const responses = await Promise.allSettled([invoke<string>('player_home'), invoke<string>('player_series_home')]);
        if (scope !== generation.current || request !== homeRequest.current) return;
        let usable = false;
        try { if (responses[0].status === 'fulfilled') { setHome(JSON.parse(responses[0].value)); usable = true; } } catch { /* Other endpoint remains usable. */ }
        try { if (responses[1].status === 'fulfilled') { setSeriesHome(JSON.parse(responses[1].value)); usable = true; } } catch { /* Other endpoint remains usable. */ }
        setHomeError(!usable);
    }, []);
    return { movies, series, home, seriesHome, homeError, loadLibrary, loadHome, invalidate, clear };
}
