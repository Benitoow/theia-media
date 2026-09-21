#!/usr/bin/env node
//
// The decision record's guard, index generator and query tool.
//
//	node scripts/decisions.mjs          check; exits non-zero on any drift
//	node scripts/decisions.mjs --write  regenerate the index block
//	node scripts/decisions.mjs --list   query without reading the file
//	                                    [--topic playback] [--status superseded]
//	node scripts/decisions.mjs --json   the whole record, machine-readable
//
// It owns no content: every fact is parsed out of docs/DECISIONS.md, which stays
// the source of truth, and the index it writes is that file's own navigation.
// Run it after adding or superseding a decision. The record is cited from code,
// tests and documents by number, so numbers are identities: the checks below are
// what keeps them stable and the status lines honest.

import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';

const FILE = 'docs/DECISIONS.md';
const START = '<!-- index:start -->';
const END = '<!-- index:end -->';

// The closed vocabulary. A new topic is a decision about the vocabulary, not a
// word added here in passing.
const TOPICS = [
	'installer',
	'interface',
	'library',
	'metadata',
	'playback',
	'player',
	'process',
	'release',
	'security',
	'server',
	'subtitles',
	'testing',
	'updates',
];

// A citation is "decision N" / "decisions N, M and K" - the same shape a reader
// writes and a document already uses. Versions such as "v1.5.0" are not cited.
const CITATION = /[Dd]ecisions?\s+((?:\d+[a-z]?)(?:\s*(?:,|and|&)\s*\d+[a-z]?)*)/g;
const NUMBER = /\d+[a-z]?/g;
const HEADING = /^## (\d+[a-z]?)\. (.+)$/;
const STATUS = /^\*\*Status:\*\* (active|living|superseded(?: in part)? by (.+?)) · \*\*Topics:\*\* (.+)$/;

const cited = (text) => {
	const seen = new Set();
	for (const match of text.matchAll(CITATION)) {
		for (const number of match[1].matchAll(NUMBER)) seen.add(number[0]);
	}
	return [...seen];
};

const rank = (key) => [parseInt(key, 10), key.replace(/^\d+/, '')];

const slug = (key, title) =>
	`#${`${key}. ${title}`
		.toLowerCase()
		.replace(/['’]/g, '')
		.replace(/[^\p{L}\p{N}\s_-]/gu, '')
		.trim()
		.replace(/\s+/g, '-')}`;

const text = readFileSync(FILE, 'utf8');
const eol = text.includes('\r\n') ? '\r\n' : '\n';
const lines = text.split(/\r?\n/);

// ---------------------------------------------------------------- the entries
const entries = [];
lines.forEach((line, index) => {
	const head = HEADING.exec(line);
	if (head) entries.push({ key: head[1], title: head[2], line: index + 1, body: [] });
});
entries.forEach((entry, index) => {
	const end = index + 1 < entries.length ? entries[index + 1].line - 1 : lines.length;
	entry.body = lines.slice(entry.line, end);
	entry.status = { kind: 'missing' };
	const first = entry.body.findIndex((line) => line.trim() !== '');
	if (first !== -1) {
		const match = STATUS.exec(entry.body[first].trim());
		if (match) {
			entry.status = match[1].startsWith('superseded')
				? { kind: 'superseded', inPart: match[1].includes('in part'), by: match[2].split(/,\s*|\s+and\s+/).map((n) => n.trim()) }
				: { kind: match[1] };
			entry.topics = match[3].split(',').map((t) => t.trim());
			entry.statusAt = entry.line + first + 1;
		} else if (entry.body[first].trim().startsWith('**Status:**')) {
			entry.status = { kind: 'malformed' };
			entry.statusAt = entry.line + first + 1;
		}
	}
});

const keys = new Set(entries.map((entry) => entry.key));
const byKey = new Map(entries.map((entry) => [entry.key, entry]));
const failed = [];
const fail = (line, message) => failed.push(`${line === null ? FILE : `${FILE}:${line}`}: ${message}`);

// ------------------------------------------------------- the checks (in order)
// 1. Numbers are identities: unique, and the main sequence has no hole.
const seen = new Map();
for (const entry of entries) {
	if (seen.has(entry.key)) fail(entry.line, `decision ${entry.key} appears twice (also line ${seen.get(entry.key)})`);
	seen.set(entry.key, entry.line);
}
const integers = [...new Set(entries.map((entry) => parseInt(entry.key, 10)))].sort((a, b) => a - b);
const top = integers[integers.length - 1];
for (let n = 1; n <= top; n++) {
	if (!integers.includes(n)) fail(null, `decision ${n} is missing from the sequence`);
}

// 2. The file reads in order, except the living section, which sits last on
// purpose: it was written at M0 and new decisions are inserted above it.
let previous = [0];
let living = null;
for (const entry of entries) {
	const here = rank(entry.key);
	if (entry.status.kind === 'living') {
		living = entry;
		continue;
	}
	if (here[0] < previous[0] || (here[0] === previous[0] && here[1] < previous[1])) {
		fail(entry.line, `decision ${entry.key} is out of order`);
	}
	previous = here;
}
if (entries.length && entries[entries.length - 1].status.kind !== 'living' && living) {
	fail(living.line, `the living section is not the last entry`);
}

// 3. Every entry says what it is, and its topics are from the vocabulary.
for (const entry of entries) {
	const { status } = entry;
	if (status.kind === 'missing') fail(entry.line, `decision ${entry.key} has no **Status:** line`);
	if (status.kind === 'malformed') fail(entry.statusAt, `decision ${entry.key}'s **Status:** line does not parse`);
	if (!entry.topics || entry.topics.length === 0) fail(entry.line, `decision ${entry.key} has no **Topics:**`);
	else {
		for (const topic of entry.topics) {
			if (!TOPICS.includes(topic)) fail(entry.line, `decision ${entry.key}: unknown topic "${topic}"`);
		}
		if (new Set(entry.topics).size !== entry.topics.length) fail(entry.line, `decision ${entry.key}: duplicate topic`);
	}
	if (status.kind !== 'superseded') continue;
	for (const target of status.by) {
		if (!keys.has(target)) fail(entry.line, `decision ${entry.key} is superseded by ${target}, which does not exist`);
		else if (rank(target)[0] <= rank(entry.key)[0]) fail(entry.line, `decision ${entry.key} is superseded by ${target}, which is not later`);
	}
}

// 4. Reciprocity: an entry that says it supersedes decision N is named by N's
// status line. This is the check that would have caught the audit's thirty-two
// silent replacements.
for (const entry of entries) {
	if (entry.status.kind === 'living') continue;
	for (const paragraph of entry.body.join('\n').split(/\n\s*\n/)) {
		// A claim, not a mention: "supersedes decision N" / "superseding N".
		// The participle ("superseded") describes the record and is the status
		// line's job; prose about statuses must not read as a claim.
		if (!/supersedes?\b|superseding\b/i.test(paragraph)) continue;
		if (/^\*\*Status:\*\*/.test(paragraph.trim())) continue;
		for (const target of cited(paragraph)) {
			if (target === entry.key || !keys.has(target)) continue;
			const status = byKey.get(target).status;
			const named =
				status.kind === 'superseded' && status.by.includes(entry.key);
			if (!named) {
				fail(entry.statusAt ?? entry.line, `decision ${entry.key} supersedes ${target}, whose status line does not say so`);
			}
		}
	}
}

// 5. Every citation in the file resolves - and in the repository too: the
// numbers are cited from code, tests, docs and commit messages.
for (const entry of entries) {
	for (const target of cited(entry.body.join('\n'))) {
		if (!keys.has(target)) fail(entry.line, `decision ${entry.key} cites ${target}, which does not exist`);
	}
}
const tracked = execFileSync('git', ['ls-files'], { encoding: 'utf8' }).split('\n').filter(Boolean);
const readable = /\.(md|go|rs|js|mjs|ts|svelte|ps1|yml|yaml|toml|json|html|css|txt)$/;
for (const file of tracked) {
	if (!readable.test(file) || file === FILE) continue;
	let content;
	try {
		content = readFileSync(file, 'utf8');
	} catch {
		continue;
	}
	for (const match of content.matchAll(CITATION)) {
		for (const number of match[1].matchAll(NUMBER)) {
			if (!keys.has(number[0])) {
				const line = content.slice(0, match.index).split('\n').length;
				fail(null, `${file}:${line}: cites decision ${number[0]}, which does not exist`);
			}
		}
	}
}

// 6. The index below is generated, and it is fresh.
const index = () => {
	const out = ['## Index', ''];
	out.push('Generated by `scripts/decisions.mjs --write`. The status line of an entry is the source of truth; this table only mirrors it, so a blank status is an active entry.', '');
	out.push('### By number', '');
	out.push('| # | Decision | Status |');
	out.push('| --- | --- | --- |');
	for (const entry of [...entries].sort((a, b) => rank(a.key)[0] - rank(b.key)[0] || rank(a.key)[1].localeCompare(rank(b.key)[1]))) {
		const { status } = entry;
		const what =
			status.kind === 'active' ? 'active'
			: status.kind === 'living' ? 'living'
			: status.kind === 'superseded' ? `${status.inPart ? 'superseded in part' : 'superseded'} by ${status.by.join(', ')}`
			: 'unknown';
		const note = status.kind === 'living' ? ' *(living section, at the end of this file)*' : '';
		out.push(`| ${entry.key} | [${entry.title}](${slug(entry.key, entry.title)})${note} | ${status.kind === 'active' ? '' : what} |`);
	}
	out.push('', '### By topic', '');
	out.push('Only the entries that still bind. A topic lists what speaks today; the status line of each entry names what replaced it, if anything.', '');
	const live = entries.filter((entry) => entry.status.kind === 'active' || entry.status.kind === 'living');
	for (const topic of TOPICS) {
		const members = live.filter((entry) => entry.topics?.includes(topic)).sort((a, b) => rank(a.key)[0] - rank(b.key)[0] || rank(a.key)[1].localeCompare(rank(b.key)[1]));
		if (members.length === 0) continue;
		out.push(`**${topic}**`, '');
		for (const entry of members) out.push(`- [${entry.key}. ${entry.title}](${slug(entry.key, entry.title)})`);
		out.push('');
	}
	return out.join(eol);
};
const generated = index();
const start = lines.findIndex((line) => line.trim() === START);
const end = lines.findIndex((line) => line.trim() === END);
if (start === -1 || end === -1 || end < start) {
	fail(null, `the index block (${START} … ${END}) is missing`);
} else {
	const current = lines.slice(start + 1, end).join(eol).trim();
	if (current !== generated.trim()) fail(start + 1, 'the index is out of date - run node scripts/decisions.mjs --write');
}

// ------------------------------------------------------------------- the CLI
const argv = process.argv.slice(2);
const flag = (name) => {
	const at = argv.indexOf(name);
	return at === -1 ? null : argv[at + 1];
};

if (argv.includes('--write')) {
	if (start === -1 || end === -1) {
		console.error(`cannot write: ${START} / ${END} not found in ${FILE}`);
		process.exit(1);
	}
	const next = [...lines.slice(0, start + 1), ...generated.split(eol), ...lines.slice(end)];
	writeFileSync(FILE, next.join(eol));
	const other = failed.filter((line) => !line.includes('the index is out of date'));
	if (other.length > 0) {
		console.error(`\nindex written, but the record still has ${other.length} problem(s):\n`);
		for (const line of other) console.error(`  ${line}`);
		console.error('');
		process.exit(1);
	}
	console.log(`index written: ${entries.length} entries, ${entries.filter((e) => e.status.kind === 'active').length} active`);
	process.exit(0);
}

if (argv.includes('--list') || argv.includes('--json')) {
	const wantTopic = flag('--topic');
	const wantStatus = flag('--status');
	const rows = entries
		.filter((entry) => (wantTopic ? entry.topics?.includes(wantTopic) : true))
		.filter((entry) =>
			wantStatus === 'superseded' ? entry.status.kind === 'superseded'
			: wantStatus === 'active' ? entry.status.kind === 'active'
			: wantStatus === 'living' ? entry.status.kind === 'living'
			: true
		)
		.sort((a, b) => rank(a.key)[0] - rank(b.key)[0] || rank(a.key)[1].localeCompare(rank(b.key)[1]));
	if (argv.includes('--json')) {
		console.log(JSON.stringify(rows.map((entry) => ({ n: entry.key, title: entry.title, topics: entry.topics, line: entry.line, status: entry.status })), null, '\t'));
	} else {
		for (const entry of rows) {
			const what =
				entry.status.kind === 'active' ? 'active'
				: entry.status.kind === 'living' ? 'living'
				: entry.status.kind === 'superseded' ? `${entry.status.inPart ? 'in part' : 'superseded'} by ${entry.status.by.join(', ')}`
				: 'unknown';
			console.log(`${entry.key}\t${what}\t${entry.topics?.join(', ')}\t${entry.title}\tline ${entry.line}`);
		}
	}
	process.exit(0);
}

if (failed.length > 0) {
	console.error(`\nthe decision record has ${failed.length} problem(s):\n`);
	for (const line of failed) console.error(`  ${line}`);
	console.error('');
	process.exit(1);
}
console.log(
	`decision record: ${entries.length} entries, ${entries.filter((e) => e.status.kind === 'active').length} active, ` +
		`${entries.filter((e) => e.status.kind === 'superseded').length} superseded, every citation resolves, the index is fresh.`
);
