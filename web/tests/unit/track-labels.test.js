// Run with `npm run test:unit`, which is node's own test runner: no browser and
// no dependency. These functions could not be tested at all while they lived
// inside a 1800-line component.
import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
	audioLabel,
	channelLabel,
	detailOf,
	languageName,
	looksLikeCommentary,
	subtitleLabel,
	titleIsShortEnough
} from '../../src/lib/track-labels.js';

// A stand-in for the real catalogue, holding only what these functions read.
const t = {
	languages: { fre: 'Français', eng: 'Anglais' },
	film: { audio: { unnamed: (n) => `Piste ${n}` } },
	player: {
		tracks: {
			channels: { stereo: 'Stéréo', mono: 'Mono' },
			commentary: 'Commentaire',
			forced: 'forcés',
			external: 'fichier joint',
			unnamedSubtitle: (n) => `Sous-titres ${n}`
		}
	}
};

test('a channel layout drops the mixing desk part', () => {
	assert.equal(channelLabel('5.1(side)', t), '5.1');
	assert.equal(channelLabel('7.1', t), '7.1');
	assert.equal(channelLabel('stereo', t), 'Stéréo');
	assert.equal(channelLabel('', t), null);
});

test('the catalogue names the language, and an unknown code says itself', () => {
	assert.equal(languageName('fre', t), 'Français');
	assert.equal(languageName('zxx', t), 'ZXX');
});

test('a detail that repeats the line above it is dropped', () => {
	// The fault this exists for: a French track titled "Français" rendered as
	// "Français · Français".
	assert.equal(detailOf(['Français', '5.1'], 'Français'), '5.1');
	assert.equal(detailOf(['français'], 'Français'), '');
	assert.equal(detailOf([null, '', 'AC3'], 'Français'), 'AC3');
});

test('commentary is recognised in both languages, and only from the title', () => {
	assert.ok(looksLikeCommentary("Director's Commentary"));
	assert.ok(looksLikeCommentary('Commentaire audio'));
	assert.ok(!looksLikeCommentary('Version Française'));
	assert.ok(!looksLikeCommentary(null));
});

test('a title too long for one line is left out of the detail', () => {
	assert.ok(titleIsShortEnough('VFF'));
	assert.ok(!titleIsShortEnough('A title far too long to sit on a single line of a menu'));
	assert.ok(!titleIsShortEnough(''));
});

test('an audio track is named by its language, described by the rest', () => {
	const label = audioLabel({ language: 'fre', codec: 'ac3', channels: '5.1(side)' }, 0, t);
	assert.equal(label.primary, 'Français');
	assert.equal(label.detail, '5.1 · AC3');
});

test('a commentary track says so instead of listing its codec', () => {
	const label = audioLabel(
		{ language: 'eng', title: "Director's Commentary", codec: 'aac', channels: 'stereo' },
		0,
		t
	);
	assert.equal(label.primary, 'Anglais');
	assert.equal(label.detail, 'Commentaire · Stéréo');
});

test('a track with no language falls back to its title, then to a number', () => {
	assert.equal(audioLabel({ title: 'VFF', codec: 'ac3' }, 0, t).primary, 'VFF');
	assert.equal(audioLabel({ codec: 'ac3' }, 2, t).primary, 'Piste 3');
});

test('a subtitle track carries forced and external as details', () => {
	const label = subtitleLabel(
		{ language: 'fre', title: 'FR Forced', is_forced: true, is_external: true },
		0,
		t
	);
	assert.equal(label.primary, 'Français');
	assert.equal(label.detail, 'FR Forced · forcés · fichier joint');
});

test('an unnamed subtitle track is numbered', () => {
	assert.equal(subtitleLabel({}, 1, t).primary, 'Sous-titres 2');
});
