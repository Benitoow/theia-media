import fr from './fr.js';
import en from './en.js';

// English is the base of the product (decision 137). It is what answers a
// browser that has never chosen, and what an unknown code means. The language
// this installation was set up in arrives from the server - see
// i18n.adoptServerLanguage - and an explicit choice still wins over both.
export const defaultLocale = 'en';

// This is the only registry to extend when a catalogue is added. Both the
// runtime and the build-time parity check consume it.
export const catalogs = Object.freeze({
	fr,
	en
});
