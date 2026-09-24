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

import { catalogues, vocabulary } from '../src/lib/catalogues.js';

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

// The vocabulary is a map of maps - a language name, a channel layout, the
// readable name of a codec - so the same comparison is walked instead of written
// twice. It drifts the same way, and the symptom is worse: a missing codec word
// shows the engine's own abbreviation in the middle of a French menu.
const vocabularyPaths = (value, prefix = '') =>
	Object.entries(value).flatMap(([key, entry]) =>
		entry && typeof entry === 'object' ? vocabularyPaths(entry, `${prefix}${key}.`) : [`${prefix}${key}`]
	);

for (const language of languages) {
	if (!vocabulary[language]) {
		errors.push(`vocabulary.${language}: missing`);
	}
}
if (vocabulary[reference]) {
	const expected = vocabularyPaths(vocabulary[reference]).sort();
	for (const language of others) {
		if (!vocabulary[language]) continue;
		const actual = vocabularyPaths(vocabulary[language]).sort();
		for (const key of expected) {
			if (!actual.includes(key)) errors.push(`vocabulary.${language}: missing "${key}"`);
		}
		for (const key of actual) {
			if (!expected.includes(key)) errors.push(`vocabulary.${language}: "${key}" is not in ${reference}`);
		}
	}
	for (const language of languages) {
		for (const key of expected) {
			const value = key.split('.').reduce((node, step) => node?.[step], vocabulary[language]);
			if (typeof value !== 'string' || value.trim() === '') {
				errors.push(`vocabulary.${language}.${key}: empty or not a string`);
			}
		}
	}
}

const counts = languages.map((l) => `${l} ${Object.keys(catalogues[l]).length}`).join(', ');
const wordCounts = languages.map((l) => `${l} ${vocabularyPaths(vocabulary[l] ?? {}).length}`).join(', ');

if (errors.length) {
	console.error('locale catalogues disagree:');
	for (const error of errors) console.error(`  ${error}`);
	process.exit(1);
}

console.log(`osd locales agree (${counts}; vocabulary ${wordCounts})`);
