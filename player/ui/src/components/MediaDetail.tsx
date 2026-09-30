import { useState, type ReactNode } from 'react';
import { motion } from 'motion/react';
import type { CastCredit, Progress } from '../types';
import { CastRail } from './CastRail';

type Props = {
    kind: 'movie' | 'series'; title: string; poster?: string; backdrop?: string; tagline?: string;
    facts: Array<string | number | null | undefined>; overview?: string; cast?: CastCredit[];
    credits: Array<{ label: string; value?: string }>; progress?: Progress; actions?: ReactNode;
    badges?: string[]; versions?: ReactNode; playback?: ReactNode;
    children?: ReactNode; t: (key: string) => string;
};

/** Artwork always passes through Theia's local image cache. */
function Artwork({ src, className, initial }: { src?: string; className: string; initial?: string }) {
    const [failed, setFailed] = useState(false);
    return <div className={className}>{src && !failed ? <img src={src} alt="" crossOrigin="anonymous" onError={() => setFailed(true)} loading="eager" /> : initial && <span aria-hidden="true">{initial}</span>}</div>;
}

export function MediaDetail(props: Props) {
    const percent = props.progress?.duration_seconds ? Math.max(0, Math.min(100, (props.progress.position_seconds ?? 0) / props.progress.duration_seconds * 100)) : 0;
    const credits = props.credits.filter((credit) => credit.value);
    return <motion.article className={`media-detail ${props.kind}-detail`} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
        <Artwork key={props.backdrop} src={props.backdrop} className={`detail-backdrop ${props.backdrop ? '' : 'detail-backdrop--empty'}`} />
        <div className="detail-record">
            <header className="detail-identity">
                <Artwork key={props.poster} src={props.poster} className="detail-poster" initial={props.title.trim()[0] || '?'} />
                <div className="detail-title-block">
                    <p className="detail-kind label">{props.t(props.kind === 'movie' ? 'filmSingular' : 'seriesLabel')}</p>
                    <h1 className="detail-title">{props.title}</h1>
                    <p className="detail-facts">{props.facts.filter(Boolean).join(' · ')}</p>
                    {Boolean(props.badges?.length) && <ul className="detail-formats" aria-label={props.t('fileCharacteristics')}>{props.badges?.map((badge) => <li key={badge}>{badge}</li>)}</ul>}
                    <div className="detail-actions">{props.actions}</div>
                    {percent > 0 && !props.progress?.finished && <div className="detail-progress" role="progressbar" aria-label={props.t('viewingProgress')} aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(percent)}><span style={{ width: `${percent}%` }} /></div>}
                    {props.versions}
                    <p className="detail-overview">{props.overview || props.t('noOverview')}</p>
                    {props.tagline && <p className="detail-tagline">{props.tagline}</p>}
                </div>
            </header>
            <div className="detail-body">
                {props.playback && <details className="detail-playback"><summary>{props.t('playbackCompatibility')}</summary>{props.playback}</details>}
                {props.children}
                {props.cast && <CastRail cast={props.cast} t={props.t} />}
                {credits.length > 0 && <section className="detail-credits"><h2 className="label">{props.t('credits')}</h2><dl>{credits.map((credit) => <div key={credit.label}><dt>{credit.label}</dt><dd>{credit.value}</dd></div>)}</dl></section>}
                <p className="detail-attribution">{props.t('tmdbAttribution')}</p>
            </div>
        </div>
    </motion.article>;
}
