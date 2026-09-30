import { useState } from 'react';
import { Check, Copy, Keyboard, X } from 'lucide-react';
import { Dialog, DialogClose, DialogContent, DialogTitle, DialogDescription } from './ui/dialog';
import { Button } from './ui/button';
import { invoke } from '../lib/bridge';
import '../player-help.css';

export default function PlayerHelp({ open, t, returnFocus, onClose }: { open: boolean; t: (key: string) => string; returnFocus: HTMLElement | null; onClose: () => void }) {
    const [copyState, setCopyState] = useState<'idle' | 'busy' | 'copied' | 'failed'>('idle');
    const copy = async () => {
        if (copyState === 'busy') return;
        setCopyState('busy');
        try {
            const diagnostic = await invoke<string>('player_clean_diagnostic');
            await navigator.clipboard.writeText(diagnostic);
            setCopyState('copied');
        } catch { setCopyState('failed'); }
    };
    const shortcuts = [
        { label: 'playPauseHelp', keys: [t('spaceKey'), 'K'] },
        { label: 'seekHelp', keys: ['←', '→'] },
        { label: 'volume', keys: ['↑', '↓'] },
        { label: 'mute', keys: ['M'] },
        { label: 'fullscreen', keys: ['F'] },
        { label: 'escapeHelp', keys: ['Esc'] },
        { label: 'search', keys: ['Ctrl / ⌘ + F'] },
        { label: 'keyboardHelp', keys: ['?'] },
    ];
    return <Dialog open={open} onOpenChange={(value) => !value && onClose()}><DialogContent className="help-sheet" overlayClassName="help-overlay" onCloseAutoFocus={(event) => {
        event.preventDefault();
        if (returnFocus?.isConnected) returnFocus.focus();
    }}>
        <header className="help-header">
            <span className="help-icon" aria-hidden="true"><Keyboard size={23} strokeWidth={1.5} /></span>
            <div><DialogTitle>{t('keyboardHelp')}</DialogTitle><DialogDescription>{t('keyboardHelpHint')}</DialogDescription></div>
            <DialogClose asChild><Button variant="ghost" size="icon" aria-label={t('closeHelp')}><X size={18} aria-hidden="true" /></Button></DialogClose>
        </header>
        <dl className="shortcut-list">{shortcuts.map(({ label, keys }) => <div key={label}>
            <dt>{t(label)}</dt><dd>{keys.map((key, index) => <span key={key}>{index > 0 && <span className="key-separator" aria-hidden="true">/</span>}<kbd>{key}</kbd></span>)}</dd>
        </div>)}</dl>
        <section className="help-diagnostic">
            <div><h3>{t('playbackDiagnostic')}</h3><p>{t('diagnosticHint')}</p></div>
            <Button variant="outline" disabled={copyState === 'busy'} aria-busy={copyState === 'busy'} onClick={() => void copy()}>{copyState === 'copied' ? <Check size={16} aria-hidden="true" /> : <Copy size={16} aria-hidden="true" />}{t(copyState === 'copied' ? 'diagnosticCopied' : 'copyDiagnostic')}</Button>
        </section>
        {copyState === 'failed' && <p role="alert">{t('copyFailed')}</p>}
    </DialogContent></Dialog>;
}
