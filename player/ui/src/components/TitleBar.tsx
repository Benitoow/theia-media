import { ArrowLeft, Copy, Minus, Square, X } from 'lucide-react';

import { Button } from './ui/button';

type Props = {
	title?: string | null;
	playing: boolean;
	maximized: boolean;
	language: string;
	onBack: () => void;
	onLanguage: () => void;
	onMinimize: () => void;
	onMaximize: () => void;
	onClose: () => void;
	labels: {
		back: string;
		minimize: string;
		maximize: string;
		restore: string;
		close: string;
	};
};

export function TitleBar({
	title,
	playing,
	maximized,
	language,
	onBack,
	onLanguage,
	onMinimize,
	onMaximize,
	onClose,
	labels,
}: Props) {
	return (
		<header
			// In the library the bar is a drag strip and window controls only:
			// transparent, no blur, no seam — the page behind is continuous and
			// the floating nav below carries the brand. During playback it
			// turns back into solid chrome over the picture.
			className={
				playing
					? 'title-bar absolute inset-x-0 top-0 z-50 flex h-[3.25rem] items-center border-b border-white/[0.075] bg-[rgb(11_10_9/.78)] pl-3 backdrop-blur-xl'
					: 'title-bar title-bar--overlay absolute inset-x-0 top-0 z-50 flex h-[3.25rem] items-center border-b border-transparent bg-transparent pl-3'
			}
			data-tauri-drag-region
			onDoubleClick={onMaximize}
		>
			<div className="flex min-w-0 flex-1 items-center gap-3" data-tauri-drag-region>
				{playing && (
					<Button className="control control--back" variant="ghost" size="icon" onClick={onBack} aria-label={labels.back}>
						<ArrowLeft size={19} />
					</Button>
				)}
				{playing && (
					<span className="film-title truncate font-display text-lg text-[var(--color-parchment)]" data-tauri-drag-region>
						{title}
					</span>
				)}
			</div>
			{!playing && (
				<Button className="control control--language" variant="ghost" size="icon" onClick={onLanguage} aria-label="Français / English">
					<span className="label">{language.toUpperCase()}</span>
				</Button>
			)}
			<div className="window-controls flex h-full">
				<Button className="window-control" variant="ghost" size="window" onClick={onMinimize} aria-label={labels.minimize}>
					<Minus size={17} />
				</Button>
				<Button className="window-control" variant="ghost" size="window" onClick={onMaximize} aria-label={maximized ? labels.restore : labels.maximize}>
					{maximized ? <Copy size={15} /> : <Square size={15} />}
				</Button>
				<Button className="window-control window-control--close" variant="danger" size="window" onClick={onClose} aria-label={labels.close}>
					<X size={17} />
				</Button>
			</div>
		</header>
	);
}
