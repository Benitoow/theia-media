#!/usr/bin/env node
// Structural publishing checks for every generated page of every locale and the
// static agent indexes. Browser acceptance remains separate: this cannot
// establish layout, fonts, motion or usability.
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
const releasesHistory = json("releases-history.json");
const reference = json("locales/en.json");
const major = Number(release.tag.replace(/^v/, "").split(".")[0]);
const visibleReleases = releasesHistory.filter((item) => major >= item.minMajor);
// Decisions 227, 230 and 237: the pre-release announcement exists exactly while
// the published release is older than the announced one.
const announcement = json("announcement.json");
const announcing = major < Number(announcement.version.split(".")[0]);
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
// single page had, now on the page that holds them (decisions 230 and 237).
const anchors = {
  home: ["top", "main", "discover", "demo", "built", "help"],
  download: ["top", "main", "download", "panel-windows", "panel-macos", "panel-linux"],
  changelog: [
    "top",
    "main",
    "changelog",
    "new",
    "help",
    ...["v5", "v4", "v3"]
      .filter((gen) => visibleReleases.some((item) => item.generation === gen))
      .map((gen) => `gen-${gen}`),
    ...visibleReleases.map((item) => item.tag.replaceAll(".", "-")),
  ],
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
  // Decision 237: Discord belongs in the persistent header (desktop and mobile)
  // on every page, and in the home hero action row.
  assert(
    html.includes('class="nav-discord" href="https://discord.gg/5E55F7XFZp"'),
    "missing Discord pill in desktop header",
  );
  // The announcement belongs to the home page (its pill) and the changelog page
  // (its section), and leaves both the day its version is published.
  const pill = ofTag("a").filter((tag) =>
    (tag.attributes.class ?? "").split(" ").includes("v5-pill"),
  );
  const dot = html.includes('class="nav-dot"');
  if (announcing) {
    assert(
      pill.length === (doc.page.id === "home" ? 1 : 0),
      "the announcement pill belongs to the home page, once",
    );
    assert(dot, "the navigation should mark the changelog page while announcing");
    assert(
      ids.has("v5") === (doc.page.id === "changelog"),
      "the announcement section belongs to the changelog page",
    );
    if (doc.page.id === "changelog") {
      assert(
        html.includes(`data-opens="${new Date(announcement.opens).toISOString()}"`) &&
          html.includes(
            `data-closes="${new Date(announcement.closes).toISOString()}"`,
          ),
        "announcement dates differ from announcement.json",
      );
      assert(
        html.indexOf('id="v5"') < html.indexOf('id="changelog"'),
        "the announcement must precede the release history",
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
    (tag) => tag.attributes.rel === "alternate" && "hreflang" in tag.attributes,
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
  const machineAlternates = ofTag("link").filter(
    (tag) => tag.attributes.rel === "alternate" && !("hreflang" in tag.attributes),
  );
  assert(
    machineAlternates.length === 4 &&
      machineAlternates.some((tag) => (tag.attributes.href ?? "").endsWith("llms.txt")) &&
      machineAlternates.some((tag) => (tag.attributes.href ?? "").endsWith("llms-full.txt")) &&
      machineAlternates.some((tag) => (tag.attributes.href ?? "").endsWith("releases.json")) &&
      machineAlternates.some(
        (tag) =>
          tag.attributes.href ===
          "https://github.com/Benitoow/theia-media/releases.atom",
      ),
    "missing machine-readable discovery links",
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
    const heroSection = html.slice(0, html.indexOf('id="discover"'));
    assert(
      heroSection.includes('href="https://discord.gg/5E55F7XFZp"'),
      "missing Discord button in home hero",
    );
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
  if (doc.page.id === "changelog") {
    const entries = ofTag("li").filter(
      (tag) => tag.attributes.class === "release-entry",
    );
    assert(
      entries.length === visibleReleases.length,
      `expected ${visibleReleases.length} changelog entries, got ${entries.length}`,
    );
  }
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
    const expectedType = software
      ? "SoftwareApplication"
      : doc.page.id === "questions"
        ? "FAQPage"
        : "WebPage";
    assert(
      ld.length === 1 &&
        data["@type"] === expectedType &&
        data.inLanguage === doc.lang &&
        data.url === canonical,
      "incorrect structured data",
    );
    if (software) {
      assert(
        Array.isArray(data.sameAs) &&
          data.sameAs.includes("https://discord.gg/5E55F7XFZp") &&
          data.sameAs.includes("https://github.com/Benitoow/theia-media") &&
          data.releaseNotes === `${siteUrl}/${doc.id}/changelog/`,
        "SoftwareApplication missing sameAs or releaseNotes",
      );
    }
    if (doc.page.id === "questions") {
      assert(
        Array.isArray(data.mainEntity) && data.mainEntity.length === 8,
        "FAQPage missing eight questions",
      );
    }
    if (doc.page.id === "changelog") {
      assert(
        data.mainEntity?.["@type"] === "ItemList" &&
          data.mainEntity?.itemListElement?.length === visibleReleases.length,
        "changelog JSON-LD missing release ItemList",
      );
    }
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
  const maxBytes = doc.page.id === "changelog" ? 260 * 1024 : 110 * 1024;
  assert(
    Buffer.byteLength(html) < maxBytes,
    `HTML exceeds ${Math.round(maxBytes / 1024)} KiB per page`,
  );
}

// Legacy /<language>/new/ redirect stubs forward cleanly to ../changelog/.
for (const language of languages) {
  const redirectFile = join(dist, language.id, "new", "index.html");
  check(
    existsSync(redirectFile),
    `missing legacy redirect: ${language.id}/new/index.html`,
  );
  if (existsSync(redirectFile)) {
    const redirectHtml = readFileSync(redirectFile, "utf8");
    check(
      redirectHtml.includes(
        `<link rel="canonical" href="${siteUrl}/${language.id}/changelog/">`,
      ) &&
        redirectHtml.includes(
          '<noscript><meta http-equiv="refresh" content="0; url=../changelog/"></noscript>',
        ) &&
        redirectHtml.includes('<meta name="robots" content="noindex">'),
      `${language.id}/new/index.html: invalid redirect metadata`,
    );
    const redirectStyle =
      /<style>([\s\S]*?)<\/style>/.exec(redirectHtml)?.[1] ?? "";
    check(
      !/src="https?:/i.test(redirectHtml) &&
        !/@import|url\(/i.test(redirectStyle),
      `${language.id}/new/index.html: unexpected remote resource`,
    );
  }
}

// Static agent indexes: llms.txt, llms-full.txt, releases.json and robots.txt.
for (const artifact of ["llms.txt", "llms-full.txt", "releases.json", "robots.txt"]) {
  const file = join(dist, artifact);
  check(existsSync(file), `missing root agent artifact: ${artifact}`);
  if (!existsSync(file)) continue;
  const content = readFileSync(file, "utf8");
  check(
    !/\.theia-private|CLAUDE\.md|AGENTS\.md|v5-implementation\.md/i.test(content),
    `${artifact}: leaks private path`,
  );
  if (artifact !== "robots.txt") {
    check(
      !/\bopen[- ]source\b|\bsource-available\b|\bcoming soon\b/i.test(content),
      `${artifact}: forbidden release or source claim`,
    );
    check(
      content.includes(release.tag) &&
        content.includes("https://discord.gg/5E55F7XFZp"),
      `${artifact}: missing current release tag or Discord link`,
    );
  }
}
if (existsSync(join(dist, "llms-full.txt"))) {
  const full = readFileSync(join(dist, "llms-full.txt"), "utf8");
  for (const item of visibleReleases)
    check(
      full.includes(`### ${item.tag} `),
      `llms-full.txt missing release ${item.tag}`,
    );
  for (const asset of Object.values(release.assets))
    check(full.includes(asset.sha256), "llms-full.txt missing setup SHA-256");
}
if (existsSync(join(dist, "releases.json"))) {
  try {
    const manifest = JSON.parse(readFileSync(join(dist, "releases.json"), "utf8"));
    check(
      manifest.latest?.tag === release.tag &&
        manifest.discord === "https://discord.gg/5E55F7XFZp" &&
        Boolean(manifest.upcoming) === announcing &&
        Array.isArray(manifest.releases) &&
        manifest.releases.length === visibleReleases.length &&
        manifest.releases.at(-1)?.tag === "v3.3.0",
      "releases.json out of sync with release.json or releases-history.json",
    );
  } catch (_) {
    failures.push("releases.json: invalid JSON");
  }
}
if (existsSync(join(dist, "robots.txt"))) {
  const robots = readFileSync(join(dist, "robots.txt"), "utf8");
  check(
    robots.includes(`Sitemap: ${siteUrl}/sitemap.xml`) &&
      robots.includes(`${siteUrl}/llms.txt`) &&
      robots.includes(`${siteUrl}/releases.json`),
    "robots.txt missing sitemap or agent index pointers",
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
    // old link to it then reaches the changelog page, which is where it should land.
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
  if (!/\.(?:html|txt|xml|json)$|og-image\.jpg$/.test(file))
    check(
      allReferences.has(file),
      `unreferenced asset ${relative(dist, file)}`,
    );
// Sixty-one documents share one stylesheet, so the budget is per page and for the
// shared assets, with a ceiling on the whole site (decisions 230 and 237).
const shared = files
  .filter(
    (file) =>
      !/\.(mp4|webm|html)$/.test(file) &&
      !["llms.txt", "llms-full.txt", "releases.json", "robots.txt", "sitemap.xml"].includes(
        relative(dist, file).replaceAll("\\", "/"),
      ),
  )
  .reduce((sum, file) => sum + statSync(file).size, 0);
const weight = files
  .filter((file) => !/\.(mp4|webm)$/.test(file))
  .reduce((sum, file) => sum + statSync(file).size, 0);
check(
  shared < 2 * 1024 * 1024,
  "shared assets (fonts, images, captions) exceed 2 MiB without videos",
);
check(
  weight < 10 * 1024 * 1024,
  "all pages, agent indexes and shared assets exceed 10 MiB without videos",
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
    `check: ${documents.length} pages (${pages.length} pages x ${languages.length} languages and the English root), ${languages.length} legacy redirects, agent indexes, ten complete catalogues/caption tracks, ${files.length} files, largest page ${(Math.max(...sizes) / 1024).toFixed(0)} KiB, ${(weight / 1024).toFixed(0)} KiB without videos; all checks passed`,
  );
