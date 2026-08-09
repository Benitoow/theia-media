// Produces the build-time release metadata consumed by build.mjs. This script
// runs in GitHub Actions; the published page itself never calls GitHub's API.

import { writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const output = resolve(process.argv[2] || 'site/release.generated.json');
const expected = new Map([
	['theia-windows-amd64.exe', ['windows', 'x64']],
	['theia-windows-arm64.exe', ['windows', 'ARM64']],
	['theia-darwin-arm64', ['macos', 'Apple Silicon']],
	['theia-darwin-amd64', ['macos', 'Intel']],
	['theia-linux-amd64', ['linux', 'x64']],
	['theia-linux-arm64', ['linux', 'ARM64']]
]);

const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
const response = await fetch('https://api.github.com/repos/Benitoow/theia-media/releases/latest', {
	headers: {
		Accept: 'application/vnd.github+json',
		'X-GitHub-Api-Version': '2022-11-28',
		'User-Agent': 'theia-site-build',
		...(token ? { Authorization: `Bearer ${token}` } : {})
	}
});

if (!response.ok) throw new Error(`GitHub release API returned ${response.status} ${response.statusText}.`);
const source = await response.json();
const byName = new Map(source.assets.map((asset) => [asset.name, asset]));
const assets = [];

for (const [file, [platform, architecture]] of expected) {
	const asset = byName.get(file);
	if (!asset) throw new Error(`Latest release ${source.tag_name} is missing ${file}.`);
	const digest = typeof asset.digest === 'string' && asset.digest.startsWith('sha256:')
		? asset.digest.slice('sha256:'.length)
		: undefined;
	assets.push({ file, platform, architecture, size: asset.size, ...(digest ? { sha256: digest } : {}) });
}

const release = {
	version: source.tag_name,
	publishedAt: source.published_at,
	url: source.html_url,
	assets
};

await writeFile(output, `${JSON.stringify(release, null, 2)}\n`);
console.log(`Release metadata ${release.version} written to ${output}.`);
