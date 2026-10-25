#!/usr/bin/env node
// Structural publishing checks for every generated locale. Browser acceptance
// remains separate: this cannot establish layout, fonts, motion or usability.
import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { validateCatalogue } from "./build.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const dist = join(here, "dist");
const json = (path) => JSON.parse(readFileSync(join(here, path), "utf8"));
const argv = process.argv.slice(2);
const given = argv.indexOf("--site-url");
const siteUrl = (
  given >= 0
    ? argv[given + 1]
    : process.env.SITE_URL || json("site.config.json").siteUrl
).replace(/\/+$/, "");
const languages = json("locales/languages.json");
const release = json("release.json");
const reference = json("locales/en.json");
const failures = [];
const check = (condition, message) => {
  if (!condition) failures.push(message);
};
const hosts = new Set([
  "github.com",
  "discord.gg",
  "polyformproject.org",
  "www.gnu.org",
  "www.themoviedb.org",
]);
const allHTML = [];
const allReferences = new Set();

for (const language of languages) {
  try {
    validateCatalogue(
      reference,
      json(`locales/${language.id}.json`),
      language.id,
    );
  } catch (error) {
    failures.push(error.message);
  }
}

const pages = [
  { ...languages.find(language => language.id === 'en'), path: "index.html", entry: true },
  ...languages.map((language) => ({
    ...language,
    path: `${language.id}/index.html`,
  })),
];
for (const language of pages) {
  const file = join(dist, language.path);
  if (!existsSync(file)) {
    failures.push(`missing page: ${language.path}`);
    continue;
  }
  const html = readFileSync(file, "utf8");
  allHTML.push(html);
  const assert = (condition, message) =>
    check(condition, `${language.path}: ${message}`);
  const tags = [
    ...html.matchAll(
      /<([a-zA-Z][a-zA-Z0-9-]*)((?:\s+[^\s=>"']+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+))?)*)\s*\/?>/g,
    ),
  ].map((match) => {
    const attributes = {};
    for (const value of match[2].matchAll(
      /([^\s=>"']+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?/g,
    ))
      attributes[value[1].toLowerCase()] =
        value[2] ?? value[3] ?? value[4] ?? "";
    return { name: match[1].toLowerCase(), attributes };
  });
  const ofTag = (name) => tags.filter((tag) => tag.name === name);
  const meta = (key) =>
    ofTag("meta").find(
      (tag) => tag.attributes.name === key || tag.attributes.property === key,
    )?.attributes.content;
  const document = ofTag("html")[0]?.attributes;
  assert(
    document?.lang === language.lang && document?.dir === language.dir,
    "incorrect lang/dir",
  );
  assert(
    document?.["data-entry"] === String(!!language.entry),
    "incorrect negotiation boundary",
  );
  assert(!html.includes("{{"), "unresolved template token");
  assert(ofTag("h1").length === 1, "expected exactly one h1");
  let depth = 0;
  for (const heading of tags.filter((tag) => /^h[1-6]$/.test(tag.name))) {
    const level = Number(heading.name[1]);
    assert(level <= depth + 1, `heading jumps from ${depth} to ${level}`);
    depth = level;
  }
  const ids = new Set();
  for (const tag of tags) {
    if (!tag.attributes.id) continue;
    assert(!ids.has(tag.attributes.id), `duplicate id ${tag.attributes.id}`);
    ids.add(tag.attributes.id);
  }
  for (const id of [
    "top",
    "main",
    "demo",
    "download",
    "new",
    "ledger",
    "limits",
    "licence",
    "help",
    "faq",
    "privacy",
  ])
    assert(ids.has(id), `missing public anchor ${id}`);
  assert(
    html.indexOf('id="download"') < html.indexOf('id="new"'),
    "downloads must precede release highlights",
  );
  const canonical = `${siteUrl}/${language.id}/`;
  assert(
    ofTag("link").find((tag) => tag.attributes.rel === "canonical")?.attributes
      .href === canonical,
    "incorrect canonical",
  );
  assert(meta("og:url") === canonical, "incorrect Open Graph URL");
  for (const key of [
    "description",
    "og:locale",
    "og:title",
    "og:description",
    "og:image:alt",
    "twitter:title",
    "twitter:description",
    "twitter:image:alt",
  ])
    assert(!!meta(key), `missing ${key}`);
  assert(meta("og:locale") === language.og, "incorrect Open Graph locale");
  assert(
    meta("twitter:card") === "summary_large_image",
    "missing large share card",
  );
  const title = /<title>([^<]+)<\/title>/.exec(html)?.[1];
  assert(
    title?.length >= 10 && title.length <= 100,
    "title is missing or too long",
  );
  const alternates = ofTag("link").filter(
    (tag) => tag.attributes.rel === "alternate",
  );
  assert(
    alternates.length === 11,
    "expected ten reciprocal alternates and x-default",
  );
  for (const other of languages)
    assert(
      alternates.some(
        (tag) =>
          tag.attributes.hreflang === other.lang &&
          tag.attributes.href === `${siteUrl}/${other.id}/`,
      ),
      `missing alternate ${other.id}`,
    );
  assert(
    alternates.some(
      (tag) =>
        tag.attributes.hreflang === "x-default" &&
        tag.attributes.href === `${siteUrl}/`,
    ),
    "missing neutral entry alternate",
  );
  for (const tag of tags) {
    for (const key of ["href", "src", "poster", "data-src"]) {
      const value = tag.attributes[key];
      if (value === undefined) continue;
      if (value.startsWith("#")) {
        assert(ids.has(value.slice(1)), `missing fragment ${value}`);
        continue;
      }
      if (/^(?:https?:)?\/\//.test(value)) {
        const url = new URL(value);
        const externalResource =
          tag.name !== "a" &&
          !(
            tag.name === "link" &&
            ["canonical", "alternate"].includes(tag.attributes.rel)
          );
        assert(!externalResource, `remote subresource: ${value}`);
        if (tag.name === "a")
          assert(
            url.protocol === "https:" && hosts.has(url.hostname),
            `unapproved external link ${value}`,
          );
        continue;
      }
      assert(
        !/^(?:javascript|data|mailto):/i.test(value),
        `unexpected URI ${value}`,
      );
      let target = resolve(dirname(file), value.split("#")[0].split("?")[0]);
      if (value.endsWith("/")) target = join(target, "index.html");
      assert(
        target.startsWith(dist + "/") || target.startsWith(dist + "\\"),
        `reference escapes output: ${value}`,
      );
      assert(existsSync(target), `missing local resource ${value}`);
      allReferences.add(target);
    }
  }
  for (const match of html.matchAll(/url\(['"]?([^)'"]+)['"]?\)/g)) {
    const path = resolve(dirname(file), match[1]);
    assert(
      !/^(?:https?:)?\/\//.test(match[1]) && existsSync(path),
      `missing or remote CSS resource ${match[1]}`,
    );
    allReferences.add(path);
  }
  assert(!/@import/.test(html), "CSS import is not allowed");
  for (const tag of ["iframe", "embed", "object", "audio"])
    assert(ofTag(tag).length === 0, `${tag} is not allowed`);
  for (const image of ofTag("img"))
    assert(
      "alt" in image.attributes &&
        image.attributes.width &&
        image.attributes.height,
      "image needs alt and dimensions",
    );
  const videos = ofTag("video");
  assert(videos.length === 2, "expected decorative hero plus recording");
  const hero = videos.find(
    (tag) => tag.attributes.id === "hero-film",
  )?.attributes;
  const tour = videos.find((tag) => tag.attributes.id === "tour")?.attributes;
  assert(
    hero?.preload === "none" &&
      hero["aria-hidden"] === "true" &&
      ["autoplay", "muted", "loop", "playsinline"].every((key) => key in hero),
    "hero must start as a silent optional poster",
  );
  assert(
    tour &&
      "controls" in tour &&
      !("autoplay" in tour) &&
      tour.preload === "none",
    "recording must be user initiated and avoid an initial media fetch",
  );
  const heroSources = ofTag("source").filter(
    (tag) => "data-src" in tag.attributes,
  );
  assert(
    heroSources.length === 2 && heroSources.every((tag) => !tag.attributes.src),
    "hero sources must remain deferred",
  );
  assert(
    ofTag("button").some(
      (tag) => tag.attributes["aria-controls"] === "hero-film",
    ),
    "missing pause control",
  );
  const tracks = ofTag("track");
  assert(
    tracks.length === 10 &&
      tracks.filter((tag) => "default" in tag.attributes).length === 1,
    "expected ten captions and one default",
  );
  assert(
    tracks.find((tag) => "default" in tag.attributes)?.attributes.srclang ===
      language.lang,
    "caption default does not follow locale",
  );
  for (const track of tracks) {
    const path = resolve(dirname(file), track.attributes.src);
    if (existsSync(path)) {
      const content = readFileSync(path, "utf8");
      assert(
        content.startsWith("WEBVTT\n") &&
          (content.match(/ --> /g) || []).length === 13,
        "invalid caption cues",
      );
    }
  }
  assert(
    ofTag("details").filter((tag) => tag.attributes.class === "faq-item")
      .length === 8,
    "expected eight native FAQ disclosures",
  );
  const links = ofTag("a");
  assert(
    links.filter((tag) => tag.attributes["data-language"]).length === 10,
    "missing language links",
  );
  for (const [filename, asset] of Object.entries(release.assets))
    assert(
      links.some(
        (tag) =>
          tag.attributes.href === asset.url &&
          tag.attributes.href.endsWith("/" + filename),
      ),
      `download lost its actual release: ${filename}`,
    );
  for (const code of ofTag("code"))
    assert(
      code.attributes.dir === "ltr",
      "technical value is not explicitly LTR",
    );
  const ld = [
    ...html.matchAll(
      /<script type="application\/ld\+json">([\s\S]*?)<\/script>/g,
    ),
  ];
  try {
    const data = JSON.parse(ld[0]?.[1]);
    assert(
      ld.length === 1 &&
        data["@type"] === "SoftwareApplication" &&
        data.inLanguage === language.lang &&
        data.url === canonical,
      "incorrect structured data",
    );
    assert(
      data.offers?.price === "0" && !data.aggregateRating && !data.review,
      "structured data invents a price or review",
    );
  } catch (_) {
    assert(false, "invalid JSON-LD");
  }
  const text = html
    .replace(/<script[\s\S]*?<\/script>|<style[\s\S]*?<\/style>/g, "")
    .replace(/<[^>]*>/g, " ")
    .replace(/\s+/g, " ");
  assert(
    text.includes(
      "This product uses the TMDB API but is not endorsed or certified by TMDB.",
    ),
    "TMDB legal attribution changed",
  );
  assert(
    text.includes("PolyForm Noncommercial 1.0.0") && text.includes("GPL-3.0"),
    "licence names missing",
  );
  if (language.id === "en") {
    assert(
      /source code is developed privately/.test(text),
      "private-source disclosure missing",
    );
    assert(
      !/\bopen[- ]source\b|\bsource-available\b|\bcoming soon\b/i.test(text),
      "forbidden release or source claim",
    );
    assert(
      text.includes("sample library") && text.includes("unverified"),
      "provenance or validation limits missing",
    );
  }
  assert(
    Buffer.byteLength(html) < 110 * 1024,
    "HTML exceeds 110 KiB per locale",
  );
}

const coverage = json("assets/fonts/coverage.json");
const scriptFor = (code) =>
  code >= 0x600 && code <= 0x8ff
    ? "noto-arabic"
    : code >= 0x900 && code <= 0x97f
      ? "noto-devanagari"
      : code >= 0x980 && code <= 0x9ff
        ? "noto-bengali"
        : (code >= 0x2e80 && code <= 0x9fff) ||
            (code >= 0xff00 && code <= 0xffef)
          ? "noto-chinese"
          : null;
const glyphs = Object.fromEntries(
  Object.entries(coverage).map(([name, data]) => [
    name,
    new Set(data.codepoints),
  ]),
);
for (const [name, data] of Object.entries(coverage))
  check(
    createHash("sha256")
      .update(readFileSync(join(here, `assets/fonts/${name}.woff2`)))
      .digest("hex") === data.sha256,
    `font digest changed: ${name}`,
  );
for (const language of languages) {
  const content =
    JSON.stringify(json(`locales/${language.id}.json`)) +
    JSON.stringify(languages) +
    readFileSync(join(dist, language.id, "index.html"), "utf8");
  for (const character of new Set(content)) {
    const code = character.codePointAt(0);
    const script = scriptFor(code);
    if (script)
      check(
        glyphs[script].has(code),
        `${language.id}: missing glyph U+${code.toString(16)}; regenerate with scripts/site-fonts.py`,
      );
  }
}
const sitemap = readFileSync(join(dist, "sitemap.xml"), "utf8");
for (const language of languages)
  check(
    sitemap.includes(`<loc>${siteUrl}/${language.id}/</loc>`),
    `sitemap missing ${language.id}`,
  );
const files = [];
const walk = (directory) => {
  for (const name of readdirSync(directory)) {
    const path = join(directory, name);
    if (statSync(path).isDirectory()) walk(path);
    else files.push(path);
  }
};
walk(dist);
for (const file of files)
  if (!/\.(?:html|txt|xml)$|og-image\.jpg$/.test(file))
    check(
      allReferences.has(file),
      `unreferenced asset ${relative(dist, file)}`,
    );
const weight = files
  .filter((file) => !/\.(mp4|webm)$/.test(file))
  .reduce((sum, file) => sum + statSync(file).size, 0);
check(
  weight < 3 * 1024 * 1024,
  "all locales and shared assets exceed 3 MiB without videos",
);
const card = readFileSync(join(dist, "assets/img/og-image.jpg"));
check(card.length < 600 * 1024, "share card exceeds 600 KiB");
if (failures.length) {
  console.error(
    `check: ${failures.length} problem(s)\n${[...new Set(failures)].map((failure) => `  - ${failure}`).join("\n")}`,
  );
  process.exitCode = 1;
} else
  console.log(
    `check: ${pages.length} pages, ten complete catalogues/caption tracks, ${files.length} files, ${(weight / 1024).toFixed(0)} KiB without videos; all checks passed`,
  );
