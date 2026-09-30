import { Check, RotateCcw } from 'lucide-react';
import { useState } from 'react';
import { MediaCard } from './MediaCard';
import { Button } from './ui/button';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from './ui/dialog';
import type { Episode } from '../types';

export function EpisodeCard({ item, seriesLabel, compactLegend, busy, onPlay, onHistory, reducedMotion, t, gridIndex, gridTotal }: {
    gridIndex?: number; gridTotal?: number; compactLegend?: boolean;
    item: Episode; seriesLabel: string; busy: boolean; onPlay: (id: number) => void;
    onHistory: (id: number, watched: boolean) => void; reducedMotion: boolean; t: (key: string) => string;
}) {
    const [confirm, setConfirm] = useState(false);
    return <MediaCard gridIndex={gridIndex} gridTotal={gridTotal} kind="episode" item={item} seriesLabel={seriesLabel} compactLegend={compactLegend} onOpen={onPlay}
        resumeLabel={t('resumeAt')} actionLabel={t('playEpisode')} kindLabel={t('episodeUntitled')} reducedMotion={reducedMotion} t={t}
        footer={<div className="episode-actions">
            {!item.progress?.finished && <Button variant="ghost" aria-label={t('markWatched')} title={t('markWatched')} disabled={busy} onClick={() => onHistory(item.id, true)}><Check size={15} aria-hidden="true" /><span className="episode-action-label">{t('markWatched')}</span></Button>}
            {(item.progress?.finished || (item.progress?.position_seconds ?? 0) >= 30) && <Button variant="ghost" size="icon" aria-label={t('resetProgress')} title={t('resetProgress')} disabled={busy} onClick={() => setConfirm(true)}><RotateCcw size={15} aria-hidden="true" /></Button>}
            <Dialog open={confirm} onOpenChange={setConfirm}><DialogContent className="episode-reset-sheet">
                <DialogTitle>{t('resetProgress')}</DialogTitle><DialogDescription>{t('resetEpisodeConfirm')}</DialogDescription>
                <Button onClick={() => { setConfirm(false); onHistory(item.id, false); }}>{t('resetProgress')}</Button>
                <Button variant="outline" onClick={() => setConfirm(false)}>{t('cancel')}</Button>
            </DialogContent></Dialog>
        </div>} />;
}
