// Guards the OSD's French and English catalogues.
//
// The native player grew a second interface, and a second interface grows a
// second catalogue (src/lib/catalogues.js). Two catalogues drift the moment
// somebody adds a sentence to one of them, and the symptom is a French word in
// the middle of an English screen - which is exactly the fault decision 25
// exists to prevent on the web side.
//
// The web application has its own guard for its own catalogue. This one has
// less to do: the player's catalogue is a flat map of codes to sentences, so
// comparing key sets and checking that nothing is empty is the whole job.
//
//   node scripts/check-locales.mjs

import { catalogues } from '../src/lib/catalogues.js';

const languages = Object.keys(catalogues);
const errors = [];

if (languages.length < 2) {
	errors.push(`expected at least two catalogues, found ${languages.length}`);
}

const [reference, ...others] = languages;

for (const language of others) {
	const expected = Object.keys(catalogues[reference]).sort();
	const actual = Object.keys(catalogues[language]).sort();

	for (const key of expected) {
		if (!actual.includes(key)) errors.push(`${language}: missing "${key}"`);
	}
	for (const key of actual) {
		if (!expected.includes(key)) errors.push(`${language}: "${key}" is not in ${reference}`);
	}
}

for (const language of languages) {
	for (const [key, value] of Object.entries(catalogues[language])) {
		if (typeof value !== 'string' || value.trim() === '') {
			errors.push(`${language}.${key}: empty or not a string`);
		}
	}
}

const counts = languages.map((l) => `${l} ${Object.keys(catalogues[l]).length}`).join(', ');

if (errors.length) {
	console.error('locale catalogues disagree:');
	for (const error of errors) console.error(`  ${error}`);
	process.exit(1);
}

console.log(`osd locales agree (${counts})`);
