import fr from './fr.js';
import en from './en.js';

export const defaultLocale = 'fr';

// This is the only registry to extend when a catalogue is added. Both the
// runtime and the build-time parity check consume it.
export const catalogs = Object.freeze({
	fr,
	en
});
