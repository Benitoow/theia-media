import { catalogs, defaultLocale } from '../src/lib/i18n/locales/index.js';

const requiredTMDBAttribution =
	'This product uses the TMDB API but is not endorsed or certified by TMDB.';

function valueType(value) {
	if (value === null) return 'null';
	if (Array.isArray(value)) return 'array';
	return typeof value;
}

function compareShape(reference, candidate, path, errors, stats) {
	const referenceType = valueType(reference);
	const candidateType = valueType(candidate);

	if (referenceType !== candidateType) {
		errors.push(`${path}: expected ${referenceType}, received ${candidateType}`);
		return;
	}

	if (referenceType === 'function') {
		stats.functions++;
		stats.entries++;
		if (reference.length !== candidate.length) {
			errors.push(
				`${path}: expected a function with ${reference.length} argument(s), received ${candidate.length}`
			);
		}
		return;
	}

	if (referenceType === 'array') {
		if (reference.length !== candidate.length) {
			errors.push(`${path}: expected ${reference.length} item(s), received ${candidate.length}`);
		}

		const sharedLength = Math.min(reference.length, candidate.length);
		for (let index = 0; index < sharedLength; index++) {
			compareShape(reference[index], candidate[index], `${path}[${index}]`, errors, stats);
		}
		return;
	}

	if (referenceType === 'object') {
		const referenceKeys = Object.keys(reference).sort();
		const candidateKeys = Object.keys(candidate).sort();
		const missing = referenceKeys.filter((key) => !candidateKeys.includes(key));
		const extra = candidateKeys.filter((key) => !referenceKeys.includes(key));

		if (missing.length > 0) errors.push(`${path}: missing ${missing.join(', ')}`);
		if (extra.length > 0) errors.push(`${path}: unexpected ${extra.join(', ')}`);

		for (const key of referenceKeys) {
			if (!Object.hasOwn(candidate, key)) continue;
			compareShape(reference[key], candidate[key], `${path}.${key}`, errors, stats);
		}
		return;
	}

	stats.entries++;
}

const errors = [];
const stats = { entries: 0, functions: 0 };
const reference = catalogs[defaultLocale];
const localeCodes = Object.keys(catalogs);

if (!reference) errors.push(`Default locale "${defaultLocale}" is not registered.`);
if (localeCodes.length < 2) errors.push('At least two locale catalogues must be registered.');

if (reference) {
	// Count the reference once so the success message stays meaningful as more
	// languages are registered.
	compareShape(reference, reference, `catalog.${defaultLocale}`, [], stats);
	for (const [code, candidate] of Object.entries(catalogs)) {
		if (code === defaultLocale) continue;
		compareShape(reference, candidate, `catalog.${code}`, errors, { entries: 0, functions: 0 });
	}
}

for (const catalog of Object.values(catalogs)) {
	if (catalog.strings.tmdbAttribution !== requiredTMDBAttribution) {
		errors.push(
			`catalog.${catalog.metadata.code}.strings.tmdbAttribution: required TMDB wording changed`
		);
	}
}

if (errors.length > 0) {
	console.error('Locale catalogue mismatch (fr → en):');
	for (const error of errors) console.error(`- ${error}`);
	process.exitCode = 1;
} else {
	console.log(
		`Locale catalogues match: ${localeCodes.join(' ↔ ')} (${stats.entries} values, ${stats.functions} functions).`
	);
}
