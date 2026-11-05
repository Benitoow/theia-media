#!/usr/bin/env node
// One layout, six page templates, ten strict catalogues, no runtime service or
// build dependency.
// node site/build.mjs --check [--site-url https://example.org/theia]
import {
  cpSync,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { spawnSync } from "node:child_process";
import { loadIcons, spriteFor } from "./icons.mjs";

const here = dirname(fileURLToPath(import.meta.url));
export const escapeHTML = (value) =>
  String(value).replace(
    /[&<>"']/g,
    (character) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        character
      ],
  );

export function validateCatalogue(reference, value, at = "catalogue") {
  if (typeof reference === "string") {
    if (typeof value !== "string" || !value.trim())
      throw new Error(`${at}: expected nonempty text`);
    if (/[<>]|\{\{/.test(value))
      throw new Error(
        `${at}: catalogues contain text, not markup or template tokens`,
      );
  } else if (Array.isArray(reference)) {
    if (!Array.isArray(value) || value.length !== reference.length)
      throw new Error(`${at}: expected ${reference.length} entries`);
    reference.forEach((item, index) =>
      validateCatalogue(item, value[index], `${at}.${index}`),
    );
  } else {
    if (!value || typeof value !== "object" || Array.isArray(value))
      throw new Error(`${at}: expected an object`);
    const keys = Object.keys(reference).sort();
    if (JSON.stringify(keys) !== JSON.stringify(Object.keys(value).sort()))
      throw new Error(`${at}: catalogue keys differ`);
    keys.forEach((key) =>
      validateCatalogue(reference[key], value[key], `${at}.${key}`),
    );
  }
}

/**
 * A relative link from one output folder to another ("" is the site root), so
 * the pages work under a project path as well as at a domain root.
 */
export function relativeHref(fromFolder, toFolder, fragment = "") {
  const from = fromFolder.split("/").filter(Boolean);
  const to = toFolder.split("/").filter(Boolean);
  let shared = 0;
  while (
    shared < from.length &&
    shared < to.length &&
    from[shared] === to[shared]
  )
    shared += 1;
  const down = to.slice(shared);
  const up = from.length - shared;
  if (up === 0 && down.length === 0) return fragment ? `#${fragment}` : "./";
  const path = [...Array(up).fill(".."), ...down].join("/");
  return `${path ? `${path}/` : "./"}${fragment ? `#${fragment}` : ""}`;
}

const GITHUB = "https://github.com/Benitoow/theia-media";
const DISCORD = "https://discord.gg/5E55F7XFZp";
const FIELD_REPORT = `${GITHUB}/issues/10`;
const FEED = `${GITHUB}/releases.atom`;
// The icons that stand for each announced item, in the order of the catalogue.
const V5_ICONS = {
  features: [
    "layers",
    "profile",
    "googledrive",
    "nas",
    "server",
    "title",
    "watched",
    "update",
  ],
  later: ["phone", "tv", "chip"],
};
const MOMENT_ICONS = ["library", "player", "friends"];
const BUILT_WITH = [
  ["ffmpeg", "FFmpeg"],
  ["mpv", "mpv"],
  ["sqlite", "SQLite"],
  ["go", "Go"],
  ["svelte", "Svelte"],
];
// Page switches the script toggles after load: always in the sprite.
const SCRIPTED_ICONS = ["play", "pause"];

export function build(argv = process.argv.slice(2)) {
  const flag = (name) => {
    const index = argv.indexOf(name);
    return index < 0 ? undefined : argv[index + 1];
  };
  const json = (path) => JSON.parse(readFileSync(join(here, path), "utf8"));
  const raw =
    flag("--site-url") ||
    process.env.SITE_URL ||
    json("site.config.json").siteUrl;
  const address = new URL(raw);
  const loopback = ["localhost", "127.0.0.1", "[::1]"].includes(
    address.hostname,
  );
  if (
    address.protocol !== "https:" &&
    !(loopback && address.protocol === "http:")
  )
    throw new Error("site URL must be HTTPS (HTTP only on loopback)");
  if (address.search || address.hash || address.username || address.password)
    throw new Error("site URL must have no query, fragment or credentials");
  const siteUrl = `${address.origin}${address.pathname.replace(/\/+$/, "")}`;
  const languages = json("locales/languages.json");
  if (
    languages.length !== 10 ||
    new Set(languages.map((language) => language.id)).size !== 10
  )
    throw new Error("expected ten unique languages");
  const pages = json("pages.json");
  if (
    pages[0]?.id !== "home" ||
    pages[0].slug !== "" ||
    new Set(pages.map((page) => page.id)).size !== pages.length ||
    new Set(pages.map((page) => page.slug)).size !== pages.length
  )
    throw new Error("pages.json must start with home and name unique pages");
  const catalogues = Object.fromEntries(
    languages.map((language) => [
      language.id,
      json(`locales/${language.id}.json`),
    ]),
  );
  for (const [id, catalogue] of Object.entries(catalogues))
    validateCatalogue(catalogues.en, catalogue, id);
  const release = json("release.json");
  const version = release.tag.replace(/^v/, "");
  if (
    !/^\d+\.\d+\.\d+$/.test(version) ||
    !Number.isFinite(Date.parse(release.publishedAt))
  )
    throw new Error("invalid release version or date");
  const [major, minor] = version.split(".").map(Number);
  // A pre-release announcement (decisions 227 and 230): shown while the published
  // release is older than the announced one, and gone from the build once it is out.
  const announcement = json("announcement.json");
  const opens = new Date(announcement.opens);
  const closes = new Date(announcement.closes);
  if (
    !/^\d+\.\d+\.\d+$/.test(announcement.version) ||
    !Number.isFinite(opens.getTime()) ||
    !Number.isFinite(closes.getTime()) ||
    closes <= opens ||
    (closes - opens) % 86400000 !== 0
  )
    throw new Error("invalid announcement");
  const announcing = major < Number(announcement.version.split(".")[0]);
  for (const [name, count] of [
    ["features", V5_ICONS.features.length],
    ["later", V5_ICONS.later.length],
  ])
    if (catalogues.en.v5[name].length !== count)
      throw new Error(
        `the announcement lists ${catalogues.en.v5[name].length} ${name}; build.mjs has icons for ${count}`,
      );
  const licence =
    major < 3 || (major === 3 && minor <= 4)
      ? "GPL-3.0"
      : "PolyForm Noncommercial 1.0.0";
  const year = String(new Date(release.publishedAt).getUTCFullYear());
  const layout = readFileSync(join(here, "layout.html"), "utf8");
  const templates = Object.fromEntries(
    pages.map((page) => [
      page.id,
      readFileSync(join(here, `pages/${page.id}.html`), "utf8"),
    ]),
  );
  const sourceCSS = readdirSync(join(here, "assets/css"))
    .filter((name) => name.endsWith(".css"))
    .sort()
    .map((name) => readFileSync(join(here, "assets/css", name), "utf8"))
    .join("\n");
  const bootstrap = readFileSync(join(here, "assets/js/language.js"), "utf8");
  const icons = loadIcons();
  const usedAssets = new Set(["assets/js/site.js", "assets/img/og-image.jpg"]);
  const rendered = [];
  const render = (language, page, entry = false) => {
    const catalogue = catalogues[language.id];
    const folder = entry
      ? ""
      : `${language.id}${page.slug ? `/${page.slug}` : ""}`;
    const depth = folder.split("/").filter(Boolean).length;
    const base = "../".repeat(depth);
    const lookup = (key) =>
      key.split(".").reduce((object, part) => object?.[part], catalogue);
    const text = (key) => {
      const value = lookup(key);
      if (typeof value !== "string")
        throw new Error(`${language.id}: unknown string ${key}`);
      return value;
    };
    const t = (key) => escapeHTML(text(key));
    const linkLanguage = entry ? "en" : language.id;
    const folderOf = (id, lang = linkLanguage) => {
      const target = pages.find((item) => item.id === id);
      if (!target) throw new Error(`unknown page ${id}`);
      return `${lang}${target.slug ? `/${target.slug}` : ""}`;
    };
    const href = (spec) => {
      const [id, fragment] = spec.split("#");
      return relativeHref(folder, folderOf(id), fragment);
    };
    const icon = (name, extra = "") =>
      `<svg class="icon${extra ? ` ${extra}` : ""}" aria-hidden="true"><use href="#i-${name}"/></svg>`;
    const logo = (name, extra = "") =>
      `<svg class="logo${extra ? ` ${extra}` : ""}" aria-hidden="true"><use href="#i-${name}"/></svg>`;
    const copyField = (id, label, value, context) =>
      `<div class="copy-field"><span>${label}</span><div class="copy-row"><code id="${id}" dir="ltr">${escapeHTML(value)}</code><button class="copy-button" type="button" data-copy="${id}" aria-label="${t("download.copy")} — ${label} — ${context}">${icon("copy")}<span>${t("download.copy")}</span></button></div></div>`;

    const assetNames = {
      windows: [
        [
          "theia-setup-windows-amd64.exe",
          "Windows x64",
          "x64",
          "verifiedWindows",
        ],
        [
          "theia-setup-windows-arm64.exe",
          "Windows ARM64",
          "arm",
          "verifiedWindows",
        ],
      ],
      macos: [
        ["theia-setup-darwin-arm64", "Mac", "apple", "verifiedMac"],
        ["theia-setup-darwin-amd64", "Mac Intel", "intel", "verifiedIntel"],
      ],
      linux: [
        ["theia-setup-linux-amd64", "Ubuntu 24.04 x64", "x64", "verifiedLinux"],
        [
          "theia-setup-linux-arm64",
          "Ubuntu 24.04 ARM64",
          "arm",
          "verifiedLinux",
        ],
      ],
    };
    const osMarks = { windows: "windows", macos: "apple", linux: "linux" };
    const blocks = {
      download_panels: () =>
        Object.entries(assetNames)
          .map(([os, assets]) => {
            const title = { windows: "Windows", macos: "macOS", linux: "Linux" }[
              os
            ];
            const prerequisite =
              os === "linux"
                ? `<details class="quiet-details prerequisites" open><summary>${t("download.prerequisites")}</summary>${copyField("ubuntu-prerequisites", t("download.prerequisites"), "sudo apt install libwebkit2gtk-4.1-0 libayatana-appindicator3-1", "Ubuntu")}</details>`
                : "";
            const rows = assets
              .map(([filename, name, processor, verified]) => {
                const asset = release.assets[filename];
                if (
                  !asset ||
                  !Number.isInteger(asset.size) ||
                  asset.size <= 0 ||
                  !/^[a-f0-9]{64}$/.test(asset.sha256)
                )
                  throw new Error(`missing release facts for ${filename}`);
                const tag = asset.tag || release.tag;
                const url =
                  asset.url ||
                  `https://github.com/Benitoow/theia-media/releases/download/${tag}/${filename}`;
                if (
                  !url.startsWith(
                    `https://github.com/Benitoow/theia-media/releases/download/${tag}/`,
                  ) ||
                  new URL(url).pathname.split("/").pop() !== filename
                )
                  throw new Error(`unexpected download URL for ${filename}`);
                const legacy = tag !== release.tag;
                const size = new Intl.NumberFormat(language.intl, {
                  style: "unit",
                  unit: "megabyte",
                  maximumFractionDigits: 1,
                }).format(asset.size / 1000000);
                const id = filename.replaceAll(".", "-");
                const command =
                  os === "windows"
                    ? `Get-FileHash -LiteralPath ".\\${filename}" -Algorithm SHA256`
                    : `${os === "macos" ? "shasum -a 256" : "sha256sum"} -- "${filename}"`;
                return `<li class="arch${legacy ? " arch-legacy" : ""}"><a class="arch-link" href="${escapeHTML(url)}"><span class="arch-version"><bdi>${escapeHTML(tag)}</bdi><span>${t(legacy ? "download.legacy" : "download.current")}</span></span><span class="arch-name"><bdi>${name}</bdi><span class="arch-sub">${t(`download.${processor}`)}</span></span><span class="arch-size">${escapeHTML(size)}</span>${icon("download", "icon-arrow")}</a><p class="arch-state">${t(legacy ? "download.legacyNote" : `download.${verified}`)}</p><details class="quiet-details verify"><summary>${t("download.verify")}</summary><div class="verify-content">${copyField(`file-${id}`, t("download.filename"), filename, name)}${copyField(`sha-${id}`, "SHA-256", asset.sha256, name)}${copyField(`command-${id}`, t("download.command"), command, name)}</div></details></li>`;
              })
              .join("\n");
            return `<section class="panel" id="panel-${os}" aria-labelledby="panel-${os}-title"><h2 class="panel-title" id="panel-${os}-title">${logo(osMarks[os])}${title}</h2><p class="panel-note">${t(`download.${os}Warning`)}</p>${prerequisite}<p class="step-label"><span aria-hidden="true">02</span>${t("download.processor")}</p><ul class="arches">${rows}</ul></section>`;
          })
          .join("\n"),

      moments: () => {
        const images = [
          ["shot-library.webp", 1200, 900],
          ["shot-player.webp", 1600, 943],
          ["shot-friends.webp", 1200, 707],
        ];
        return catalogue.discover.benefits
          .map(
            (item, index) =>
              `<article class="moment"><div class="moment-copy"><span class="tile" aria-hidden="true">${icon(MOMENT_ICONS[index])}</span><h3>${escapeHTML(item.title)}</h3><p>${escapeHTML(item.body)}</p></div><figure class="moment-shot"><img src="${base}assets/img/${images[index][0]}" width="${images[index][1]}" height="${images[index][2]}" alt="${escapeHTML(item.alt)}" loading="lazy" decoding="async"><figcaption>${escapeHTML(item.label)}</figcaption></figure></article>`,
          )
          .join("\n");
      },

      scenes: () => {
        const times = [
          [0, "0:00"],
          [31, "0:31"],
          [68, "1:08"],
        ];
        return catalogue.tour.steps
          .map(
            (item, index) =>
              `<li class="scene"><a class="scene-time" href="#demo" data-seek="${times[index][0]}" aria-label="${t("tour.from")} ${times[index][1]}"><bdi>${times[index][1]}</bdi></a><span class="scene-title">${escapeHTML(item.title)}</span><p>${escapeHTML(item.body)}</p></li>`,
          )
          .join("\n");
      },

      caption_tracks: () =>
        languages
          .map(
            (item) =>
              `<track kind="captions" src="${base}assets/video/theia-install-tour.${item.id}.vtt" srclang="${item.lang}" label="${escapeHTML(item.name)}"${item.id === language.id ? " default" : ""}>`,
          )
          .join("\n"),

      faq: () =>
        catalogue.faq.items
          .map(
            (item, index) =>
              `<details class="faq-item" id="${FAQ_IDS[index]}"><summary>${escapeHTML(item.question)}<span class="faq-sign" aria-hidden="true"></span></summary><div class="faq-answer">${item.answer
                .split("\n\n")
                .map((paragraph) => `<p>${escapeHTML(paragraph)}</p>`)
                .join("")}</div></details>`,
          )
          .join("\n"),

      privacy_full: () =>
        catalogue.faq.items[FAQ_IDS.indexOf("privacy")].answer
          .split("\n\n")
          .map((paragraph) => `<p>${escapeHTML(paragraph)}</p>`)
          .join(""),

      services: () =>
        `<ul class="services">${[
          ["film", "TMDB", "privacy.tmdb", false],
          ["github", "GitHub", "privacy.github", true],
          ["firebase", "Firebase", "privacy.firebase", true],
        ]
          .map(
            ([name, label, key, brand]) =>
              `<li class="service"><span class="tile" aria-hidden="true">${brand ? logo(name) : icon(name)}</span><div><h3><bdi lang="en">${label}</bdi></h3><p>${t(key)}</p></div></li>`,
          )
          .join("")}</ul>`,

      built: () =>
        `<section id="built" class="section built" aria-labelledby="built-title"><div class="wrap"><header class="section-head"><h2 id="built-title">${t("built.title")}</h2></header><ul class="logo-row" dir="ltr">${BUILT_WITH.map(
          ([name, label]) =>
            `<li>${logo(name)}<span>${label}</span></li>`,
        ).join("")}</ul><p class="built-note">${t("built.lead")}</p></div></section>`,

      news: () =>
        `<section id="new" class="section news-section" aria-labelledby="new-title"><div class="wrap"><header class="section-head news-head"><div><h2 id="new-title">${t("news.title")}</h2><p class="release-line"><bdi>${escapeHTML(release.tag)}</bdi><span aria-hidden="true"> · </span><time datetime="${release.publishedAt.slice(0, 10)}">${escapeHTML(new Intl.DateTimeFormat(language.intl, { dateStyle: "long", timeZone: "UTC" }).format(new Date(release.publishedAt)))}</time></p></div><a class="text-link" href="https://github.com/Benitoow/theia-media/blob/main/docs/releases/${release.tag}.md">${t("news.notes")}${icon("arrow", "icon-arrow")}</a></header><div class="news-grid">${catalogue.news.items
          .map(
            (item) =>
              `<article class="news-item"><h3>${escapeHTML(item.title)}</h3><p>${escapeHTML(item.body)}</p></article>`,
          )
          .join("\n")}</div></div></section>`,

      cta_band: () =>
        `<section id="help" class="section cta-band" aria-labelledby="help-title"><div class="wrap"><h2 id="help-title">${t("help.title")}</h2><p class="section-lead">${t("help.body")}</p><div class="cta-row"><a class="btn" href="${href("download")}">${t("hero.download")}${icon("download", "icon-arrow")}</a><a class="btn btn-secondary" href="${DISCORD}">${logo("discord")}${t("help.discord")}</a><a class="text-link" href="${FIELD_REPORT}">${t("help.feedback")}${icon("arrow", "icon-arrow")}</a></div></div></section>`,

      icon_licences: () =>
        [
          ["lucide", "Lucide (ISC)"],
          ["simple-icons", "Simple Icons (CC0)"],
          ["svg-logos", "SVG Logos (CC0)"],
        ]
          .map(([set, label]) => {
            const file = `assets/icons/LICENSE-${set}.txt`;
            return `<li><a href="${base}${file}" lang="en">${label}</a></li>`;
          })
          .concat([
            `<li><a href="${base}assets/icons/DISCLAIMER-simple-icons.txt" lang="en">Simple Icons disclaimer</a></li>`,
          ])
          .join(""),

      navigation: () => navigation(false),
      navigation_mobile: () => navigation(true),

      language_links: () =>
        languages
          .map((item) => {
            const target = relativeHref(
              folder,
              `${item.id}${page.slug ? `/${page.slug}` : ""}`,
            );
            return `<a href="${target}" data-language="${item.id}" lang="${item.lang}" dir="${item.dir}"${!entry && item.id === language.id ? ' aria-current="true"' : ""}>${escapeHTML(item.name)}</a>`;
          })
          .join("\n"),

      footer_pages: () =>
        [
          ["home", "nav.home"],
          ["download", "nav.download"],
          ["new", "nav.new"],
          ["questions", "nav.faq"],
          ["about", "nav.about"],
          ["privacy", "nav.privacy"],
        ]
          .map(
            ([id, key]) =>
              `<li><a href="${href(id)}"${page.id === id ? ' aria-current="page"' : ""}>${t(key)}</a></li>`,
          )
          .join(""),

      footer_project: () =>
        [
          [GITHUB, "GitHub", true],
          [`${GITHUB}/releases`, t("footer.releases"), false],
          [DISCORD, "Discord", true],
          [`${GITHUB}/security/policy`, t("footer.security"), false],
        ]
          .map(
            ([url, label, brand]) =>
              `<li><a href="${url}"${brand ? ' lang="en"' : ""}>${label}</a></li>`,
          )
          .join(""),

      social: () =>
        [
          [GITHUB, "GitHub", "github"],
          [DISCORD, "Discord", "discord"],
          [FEED, t("footer.feed"), "rss"],
        ]
          .map(
            ([url, label, name]) =>
              `<li><a class="social-link" href="${url}" aria-label="${label}">${logo(name)}</a></li>`,
          )
          .join(""),

      v5_pill: () =>
        announcing
          ? `<a class="v5-pill" href="${href("new#v5")}"><span class="v5-dot" aria-hidden="true"></span><span>${t("v5.pill")}</span><span aria-hidden="true">·</span><time datetime="${isoDay(opens)}">${escapeHTML(inZone({ weekday: "long", day: "numeric", month: "long" }).format(opens))}</time>${icon("arrow", "icon-arrow")}</a>`
          : "",

      v5_section: () => (announcing ? v5Section() : ""),
    };

    // The pre-release announcement. Dates come from announcement.json and are
    // formatted by Intl in the announced time zone, so no catalogue carries one.
    const inZone = (options) =>
      new Intl.DateTimeFormat(language.intl, {
        ...options,
        timeZone: announcement.timeZone,
      });
    const isoDay = (date) =>
      new Intl.DateTimeFormat("en-CA", { timeZone: announcement.timeZone }).format(
        date,
      );
    const windowEnd = new Date(closes.getTime() - 60000);
    const v5Section = () => `<section id="v5" class="section v5-section" aria-labelledby="v5-title"><div class="wrap">
<header class="section-head"><h2 id="v5-title">${t("v5.title")}</h2><p class="section-lead">${t("v5.lead")}</p></header>
<div class="v5-countdown" data-v5 data-opens="${opens.toISOString()}" data-closes="${closes.toISOString()}" data-locale="${escapeHTML(language.intl)}">
<div class="v5-when"><p class="v5-label">${t("v5.planned")}</p><p class="v5-range">${escapeHTML(inZone({ dateStyle: "full" }).formatRange(opens, windowEnd).replace(/[   ]+(?=[–—~])/, " "))}</p><p class="v5-promise">${t("v5.promise")}</p></div>
<div class="v5-live">
<div class="v5-clock" role="timer" aria-label="${t("v5.timer")}" hidden>${["days", "hours", "minutes", "seconds"].map((unit) => `<div class="v5-cell"><span class="v5-num" data-unit="${unit}" dir="ltr">--</span><span class="v5-unit">${t(`v5.${unit}`)}</span></div>`).join("")}</div>
<p class="v5-status" data-when="open" hidden>${t("v5.open")}</p>
<p class="v5-status" data-when="after" hidden>${t("v5.after")}</p>
<ol class="v5-days" aria-label="${t("v5.planned")}">${Array.from({ length: (closes - opens) / 86400000 }, (_, index) => {
      const start = new Date(opens.getTime() + index * 86400000);
      const end = new Date(start.getTime() + 86400000);
      return `<li class="v5-day" data-state="next" data-start="${start.toISOString()}" data-end="${end.toISOString()}">${escapeHTML(inZone({ weekday: "short", day: "numeric" }).format(start))}</li>`;
    }).join("")}</ol>
</div></div>
<h3 class="v5-sub">${t("v5.featuresTitle")}</h3>
<ul class="v5-grid">${catalogue.v5.features.map((item, index) => `<li class="v5-item"><span class="tile" aria-hidden="true">${V5_ICONS.features[index] === "googledrive" ? logo("googledrive") : icon(V5_ICONS.features[index])}</span><div><h4>${escapeHTML(item.title)}</h4><p>${escapeHTML(item.body)}</p></div></li>`).join("\n")}</ul>
<div class="v5-later"><h3 class="v5-sub">${t("v5.laterTitle")}</h3><p class="v5-later-intro">${t("v5.laterIntro")}</p><ul class="v5-later-list">${catalogue.v5.later.map((item, index) => `<li class="v5-later-item"><span class="tile" aria-hidden="true">${icon(V5_ICONS.later[index])}</span><div><h4>${escapeHTML(item.title)}</h4><p>${escapeHTML(item.body)}</p></div></li>`).join("\n")}</ul></div>
<p class="v5-foot">${t("v5.foot")}</p>
</div></section>`;

    const navKeys = { new: "new", download: "download", questions: "faq", about: "about" };
    const navigation = (mobile) =>
      Object.entries(navKeys)
        .map(
          ([id, key]) =>
            `<a href="${href(id)}"${page.id === id ? ' aria-current="page"' : ""}>${id === "new" && announcing ? '<span class="nav-dot" aria-hidden="true"></span>' : ""}${t(`nav.${key}`)}</a>`,
        )
        .concat(
          mobile
            ? [`<a href="${GITHUB}">${logo("github")}<span lang="en">GitHub</span></a>`]
            : [`<a class="nav-icon" href="${GITHUB}" aria-label="${t("nav.github")}">${logo("github")}</a>`],
        )
        .join("");

    const canonical = `${siteUrl}/${language.id}${page.slug ? `/${page.slug}` : ""}/`;
    const font =
      {
        "zh-hans": "noto-chinese",
        hi: "noto-devanagari",
        bn: "noto-bengali",
        ar: "noto-arabic",
        ur: "noto-arabic",
      }[language.id] || "cinzel-latin-wght-normal";
    const meta =
      page.id === "home" ? catalogue.meta : catalogue.pages[page.id];
    const alternateFor = (item) =>
      `${siteUrl}/${item.id}${page.slug ? `/${page.slug}` : ""}/`;
    const structured =
      page.id === "home" || page.id === "download"
        ? {
            "@context": "https://schema.org",
            "@type": "SoftwareApplication",
            name: "Theia",
            applicationCategory: "MultimediaApplication",
            operatingSystem: "Windows, macOS, Linux (Ubuntu 24.04)",
            description: meta.description,
            softwareVersion: version,
            inLanguage: language.lang,
            url: canonical,
            downloadUrl: release.url,
            screenshot: `${siteUrl}/assets/video/theia-install-tour-poster.webp`,
            isAccessibleForFree: true,
            offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
            license: [
              "https://polyformproject.org/licenses/noncommercial/1.0.0",
              "https://www.gnu.org/licenses/gpl-3.0.html",
            ],
          }
        : {
            "@context": "https://schema.org",
            "@type": "WebPage",
            name: meta.title,
            description: meta.description,
            inLanguage: language.lang,
            url: canonical,
            isPartOf: { "@type": "WebSite", name: "Theia", url: `${siteUrl}/` },
          };
    const tokens = {
      lang: language.lang,
      dir: language.dir,
      entry: String(entry),
      language_ids: languages.map((item) => item.id).join(","),
      page_id: page.id,
      home_path: entry ? `${language.id}/` : "../".repeat(depth - 1) || "./",
      base,
      language_name: language.name,
      og_locale: language.og,
      site_url: siteUrl,
      canonical,
      meta_title: meta.title,
      meta_description: meta.description,
      tag: release.tag,
      licence,
      year,
      release_url: release.url,
      date_iso: release.publishedAt.slice(0, 10),
      date: new Intl.DateTimeFormat(language.intl, {
        dateStyle: "long",
        timeZone: "UTC",
      }).format(new Date(release.publishedAt)),
      font_preload: [...new Set([font, "jost-latin-wght-normal"])]
        .map(
          (name) =>
            `<link rel="preload" href="${base}assets/fonts/${name}.woff2" as="font" type="font/woff2" crossorigin>`,
        )
        .join("\n"),
      head_extra:
        page.id === "home"
          ? `<link rel="preload" href="${base}assets/video/hero-ascii-poster.webp" as="image" type="image/webp" fetchpriority="high">`
          : "",
      alternates: [
        ...languages.map(
          (item) =>
            `<link rel="alternate" hreflang="${item.lang}" href="${alternateFor(item)}">`,
        ),
        `<link rel="alternate" hreflang="x-default" href="${
          page.id === "home"
            ? `${siteUrl}/`
            : `${siteUrl}/en/${page.slug}/`
        }">`,
      ].join("\n"),
      bootstrap,
      structured_data: JSON.stringify(structured).replaceAll("<", "\\u003c"),
    };
    const rawTokens = new Set([
      "alternates",
      "bootstrap",
      "structured_data",
      "font_preload",
      "head_extra",
    ]);
    const css = sourceCSS
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .replace(/url\((['"]?)\.\.\//g, `url($1${base}assets/`)
      .replace(/\s+/g, " ")
      .replace(/ ?([{};]) ?/g, "$1")
      .trim();
    let html = layout
      .replace("{{page}}", () => templates[page.id])
      .replace(/\{\{\s*([\w.:#-]+)\s*\}\}/g, (_, key) => {
        if (key.startsWith("t:")) return t(key.slice(2));
        if (key.startsWith("href:")) return href(key.slice(5));
        if (key.startsWith("icon:")) {
          const [, name, extra] = key.split(":");
          return icon(name, extra);
        }
        if (key.startsWith("logo:")) {
          const [, name, extra] = key.split(":");
          return logo(name, extra);
        }
        if (key.startsWith("block:")) {
          const name = key.slice(6);
          if (!(name in blocks)) throw new Error(`unknown block: ${name}`);
          return blocks[name]();
        }
        if (key === "css") return css;
        if (key === "sprite") return "<!--sprite-->";
        if (!(key in tokens)) throw new Error(`unknown template token: ${key}`);
        return rawTokens.has(key) ? tokens[key] : escapeHTML(tokens[key]);
      });
    if (/\{\{/.test(html)) throw new Error("unresolved template token");
    const wanted = new Set(SCRIPTED_ICONS.filter(() => page.id === "home"));
    for (const match of html.matchAll(/href="#i-([a-z0-9-]+)"/g))
      wanted.add(match[1]);
    html = html.replace("<!--sprite-->", () => spriteFor(wanted, icons));
    for (const match of html.matchAll(
      /(?:src|href|poster|data-src)="(?:\.\.\/)*(assets\/[^"#]+)"|url\(['"]?(?:\.\.\/)*(assets\/[^)'" ]+)/g,
    ))
      usedAssets.add(match[1] || match[2]);
    rendered.push({
      path: entry ? "index.html" : `${folder}/index.html`,
      page: html,
    });
  };
  const english = languages.find((language) => language.id === "en");
  if (!english) throw new Error("the English entry catalogue is required");
  render(english, pages[0], true);
  for (const language of languages)
    for (const page of pages) render(language, page);

  const dist = join(here, "dist");
  rmSync(dist, { recursive: true, force: true });
  mkdirSync(dist, { recursive: true });
  for (const { path, page } of rendered) {
    mkdirSync(dirname(join(dist, path)), { recursive: true });
    writeFileSync(join(dist, path), page);
  }
  for (const asset of usedAssets) {
    if (asset.endsWith(".vtt")) continue;
    if (!existsSync(join(here, asset)))
      throw new Error(`missing local asset: ${asset}`);
    mkdirSync(dirname(join(dist, asset)), { recursive: true });
    cpSync(join(here, asset), join(dist, asset));
  }
  for (const filename of readdirSync(join(here, "assets/fonts")).filter(
    (name) => /^LICENSE.*\.txt$/.test(name),
  ))
    cpSync(
      join(here, "assets/fonts", filename),
      join(dist, "assets/fonts", filename),
    );
  const cueTimes = [
    ["00:00:00.000", "00:00:30.900"],
    ["00:00:31.000", "00:00:35.900"],
    ["00:00:36.000", "00:00:41.900"],
    ["00:00:42.000", "00:00:49.700"],
    ["00:00:49.800", "00:00:58.000"],
    ["00:00:58.100", "00:01:02.900"],
    ["00:01:03.000", "00:01:07.900"],
    ["00:01:08.000", "00:01:09.900"],
    ["00:01:10.000", "00:01:14.900"],
    ["00:01:15.000", "00:01:17.900"],
    ["00:01:18.000", "00:01:19.900"],
    ["00:01:20.000", "00:01:25.900"],
    ["00:01:26.000", "00:01:30.000"],
  ];
  for (const language of languages)
    writeFileSync(
      join(dist, `assets/video/theia-install-tour.${language.id}.vtt`),
      `WEBVTT\n\n${catalogues[language.id].captions.map((caption, index) => `${index + 1}\n${cueTimes[index].join(" --> ")}\n${caption}\n`).join("\n")}`,
    );
  const today = new Date().toISOString().slice(0, 10);
  const urlEntry = (language, page) => {
    const suffix = page.slug ? `${page.slug}/` : "";
    const fallback =
      page.id === "home" ? `${siteUrl}/` : `${siteUrl}/en/${suffix}`;
    return `  <url><loc>${escapeHTML(`${siteUrl}/${language.id}/${suffix}`)}</loc><lastmod>${today}</lastmod>${languages.map((item) => `<xhtml:link rel="alternate" hreflang="${item.lang}" href="${escapeHTML(`${siteUrl}/${item.id}/${suffix}`)}"/>`).join("")}<xhtml:link rel="alternate" hreflang="x-default" href="${escapeHTML(fallback)}"/></url>`;
  };
  writeFileSync(
    join(dist, "sitemap.xml"),
    readFileSync(join(here, "sitemap.xml"), "utf8").replace(
      "{{localized_urls}}",
      pages
        .flatMap((page) => languages.map((language) => urlEntry(language, page)))
        .join("\n"),
    ),
  );
  writeFileSync(
    join(dist, "robots.txt"),
    readFileSync(join(here, "robots.txt"), "utf8").replaceAll(
      "{{site_url}}",
      siteUrl,
    ),
  );
  console.log(
    `build: ${release.tag}; ${pages.length} pages x ${languages.length} languages and the English root; ${siteUrl}/ -> ${relative(process.cwd(), dist)}`,
  );
  if (argv.includes("--check")) {
    const result = spawnSync(
      process.execPath,
      [join(here, "check.mjs"), "--site-url", siteUrl],
      { stdio: "inherit" },
    );
    if (result.status !== 0) throw new Error("site checks failed");
  }
}

const FAQ_IDS = [
  "faq-files",
  "faq-price",
  "limits",
  "faq-processor",
  "faq-install",
  "privacy",
  "faq-friends",
  "faq-help",
];

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  try {
    build();
  } catch (error) {
    console.error(`build: ${error.message}`);
    process.exitCode = 1;
  }
}
