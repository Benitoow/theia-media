import { useState } from 'react';
import { MediaCard } from './MediaCard';
import { Button } from './ui/button';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from './ui/dialog';
import type { Episode } from '../types';

export function EpisodeCard({ item, seriesLabel, busy, onPlay, onHistory, reducedMotion, t, gridIndex, gridTotal }: {
    gridIndex?: number; gridTotal?: number;
    item: Episode; seriesLabel: string; busy: boolean; onPlay: (id: number) => void;
    onHistory: (id: number, watched: boolean) => void; reducedMotion: boolean; t: (key: string) => string;
}) {
    const [confirm, setConfirm] = useState(false);
    return <MediaCard gridIndex={gridIndex} gridTotal={gridTotal} kind="episode" item={item} seriesLabel={seriesLabel} onOpen={onPlay}
        resumeLabel={t('resumeAt')} actionLabel={t('playEpisode')} kindLabel={t('episodeUntitled')} reducedMotion={reducedMotion} t={t}
        footer={<div className="episode-actions">
            {!item.progress?.finished && <button disabled={busy} onClick={() => onHistory(item.id, true)}>{t('markWatched')}</button>}
            {(item.progress?.finished || (item.progress?.position_seconds ?? 0) >= 30) && <button disabled={busy} onClick={() => setConfirm(true)}>{t('resetProgress')}</button>}
            <Dialog open={confirm} onOpenChange={setConfirm}><DialogContent className="episode-reset-sheet">
                <DialogTitle>{t('resetProgress')}</DialogTitle><DialogDescription>{t('resetEpisodeConfirm')}</DialogDescription>
                <Button onClick={() => { setConfirm(false); onHistory(item.id, false); }}>{t('resetProgress')}</Button>
                <Button variant="outline" onClick={() => setConfirm(false)}>{t('cancel')}</Button>
            </DialogContent></Dialog>
        </div>} />;
}
