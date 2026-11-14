#!/usr/bin/env node
//
// Writes site/release.json: the facts about the latest GitHub release that the
// page prints (version, date, and the size and SHA-256 of the six setups).
//
//	node site/fetch-release.mjs
//
// It is the only place that talks to GitHub, it runs on the maintainer's
// machine or in a workflow, and the published page never calls an API: the build
// reads the snapshot. It refuses to write an incomplete one - a release that
// lacks one of the six setups or one digest - because a page that prints
// "SHA-256" next to a file must have it from GitHub, not from memory.

import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const REPO = 'Benitoow/theia-media';
const SETUPS = [
	'theia-setup-windows-amd64.exe',
	'theia-setup-windows-arm64.exe',
	'theia-setup-darwin-arm64',
	'theia-setup-darwin-amd64',
	'theia-setup-linux-amd64',
	'theia-setup-linux-arm64'
];
const ANDROID_APK_PATTERN = /^theia-android-\d+\.\d+\.\d+\.apk$/;

const here = dirname(fileURLToPath(import.meta.url));
const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
const response = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`, {
	headers: {
		Accept: 'application/vnd.github+json',
		'X-GitHub-Api-Version': '2022-11-28',
		'User-Agent': 'theia-site-release-snapshot',
		...(token ? { Authorization: `Bearer ${token}` } : {})
	}
});
if (!response.ok) {
	console.error(`GitHub answered ${response.status} ${response.statusText}; release.json is unchanged.`);
	process.exit(1);
}

const release = await response.json();
const published = new Map(release.assets.map((asset) => [asset.name, asset]));
// A release may leave a platform out (4.2.2 shipped no macOS or ARM setup while
// GitHub's runners were out of minutes, decision 182). That platform's setup is
// then the newest published release's that has one, named with its own tag, so
// the page never links a file that does not exist.
let older = null;
const loadOlder = async () => {
	if (older) return older;
	const list = await fetch(`https://api.github.com/repos/${REPO}/releases?per_page=30`, {
		headers: { Accept: 'application/vnd.github+json', 'User-Agent': 'theia-site-release-snapshot', ...(token ? { Authorization: `Bearer ${token}` } : {}) }
	});
	if (!list.ok) return null;
	older = (await list.json()).filter((one) => !one.draft && !one.prerelease);
	return older;
};
const fallback = async (matcher) => {
	const releases = await loadOlder();
	if (!releases) return null;
	for (const one of releases) {
		const asset = one.assets.find(matcher);
		if (asset) return { asset, tag: one.tag_name };
	}
	return null;
};
const assets = {};
const recordAsset = (asset, tag) => {
	if (typeof asset.digest !== 'string' || !/^sha256:[0-9a-f]{64}$/.test(asset.digest)) {
		console.error(`${asset.name} carries no SHA-256 digest on GitHub; release.json is unchanged.`);
		process.exit(1);
	}
	assets[asset.name] = { size: asset.size, sha256: asset.digest.slice('sha256:'.length), tag, url: asset.browser_download_url };
};
for (const name of SETUPS) {
	let asset = published.get(name);
	let tag = release.tag_name;
	if (!asset) {
		const found = await fallback((candidate) => candidate.name === name);
		if (found) ({ asset, tag } = found);
	}
	if (!asset) {
		console.error(`no published release has an asset named ${name}; release.json is unchanged.`);
		process.exit(1);
	}
	recordAsset(asset, tag);
}
const expectedApk = `theia-android-${release.tag_name.replace(/^v/, '')}.apk`;
let apkAsset = published.get(expectedApk) || release.assets.find((candidate) => ANDROID_APK_PATTERN.test(candidate.name));
let apkTag = release.tag_name;
if (!apkAsset) {
	const found = await fallback((candidate) => ANDROID_APK_PATTERN.test(candidate.name));
	if (found) ({ asset: apkAsset, tag: apkTag } = found);
}
if (!apkAsset) {
	console.error('no published release has a matching Android APK; release.json is unchanged.');
	process.exit(1);
}
recordAsset(apkAsset, apkTag);

const snapshot = {
	tag: release.tag_name,
	name: release.name,
	publishedAt: release.published_at,
	url: release.html_url,
	assets
};
writeFileSync(join(here, 'release.json'), `${JSON.stringify(snapshot, null, '\t')}\n`);
console.log(`release.json: ${snapshot.tag}, published ${snapshot.publishedAt}, ${Object.keys(assets).length} assets with digests.`);
