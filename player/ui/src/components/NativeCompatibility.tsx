import { useCallback, useEffect, useState } from 'react';
import { getAppWindow, invoke } from '../lib/bridge';
import type { DisplayCapabilities, FileMedia } from '../types';
import { Button } from './ui/button';

export function mediaBadges(media?: FileMedia): string[] {
	if (media?.status !== 'ok') return [];
	const video = media.video;
	const width = video?.width ?? 0;
	const audio = media.audio_tracks?.find((track) => track.is_default) ?? media.audio_tracks?.[0];
	return [width >= 3840 ? '4K UHD' : width >= 2560 ? 'QHD' : width >= 1900 ? 'Full HD' : width >= 1200 ? 'HD' : width > 0 ? 'SD' : '',
		video?.color_transfer === 'smpte2084' ? 'HDR (PQ)' : video?.color_transfer === 'arib-std-b67' ? 'HLG' : '',
		video?.dolby_vision ? 'Dolby Vision' : '', video?.codec?.toUpperCase(),
		audio?.codec === 'dts' && /ma/i.test(audio.profile ?? '') ? 'DTS-HD MA' : audio?.codec?.toUpperCase(),
		audio?.channels?.replace(/\(.*\)$/, ''),
	].filter(Boolean) as string[];
}

export function NativeCompatibility({ media, t, onInspect, inspecting = false, inspectionError = false }: { media?: FileMedia; t: (key: string) => string; onInspect?: () => void; inspecting?: boolean; inspectionError?: boolean }) {
	const video = media?.status === 'ok' ? media.video : undefined;
	const [caps, setCaps] = useState<DisplayCapabilities | null>(null);
	const [loading, setLoading] = useState(true);
	const [revision, setRevision] = useState(0);
	const refresh = useCallback(() => setRevision((value) => value + 1), []);
	useEffect(() => {
		let current = true;
		setLoading(true); setCaps(null);
		invoke<string>('player_display_capabilities', { codec: video?.codec ?? '', width: video?.width ?? 0, height: video?.height ?? 0 })
			.then((raw) => { if (current) setCaps(JSON.parse(raw)); }).catch(() => {})
			.finally(() => { if (current) setLoading(false); });
		return () => { current = false; };
	}, [video?.codec, video?.width, video?.height, revision]);
	useEffect(() => { window.addEventListener('focus', refresh); return () => window.removeEventListener('focus', refresh); }, [refresh]);
	useEffect(() => {
		let disposed = false;
		let timer: number | undefined;
		const unlisten: Array<() => void> = [];
		const moved = () => { window.clearTimeout(timer); timer = window.setTimeout(refresh, 400); };
		const appWindow = getAppWindow();
		for (const subscribe of [appWindow?.onMoved?.bind(appWindow), appWindow?.onResized?.bind(appWindow)]) {
			subscribe?.(moved).then((stop: () => void) => { if (disposed) stop(); else unlisten.push(stop); }).catch(() => {});
		}
		return () => { disposed = true; window.clearTimeout(timer); unlisten.forEach((stop) => stop()); };
	}, [refresh]);
	const hdr = video?.color_transfer === 'smpte2084' || video?.color_transfer === 'arib-std-b67';
	const displaySize = caps?.displayWidth && caps.displayHeight ? `${caps.displayWidth} × ${caps.displayHeight}` : null;
	const native4k = caps?.displayWidth && caps.displayHeight ? Math.max(caps.displayWidth, caps.displayHeight) >= 3840 && Math.min(caps.displayWidth, caps.displayHeight) >= 2160 : null;
	const rows = [
		{ label: t('fileCharacteristics'), state: video ? `${video.width ?? '?'} × ${video.height ?? '?'} · ${video.codec.toUpperCase()}` : t('notVerified'), detail: mediaBadges(media).join(' · ') || t('mediaUnmeasured') },
		{ label: t('displayResolution'), state: displaySize || t('notVerified'), detail: native4k === null ? t('displayUnknown') : native4k ? t('display4k') : t('displayScaled') },
		{ label: t('hdrOutput'), state: !video ? t('notVerified') : !hdr ? t('sourceSDR') : caps?.hdrEnabled === true ? t('hdrEnabled') : caps?.hdrEnabled === false ? t('sdrOutput') : t('notVerified'), detail: !video ? t('mediaUnmeasured') : !hdr ? t('sourceSDRDetail') : caps?.hdrEnabled === true ? t('hdrDriverDetail') : caps?.hdrEnabled === false ? t('hdrToneMapDetail') : t('hdrUnknownDetail') },
		{ label: t('hardwareDecode'), state: loading ? t('loading') : caps?.hardwareDecode === true ? t('driverReported') : caps?.hardwareDecode === false ? t('softwareFallback') : t('notVerified'), detail: caps?.decoderProfiles?.length ? `${caps.adapter ?? ''} · ${caps.decoderProfiles.join(' · ')}` : t('decoderUnknownDetail') },
	];
	return <section className="native-compatibility" aria-label={t('playbackCompatibility')}>
		<div className="compatibility-heading"><h2 className="label">{t('playbackCompatibility')}</h2><Button variant="outline" disabled={loading} onClick={refresh}>{t('refreshCapabilities')}</Button></div>
		{!video && onInspect && <div className="compatibility-inspection"><Button variant="outline" disabled={inspecting} onClick={onInspect}>{t(inspecting ? 'inspectingFile' : 'inspectFile')}</Button>{inspectionError && <p role="alert">{t('inspectionFailed')}</p>}</div>}
		<ul>{rows.map((row) => <li key={row.label}><div><strong>{row.label}</strong><span>{row.state}</span></div><p>{row.detail}</p></li>)}</ul>
		{video?.dolby_vision && <p>{t('dolbyVisionCaveat')}</p>}
		<p className="compatibility-caveat">{t('nativePreflightCaveat')}</p>
	</section>;
}
