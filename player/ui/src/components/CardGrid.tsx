import { Children, useEffect, useLayoutEffect, useRef, useState } from 'react';

/** Small catalogues stay plain. Large ones retain a window of complete rows. */
export function CardGrid({ children }: { children: React.ReactNode }) {
    const entries = Children.toArray(children);
    const root = useRef<HTMLUListElement>(null);
    const focusRequest = useRef<number | null>(null);
    const [range, setRange] = useState({ start: 0, end: 80, before: 0, after: 0, columns: 1 });
    const virtual = entries.length > 240;
    useEffect(() => {
        if (!virtual) return;
        const grid = root.current, scroller = grid?.closest<HTMLElement>('.library');
        if (!grid || !scroller) return;
        let frame = 0;
        const update = () => {
            const style = getComputedStyle(grid);
            const columns = Math.max(1, style.gridTemplateColumns.split(' ').length);
            const card = grid.querySelector<HTMLElement>('.media-card');
            if (!card) return;
            const gap = parseFloat(style.rowGap) || 0, pitch = card.getBoundingClientRect().height + gap;
            if (!pitch) return;
            const offset = Math.max(0, scroller.getBoundingClientRect().top - grid.getBoundingClientRect().top);
            const rows = Math.ceil(entries.length / columns);
            let first = Math.max(0, Math.floor(offset / pitch) - 3);
            let last = Math.min(rows, first + Math.ceil(scroller.clientHeight / pitch) + 7);
            const focused = document.activeElement?.closest<HTMLElement>('[data-grid-index]');
            if (focused && grid.contains(focused)) {
                const focusedRow = Math.floor(Number(focused.dataset.gridIndex) / columns);
                first = Math.min(first, focusedRow); last = Math.max(last, focusedRow + 1);
            }
            setRange({ start: first * columns, end: Math.min(entries.length, last * columns), before: Math.max(0, first * pitch - gap), after: Math.max(0, (rows - last) * pitch - gap), columns });
        };
        const schedule = () => { cancelAnimationFrame(frame); frame = requestAnimationFrame(update); };
        const resize = new ResizeObserver(schedule); resize.observe(grid); resize.observe(scroller);
        scroller.addEventListener('scroll', schedule, { passive: true }); update();
        return () => { cancelAnimationFrame(frame); resize.disconnect(); scroller.removeEventListener('scroll', schedule); };
    }, [virtual, entries.length]);
    useLayoutEffect(() => {
        if (focusRequest.current === null) return;
        const button = root.current?.querySelector<HTMLButtonElement>(`[data-grid-index="${focusRequest.current}"] .film`);
        if (button) { focusRequest.current = null; button.focus({ preventScroll: true }); button.scrollIntoView({ block: 'nearest', behavior: 'instant' }); }
    }, [range]);
    const onKey = (event: React.KeyboardEvent<HTMLUListElement>) => {
        if (!virtual || event.altKey || event.ctrlKey || event.metaKey) return;
        const target = event.target as HTMLElement, item = target.closest<HTMLElement>('[data-grid-index]');
        if (!item) return;
        const current = Number(item.dataset.gridIndex), columns = range.columns;
        let next = current;
        if (target.matches('.film')) {
            if (event.key === 'ArrowRight') next++;
            else if (event.key === 'ArrowLeft') next--;
            else if (event.key === 'ArrowDown') next += columns;
            else if (event.key === 'ArrowUp') next -= columns;
            else if (event.key === 'Home') next = 0;
            else if (event.key === 'End') next = entries.length - 1;
        }
        if (event.key === 'Tab') {
            const buttons = Array.from(item.querySelectorAll<HTMLElement>('button:not(:disabled), [href], input:not(:disabled)'));
            if (target === (event.shiftKey ? buttons[0] : buttons[buttons.length - 1])) {
                const adjacent = current + (event.shiftKey ? -1 : 1);
                if (adjacent >= 0 && adjacent < entries.length && !root.current?.querySelector(`[data-grid-index="${adjacent}"]`)) next = adjacent;
            }
        }
        next = Math.max(0, Math.min(entries.length - 1, next));
        if (next === current) return;
        event.preventDefault();
        const grid = root.current!, card = grid.querySelector<HTMLElement>('.media-card')!;
        const gap = parseFloat(getComputedStyle(grid).rowGap) || 0, pitch = card.getBoundingClientRect().height + gap;
        const rows = Math.ceil(entries.length / columns), first = Math.max(0, Math.floor(next / columns) - 3), last = Math.min(rows, first + 10);
        focusRequest.current = next;
        setRange({ start: first * columns, end: Math.min(entries.length, last * columns), before: Math.max(0, first * pitch - gap), after: Math.max(0, (rows - last) * pitch - gap), columns });
    };
    const start = virtual ? range.start : 0, end = virtual ? range.end : entries.length;
    return <ul className="films" ref={root} onKeyDown={onKey} data-virtual={virtual || undefined}>
        {virtual && range.before > 0 && <li aria-hidden="true" className="grid-spacer" style={{ height: range.before }} />}
        {entries.slice(start, end).map((entry, index) => <GridEntry key={(entry as React.ReactElement).key ?? index} entry={entry} index={start + index} total={entries.length} />)}
        {virtual && range.after > 0 && <li aria-hidden="true" className="grid-spacer" style={{ height: range.after }} />}
    </ul>;
}

// Clone the existing li component rather than introduce invalid ul/div markup.
import { cloneElement, isValidElement } from 'react';
function GridEntry({ entry, index, total }: { entry: React.ReactNode; index: number; total: number }) {
    return isValidElement(entry) ? cloneElement(entry as React.ReactElement<{ gridIndex?: number; gridTotal?: number }>, { gridIndex: index, gridTotal: total }) : entry;
}
