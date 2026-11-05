#!/usr/bin/env node
// Structural publishing checks for every generated page of every locale. Browser
// acceptance remains separate: this cannot establish layout, fonts, motion or
// usability.
import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { validateCatalogue } from "./build.mjs";
import { loadIcons } from "./icons.mjs";

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
const pages = json("pages.json");
const release = json("release.json");
const reference = json("locales/en.json");
// Decisions 227 and 230: the pre-release announcement exists exactly while the
// published release is older than the announced one.
const announcement = json("announcement.json");
const announcing =
  Number(release.tag.replace(/^v/, "").split(".")[0]) <
  Number(announcement.version.split(".")[0]);
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
// The ids each page must keep: the public anchors, and the sections the old
// single page had, now on the page that holds them (decision 230).
const anchors = {
  home: ["top", "main", "discover", "demo", "built", "help"],
  download: ["top", "main", "download", "panel-windows", "panel-macos", "panel-linux"],
  new: ["top", "main", "new", "help"],
  questions: [
    "top",
    "main",
    "faq",
    "ledger",
    "faq-files",
    "faq-price",
    "limits",
    "faq-processor",
    "faq-install",
    "privacy",
    "faq-friends",
    "faq-help",
  ],
  privacy: ["top", "main", "leaves", "drive", "full", "website"],
  about: ["top", "main", "licence", "built", "credits"],
};
const allReferences = new Set();
const sizes = [];

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
for (const page of pages)
  check(!!anchors[page.id], `no expected anchors for page ${page.id}`);

const documents = [
  {
    ...languages.find((language) => language.id === "en"),
    page: pages[0],
    path: "index.html",
    entry: true,
  },
  ...languages.flatMap((language) =>
    pages.map((page) => ({
      ...language,
      page,
      path: `${language.id}${page.slug ? `/${page.slug}` : ""}/index.html`,
    })),
  ),
];
const pathOf = (languageId, page) =>
  `${languageId}${page.slug ? `/${page.slug}` : ""}/index.html`;

for (const doc of documents) {
  const file = join(dist, doc.path);
  if (!existsSync(file)) {
    failures.push(`missing page: ${doc.path}`);
    continue;
  }
  const html = readFileSync(file, "utf8");
  sizes.push(Buffer.byteLength(html));
  const assert = (condition, message) =>
    check(condition, `${doc.path}: ${message}`);
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
  const slug = doc.page.slug;
  assert(
    document?.lang === doc.lang && document?.dir === doc.dir,
    "incorrect lang/dir",
  );
  assert(
    document?.["data-entry"] === String(!!doc.entry),
    "incorrect negotiation boundary",
  );
  assert(document?.["data-page"] === doc.page.id, "incorrect data-page");
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
  for (const id of anchors[doc.page.id] ?? [])
    assert(ids.has(id), `missing anchor ${id}`);
  // The announcement belongs to the home page (its pill) and the news page (its
  // section), and leaves both the day its version is published.
  const pill = ofTag("a").filter((tag) =>
    (tag.attributes.class ?? "").split(" ").includes("v5-pill"),
  );
  const dot = html.includes('class="nav-dot"');
  if (announcing) {
    assert(
      pill.length === (doc.page.id === "home" ? 1 : 0),
      "the announcement pill belongs to the home page, once",
    );
    assert(dot, "the navigation should mark the news page while announcing");
    assert(
      ids.has("v5") === (doc.page.id === "new"),
      "the announcement section belongs to the news page",
    );
    if (doc.page.id === "new") {
      assert(
        html.includes(`data-opens="${new Date(announcement.opens).toISOString()}"`) &&
          html.includes(
            `data-closes="${new Date(announcement.closes).toISOString()}"`,
          ),
        "announcement dates differ from announcement.json",
      );
      assert(
        html.indexOf('id="v5"') < html.indexOf('id="new"'),
        "the announcement must precede the release highlights",
      );
    }
  } else {
    assert(
      !ids.has("v5") && pill.length === 0 && !dot,
      "the announcement must leave once its version is published",
    );
  }
  const canonical = `${siteUrl}/${doc.id}${slug ? `/${slug}` : ""}/`;
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
  assert(meta("og:locale") === doc.og, "incorrect Open Graph locale");
  assert(
    meta("twitter:card") === "summary_large_image",
    "missing large share card",
  );
  const title = /<title>([^<]+)<\/title>/.exec(html)?.[1];
  assert(
    title?.length >= 10 && title.length <= 100,
    "title is missing or too long",
  );
  assert(title === meta("og:title"), "title and Open Graph title differ");
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
          tag.attributes.href ===
            `${siteUrl}/${other.id}${slug ? `/${slug}` : ""}/`,
      ),
      `missing alternate ${other.id}`,
    );
  assert(
    alternates.some(
      (tag) =>
        tag.attributes.hreflang === "x-default" &&
        tag.attributes.href ===
          (doc.page.id === "home" ? `${siteUrl}/` : `${siteUrl}/en/${slug}/`),
    ),
    "missing default alternate",
  );
  const reached = new Set();
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
      if (value.split("#")[0].endsWith("/") || value.split("#")[0] === "")
        target = join(target, "index.html");
      assert(
        target.startsWith(dist + "/") || target.startsWith(dist + "\\"),
        `reference escapes output: ${value}`,
      );
      assert(existsSync(target), `missing local resource ${value}`);
      allReferences.add(target);
      if (tag.name === "a") reached.add(relative(dist, target).replaceAll("\\", "/"));
      // A same-page link with a section on another page must name a real id there.
      if (tag.name === "a" && value.includes("#") && existsSync(target)) {
        const section = value.split("#")[1];
        if (target !== file && target.endsWith("index.html") && section)
          assert(
            readFileSync(target, "utf8").includes(`id="${section}"`),
            `${value} names a section the page does not have`,
          );
      }
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
  // Every page reaches every other page, in its own language, and the language
  // menus (one in the header, one in the footer) reach this same page in each.
  const reading = doc.entry ? "en" : doc.id;
  for (const page of pages)
    assert(
      reached.has(pathOf(reading, page)),
      `no link to the ${page.id} page`,
    );
  const links = ofTag("a");
  const menu = links.filter((tag) => tag.attributes["data-language"]);
  assert(menu.length === 20, "expected two language menus of ten links");
  for (const link of menu) {
    const target = resolve(dirname(file), link.attributes.href).replaceAll("\\", "/");
    assert(
      join(target, "index.html").replaceAll("\\", "/") ===
        join(dist, pathOf(link.attributes["data-language"], doc.page)).replaceAll("\\", "/"),
      `language link ${link.attributes["data-language"]} leaves the page`,
    );
  }
  assert(
    links.filter((tag) => tag.attributes["aria-current"] === "page").length >= 1,
    "the current page is not marked",
  );
  for (const code of ofTag("code"))
    assert(
      code.attributes.dir === "ltr",
      "technical value is not explicitly LTR",
    );
  // Media belongs to the home page: the decorative hero and the real recording.
  const videos = ofTag("video");
  if (doc.page.id === "home") {
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
        doc.lang,
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
      ofTag("img").filter((tag) => /shot-/.test(tag.attributes.src ?? ""))
        .length === 3,
      "expected the three product moments",
    );
  } else {
    assert(videos.length === 0, "only the home page carries video");
  }
  const faqItems = ofTag("details").filter(
    (tag) => tag.attributes.class === "faq-item",
  ).length;
  assert(
    faqItems === (doc.page.id === "questions" ? 8 : 0),
    "the eight native FAQ disclosures belong to the questions page",
  );
  if (doc.page.id === "download") {
    for (const [filename, asset] of Object.entries(release.assets))
      assert(
        links.some(
          (tag) =>
            tag.attributes.href === asset.url &&
            tag.attributes.href.endsWith("/" + filename),
        ),
        `download lost its actual release: ${filename}`,
      );
  } else {
    assert(
      !links.some((tag) => /\/releases\/download\//.test(tag.attributes.href ?? "")),
      "a download link outside the download page",
    );
  }
  const ld = [
    ...html.matchAll(
      /<script type="application\/ld\+json">([\s\S]*?)<\/script>/g,
    ),
  ];
  try {
    const data = JSON.parse(ld[0]?.[1]);
    const software = doc.page.id === "home" || doc.page.id === "download";
    assert(
      ld.length === 1 &&
        data["@type"] === (software ? "SoftwareApplication" : "WebPage") &&
        data.inLanguage === doc.lang &&
        data.url === canonical,
      "incorrect structured data",
    );
    assert(
      !data.aggregateRating && !data.review && (!software || data.offers?.price === "0"),
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
  if (["about", "questions"].includes(doc.page.id))
    assert(
      text.includes("PolyForm Noncommercial 1.0.0") && text.includes("GPL-3.0"),
      "licence names missing",
    );
  if (doc.id === "en") {
    assert(
      !/\bopen[- ]source\b|\bsource-available\b|\bcoming soon\b/i.test(text),
      "forbidden release or source claim",
    );
    assert(text.includes("sample library"), "provenance limit missing");
    if (doc.page.id === "questions")
      assert(
        /source code is developed privately/.test(text),
        "private-source disclosure missing",
      );
    if (["download", "questions"].includes(doc.page.id))
      assert(text.includes("unverified"), "validation limits missing");
    if (doc.page.id === "about")
      assert(
        /Simple Icons/.test(text) && /Lucide/.test(text),
        "icon credits missing",
      );
  }
  assert(
    Buffer.byteLength(html) < 110 * 1024,
    "HTML exceeds 110 KiB per page",
  );
}

// Icons: every vendored file matches its recorded digest and the whitelist.
try {
  const icons = loadIcons();
  check(icons.size > 0, "no icons");
} catch (error) {
  failures.push(error.message);
}
for (const name of [
  "LICENSE-lucide.txt",
  "LICENSE-simple-icons.txt",
  "LICENSE-svg-logos.txt",
  "DISCLAIMER-simple-icons.txt",
])
  check(
    existsSync(join(dist, "assets/icons", name)),
    `icon licence text not shipped: ${name}`,
  );

// An old address keeps working: each section the single page had names the page
// that holds it now (assets/js/language.js), and that page has the id.
try {
  const script = readFileSync(join(here, "assets/js/language.js"), "utf8");
  const moved = new Function(
    `return ${/var moved = (\{[\s\S]*?\});/.exec(script)[1]}`,
  )();
  for (const [anchor, pageId] of Object.entries(moved)) {
    // #v5 leaves with the announcement the day its version is published (227): an
    // old link to it then reaches the news page, which is where it should land.
    if (anchor === "v5" && !announcing) continue;
    const page = pages.find((item) => item.id === pageId);
    const target = join(dist, pathOf("en", page));
    check(
      existsSync(target) && readFileSync(target, "utf8").includes(`id="${anchor}"`),
      `the old section #${anchor} is not on the ${pageId} page`,
    );
  }
} catch (_) {
  failures.push("could not read the moved-anchor table from language.js");
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
    pages
      .map((page) => readFileSync(join(dist, pathOf(language.id, page)), "utf8"))
      .join("");
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
  for (const page of pages)
    check(
      sitemap.includes(
        `<loc>${siteUrl}/${language.id}${page.slug ? `/${page.slug}` : ""}/</loc>`,
      ),
      `sitemap missing ${language.id} ${page.id}`,
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
// Sixty-one documents share one stylesheet, so the budget is per page and for the
// shared assets, with a ceiling on the whole site.
const shared = files
  .filter((file) => !/\.(mp4|webm|html)$/.test(file))
  .reduce((sum, file) => sum + statSync(file).size, 0);
const weight = files
  .filter((file) => !/\.(mp4|webm)$/.test(file))
  .reduce((sum, file) => sum + statSync(file).size, 0);
check(
  shared < 2 * 1024 * 1024,
  "shared assets (fonts, images, captions) exceed 2 MiB without videos",
);
check(
  weight < 8 * 1024 * 1024,
  "all pages and shared assets exceed 8 MiB without videos",
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
    `check: ${documents.length} pages (${pages.length} pages x ${languages.length} languages and the English root), ten complete catalogues/caption tracks, ${files.length} files, largest page ${(Math.max(...sizes) / 1024).toFixed(0)} KiB, ${(weight / 1024).toFixed(0)} KiB without videos; all checks passed`,
  );
