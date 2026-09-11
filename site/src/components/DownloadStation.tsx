// The download station: an explicit two-step choice, system then architecture.
//
// Contracts this component must keep (docs/design-system.md §12.3):
//  - server-rendered markup contains all three panels and all six download
//    links, so the page works without JavaScript;
//  - hydration collapses the panels and lets the pills reveal exactly one;
//  - JavaScript never guesses the architecture and never starts a download;
//  - signing warnings appear before the links, facts may be omitted, never
//    invented.
import { useState, useEffect, useRef } from 'react';
import type { Platform } from '../lib/copy';
import { LATEST } from '../lib/copy';
import { formatSize, formatDate } from '../lib/format';
import type { Release } from '../lib/release';

interface DownloadsCopy {
	eyebrow: string;
	title: string;
	lead: string;
	statusIdle: string;
	statusSelected: string;
	chooseArch: string;
	download: string;
	releaseNotes: string;
	shaLabel: string;
	noJs: string;
	warnings: Record<Platform['id'], string>;
}

interface Props {
	t: DownloadsCopy;
	platforms: Platform[];
	release: Release;
	releaseUrl: string;
}

export default function DownloadStation({ t, platforms, release, releaseUrl }: Props) {
	const [selected, setSelected] = useState<Platform['id'] | null>(null);
	const [hydrated, setHydrated] = useState(false);
	const statusRef = useRef<HTMLParagraphElement>(null);

	useEffect(() => setHydrated(true), []);

	const assetFor = (file: string) => release.assets.find((asset) => asset.file === file);
	const metaFor = (file: string) =>
		[release.version, formatSize(assetFor(file)?.size), formatDate(release.publishedAt)]
			.filter(Boolean)
			.join(' · ');

	const select = (id: Platform['id']) => {
		setSelected(id);
		const label = platforms.find((platform) => platform.id === id)?.label ?? id;
		if (statusRef.current) statusRef.current.textContent = `${t.statusSelected} ${label}.`;
	};

	return (
		<div>
			<div className="flex min-[43.75rem]:items-end min-[43.75rem]:justify-between gap-5">
				<div>
					<p className="eyebrow">{t.eyebrow}</p>
					<h2 className="font-display text-[1.5rem] leading-[1.1] tracking-[var(--tracking-title)] font-normal mt-0">
						{t.title}
					</h2>
				</div>
				<p className="text-muted text-sm leading-relaxed m-0 max-w-[38ch]">{t.lead}</p>
			</div>

			<div role="group" aria-label={t.title} className="grid grid-cols-1 max-[43.75rem]:grid-cols-1 min-[43.75rem]:grid-cols-3 gap-2.5 mt-6">
				{platforms.map((platform) => (
					<button
						key={platform.id}
						type="button"
						aria-pressed={selected === platform.id}
						onClick={() => select(platform.id)}
						className={`flex min-h-[54px] items-center justify-between rounded-full border px-5 py-3 font-semibold cursor-pointer transition-[background,border-color,transform] duration-[var(--duration-fast)] ease-[var(--ease-cine)] active:translate-y-px ${
							selected === platform.id
								? 'border-bone bg-raised text-bone'
								: 'border-line bg-transparent text-bone hover:bg-raised'
						}`}
					>
						{platform.label}
						<span aria-hidden="true" className="text-muted">→</span>
					</button>
				))}
			</div>

			<p ref={statusRef} aria-live="polite" className="text-muted text-sm min-h-[1.4rem] mt-2.5 mb-0">
				{hydrated ? t.statusIdle : ''}
			</p>

			<div className="mt-2">
				{platforms.map((platform) => (
					<section
						key={platform.id}
						hidden={hydrated && selected !== platform.id}
						aria-labelledby={`${platform.id}-download-title`}
						className="border border-line rounded-[var(--radius-card)] bg-surface p-6 [margin-top:0.75rem] first:mt-0"
					>
						<h3 id={`${platform.id}-download-title`} className="text-base font-semibold m-0">
							{platform.label} · {t.chooseArch}
						</h3>
						<p className="text-muted text-sm mt-3 mb-0">{t.warnings[platform.id]}</p>
						<div className="grid grid-cols-1 max-[43.75rem]:grid-cols-1 min-[43.75rem]:grid-cols-2 gap-2.5 mt-5">
							{platform.assets.map((declared) => (
								<a
									key={declared.file}
									href={`${LATEST}/${declared.file}`}
									aria-label={`${t.download} ${platform.label} ${declared.arch}`}
									className="flex min-h-[62px] items-center justify-between gap-4 rounded-xl border border-line bg-ink px-5 py-3.5 transition-[background,transform] duration-[var(--duration-fast)] ease-[var(--ease-cine)] hover:bg-raised active:translate-y-px"
								>
									<span>
										<strong className="block text-sm font-semibold">{declared.arch}</strong>
										<small className="block text-muted text-[0.6875rem] font-normal">
											{metaFor(declared.file)}
										</small>
									</span>
									<span aria-hidden="true" className="text-muted">↓</span>
								</a>
							))}
						</div>
						<div className="flex flex-wrap items-start gap-x-5 gap-y-2 mt-4">
							<a className="link-quiet text-sm font-semibold" href={releaseUrl}>
								{t.releaseNotes} <span aria-hidden="true">↗</span>
							</a>
							<details className="min-w-[min(100%,20rem)] flex-1">
								<summary className="flex min-h-[44px] items-center text-sm font-semibold cursor-pointer">
									{t.shaLabel}
								</summary>
								<div className="grid gap-2.5 text-muted text-[0.6875rem] pb-2">
									{platform.assets.map((declared) => {
										const sha = assetFor(declared.file)?.sha256;
										if (!sha) return null;
										return (
											<span key={declared.file} className="block">
												<span className="block">{declared.file}</span>
												<code className="block text-bone font-mono wrap-anywhere">{sha}</code>
											</span>
										);
									})}
								</div>
							</details>
						</div>
					</section>
				))}
			</div>
		</div>
	);
}
