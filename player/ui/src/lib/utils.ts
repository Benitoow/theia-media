import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

/**
 * The runtime register, shared by the hero and anything else that states how
 * long something is. French runs "1 h 35", English "1 hr 35 min" - the same
 * two catalogues' voices, without a formatter package for two lines.
 */
export function formatRuntime(minutes: number | undefined, language: string): string | null {
	if (!minutes || minutes <= 0) return null;
	if (minutes < 60) return `${minutes} min`;
	const hours = Math.floor(minutes / 60);
	const rest = minutes % 60;
	if (rest === 0) return language === 'en' ? `${hours} hr` : `${hours} h`;
	return language === 'en'
		? `${hours} hr ${String(rest).padStart(2, '0')} min`
		: `${hours} h ${String(rest).padStart(2, '0')}`;
}
