import { useState } from 'react';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from './ui/dialog';
import { Button } from './ui/button';
import { invoke } from '../lib/bridge';

export default function PlayerHelp({ open, t, onClose }: { open: boolean; t: (key: string) => string; onClose: () => void }) {
    const [copied, setCopied] = useState(false);
    const [failed, setFailed] = useState(false);
    const copy = async () => {
        try { const diagnostic = await invoke<string>('player_clean_diagnostic'); await navigator.clipboard.writeText(diagnostic); setCopied(true); setFailed(false); }
        catch { setCopied(false); setFailed(true); }
    };
    return <Dialog open={open} onOpenChange={(value) => !value && onClose()}><DialogContent className="help-sheet">
        <DialogTitle>{t('keyboardHelp')}</DialogTitle><DialogDescription>{t('keyboardHelpHint')}</DialogDescription>
        <dl className="shortcut-list">{[['Space / K', 'playPauseHelp'], ['← / →', 'seekHelp'], ['↑ / ↓', 'volume'], ['M', 'mute'], ['F', 'fullscreen'], ['Esc', 'escapeHelp'], ['Ctrl / Cmd + F', 'search'], ['?', 'keyboardHelp']].map(([key, label]) => <div key={key}><dt><kbd>{key}</kbd></dt><dd>{t(label)}</dd></div>)}</dl>
        <Button onClick={() => void copy()}>{t(copied ? 'copied' : 'copyDiagnostic')}</Button>{failed && <p role="alert">{t('copyFailed')}</p>}
    </DialogContent></Dialog>;
}
