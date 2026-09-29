
import { Check, ImagePlus, Pencil, UserRound, X } from 'lucide-react';
import { FormEvent, useEffect, useState } from 'react';

import { Button } from './ui/button';
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from './ui/dialog';

import type { Profile } from '../types';

import { profileAvatarURL } from '../lib/profiles';
export default function ProfileDialog({ open, profiles, activeProfile, serverURL, busy, t, onClose, onSelect, onRename, onSetAvatar, onClearAvatar }: { open: boolean; profiles: Profile[]; activeProfile: number | null; serverURL: string; busy: boolean; t: (key: string) => string; onClose: () => void; onSelect: (id: number) => void; onRename: (id: number, name: string) => Promise<Profile>; onSetAvatar: (id: number, file: File) => Promise<Profile>; onClearAvatar: (id: number) => Promise<Profile> }) {
	const [editingID, setEditingID] = useState<number | null>(null);
	const [draftName, setDraftName] = useState('');
	const [draftFile, setDraftFile] = useState<File | null>(null);
	const [previewURL, setPreviewURL] = useState<string | null>(null);
	const [removeAvatar, setRemoveAvatar] = useState(false);
	const [formError, setFormError] = useState(false);
	const editing = profiles.find((profile) => profile.id === editingID) ?? null;
	useEffect(() => () => { if (previewURL) URL.revokeObjectURL(previewURL); }, [previewURL]);
	useEffect(() => {
		if (!open) setEditingID(null);
	}, [open]);
	const edit = (profile: Profile) => {
		setEditingID(profile.id);
		setDraftName(profile.name || t('profileDefaultName'));
		setDraftFile(null);
		setPreviewURL(null);
		setRemoveAvatar(false);
		setFormError(false);
	};
	const chooseFile = (file?: File) => {
		if (!file) return;
		setDraftFile(file);
		setPreviewURL(URL.createObjectURL(file));
		setRemoveAvatar(false);
	};
	const saveProfile = async (event: FormEvent) => {
		event.preventDefault();
		if (!editing) return;
		setFormError(false);
		try {
			if (draftName.trim() !== (editing.name || t('profileDefaultName'))) await onRename(editing.id, draftName.trim());
			if (draftFile) await onSetAvatar(editing.id, draftFile);
			else if (removeAvatar && editing.has_avatar) await onClearAvatar(editing.id);
			setEditingID(null);
		} catch {
			setFormError(true);
		}
	};
	return (
		<Dialog open={open} onOpenChange={(next) => !next && onClose()}>
			<DialogContent className="profiles-dialog" aria-describedby="profiles-description">
				<header className="settings-header">
					<span className="settings-heading-icon" aria-hidden="true">{editing ? <Pencil size={20} /> : <UserRound size={21} />}</span>
					<div><DialogTitle>{editing ? t('personalizeProfile') : t('profiles')}</DialogTitle><DialogDescription id="profiles-description">{editing ? t('personalizeProfileDescription') : t('profileDescription')}</DialogDescription></div>
					<DialogClose asChild><Button className="settings-close" variant="ghost" size="icon" aria-label={t('closeProfiles')}><X size={18} /></Button></DialogClose>
				</header>
				{editing ? <form className="profile-editor" onSubmit={saveProfile}>
					<div className="profile-editor-avatar" style={{ '--profile-hue': `${(editing.id * 71) % 360}` } as React.CSSProperties}>
						{previewURL || (!removeAvatar && profileAvatarURL(serverURL, editing)) ? <img src={previewURL || profileAvatarURL(serverURL, editing) || ''} alt="" crossOrigin="anonymous" /> : <span>{(draftName || t('profileDefaultName')).trim().slice(0, 1).toUpperCase()}</span>}
					</div>
					<label className="profile-editor-field"><span className="label">{t('profileName')}</span><input value={draftName} onChange={(event) => setDraftName(event.target.value)} maxLength={40} required /></label>
					<div className="profile-picture-actions">
						<label className="profile-file-button"><ImagePlus size={18} /><span>{t('changePicture')}</span><input type="file" accept="image/png,image/jpeg,image/webp" onChange={(event) => chooseFile(event.target.files?.[0])} /></label>
						{editing.has_avatar && !removeAvatar && !draftFile && <Button type="button" variant="ghost" onClick={() => setRemoveAvatar(true)}>{t('removePicture')}</Button>}
					</div>
					<p className="profile-picture-hint">{t('profilePictureHint')}</p>
					{formError && <p className="hint hint--error" role="status">{t('profileSaveFailed')}</p>}
					<footer className="profile-editor-actions"><Button type="button" variant="ghost" onClick={() => setEditingID(null)}>{t('back')}</Button><Button type="submit" disabled={busy || !draftName.trim()}>{busy ? t('savingProfile') : t('saveProfile')}</Button></footer>
				</form> : <div className="profile-list">
					{profiles.map((profile, index) => {
						const active = profile.id === activeProfile;
						const name = profile.name || t('profileDefaultName');
						const avatar = profileAvatarURL(serverURL, profile);
						return <div key={profile.id} className={`profile-card ${active ? 'profile-card--active' : ''}`} style={{ '--profile-hue': `${(profile.id * 71 + index * 43) % 360}` } as React.CSSProperties}><button className="profile-card-select" onClick={() => onSelect(profile.id)} disabled={busy}><span className="profile-avatar">{avatar ? <img src={avatar} alt="" /> : name.trim().slice(0, 1).toUpperCase()}</span><span className="profile-copy"><strong>{name}</strong><small>{active ? t('currentProfile') : t('switchProfile')}</small></span>{active && <Check size={19} aria-hidden="true" />}</button><Button className="profile-edit" type="button" variant="ghost" size="icon" aria-label={`${t('personalizeProfile')} · ${name}`} onClick={() => edit(profile)}><Pencil size={17} /></Button></div>;
					})}
					{profiles.length === 0 && <p className="hint">{t('noProfiles')}</p>}
				</div>}
			</DialogContent>
		</Dialog>
	);
}
