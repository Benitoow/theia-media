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
const assets = {};
for (const name of SETUPS) {
	const asset = published.get(name);
	if (!asset) {
		console.error(`${release.tag_name} has no asset named ${name}; release.json is unchanged.`);
		process.exit(1);
	}
	if (typeof asset.digest !== 'string' || !/^sha256:[0-9a-f]{64}$/.test(asset.digest)) {
		console.error(`${name} carries no SHA-256 digest on GitHub; release.json is unchanged.`);
		process.exit(1);
	}
	assets[name] = { size: asset.size, sha256: asset.digest.slice('sha256:'.length) };
}

const snapshot = {
	tag: release.tag_name,
	name: release.name,
	publishedAt: release.published_at,
	url: release.html_url,
	assets
};
writeFileSync(join(here, 'release.json'), `${JSON.stringify(snapshot, null, '\t')}\n`);
console.log(`release.json: ${snapshot.tag}, published ${snapshot.publishedAt}, ${SETUPS.length} setups with digests.`);
