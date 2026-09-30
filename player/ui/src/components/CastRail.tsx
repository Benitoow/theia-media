import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react';
import { ChevronLeft, ChevronRight, UserRound } from 'lucide-react';
import type { CastCredit } from '../types';
import { Button } from './ui/button';
import '../cast-rail.css';

function Portrait({ credit }: { credit: CastCredit }) {
    const [failed, setFailed] = useState(false);
    return <div className="detail-portrait">
        {credit.profile_url && !failed
            ? <img src={credit.profile_url} alt="" crossOrigin="anonymous" loading="lazy" onError={() => setFailed(true)} />
            : <UserRound size={36} strokeWidth={1.2} aria-hidden="true" />}
    </div>;
}

/** Portraits remain a readable list; scrolling never changes the page width. */
export function CastRail({ cast, t }: { cast: CastCredit[]; t: (key: string) => string }) {
    const people = cast.filter((credit) => credit.name);
    const rail = useRef<HTMLUListElement>(null);
    const id = useId();
    const [edges, setEdges] = useState({ previous: false, next: false });
    const measure = () => {
        const list = rail.current;
        if (list) setEdges({ previous: list.scrollLeft > 1, next: list.scrollLeft + list.clientWidth < list.scrollWidth - 1 });
    };
    useEffect(() => {
        const list = rail.current;
        if (!list) return;
        const observer = new ResizeObserver(measure);
        observer.observe(list); measure();
        return () => observer.disconnect();
    }, [people.length]);
    const move = (direction: number) => {
        const list = rail.current;
        if (list) list.scrollBy({ left: direction * Math.max(160, list.clientWidth * 0.8) });
    };
    const key = (event: KeyboardEvent<HTMLUListElement>) => {
        if (event.target !== event.currentTarget || event.altKey || event.ctrlKey || event.metaKey) return;
        if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
            event.preventDefault(); move(event.key === 'ArrowRight' ? 1 : -1);
        } else if (event.key === 'Home' || event.key === 'End') {
            event.preventDefault(); rail.current?.scrollTo({ left: event.key === 'End' ? rail.current.scrollWidth : 0 });
        }
    };
    if (!people.length) return null;
    return <section className="detail-cast">
        <header className="cast-heading">
            <h2 className="label">{t('cast')}</h2>
            {(edges.previous || edges.next) && <div className="cast-controls">
                <Button variant="outline" size="icon" aria-label={t('previousCast')} aria-controls={id} disabled={!edges.previous} onClick={() => move(-1)}><ChevronLeft size={18} aria-hidden="true" /></Button>
                <Button variant="outline" size="icon" aria-label={t('nextCast')} aria-controls={id} disabled={!edges.next} onClick={() => move(1)}><ChevronRight size={18} aria-hidden="true" /></Button>
            </div>}
        </header>
        <ul id={id} className="cast-rail" ref={rail} tabIndex={0} aria-label={t('cast')} onScroll={measure} onKeyDown={key}>
            {people.map((credit, index) => <li key={`${credit.name}-${index}`}>
                <Portrait key={credit.profile_url} credit={credit} />
                <span className="cast-name" title={credit.name}>{credit.name}</span>
                <small className="cast-role" title={credit.character}>{credit.character}</small>
            </li>)}
        </ul>
    </section>;
}
