#!/usr/bin/env node
// One template, ten strict catalogues, no runtime service or build dependency.
// node site/build.mjs --check [--site-url https://example.org/theia]
import {
  cpSync,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { spawnSync } from "node:child_process";

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
  const licence =
    major < 3 || (major === 3 && minor <= 4)
      ? "GPL-3.0"
      : "PolyForm Noncommercial 1.0.0";
  const template = readFileSync(join(here, "index.html"), "utf8");
  const sourceCSS = readFileSync(join(here, "assets/css/site.css"), "utf8");
  const bootstrap = readFileSync(join(here, "assets/js/language.js"), "utf8");
  const icon = (name, extra = "") =>
    `<svg class="icon ${extra}" aria-hidden="true"><use href="#i-${name}"/></svg>`;
  const usedAssets = new Set(["assets/js/site.js", "assets/img/og-image.jpg"]);
  const rendered = [];
  const render = (language, entry = false) => {
    const catalogue = catalogues[language.id];
    const base = entry ? "" : "../";
    const t = (key) => {
      const value = key
        .split(".")
        .reduce((object, part) => object?.[part], catalogue);
      if (typeof value !== "string")
        throw new Error(`${language.id}: unknown string ${key}`);
      return escapeHTML(value);
    };
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
    const downloadPanels = Object.entries(assetNames)
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
        return `<section class="panel" id="panel-${os}" aria-labelledby="panel-${os}-title"><h3 class="panel-title" id="panel-${os}-title">${title}</h3><p class="panel-note">${t(`download.${os}Warning`)}</p>${prerequisite}<p class="step-label"><span aria-hidden="true">02</span>${t("download.processor")}</p><ul class="arches">${rows}</ul></section>`;
      })
      .join("\n");
    const images = [
      ["shot-library.webp", 1200, 900],
      ["shot-player.webp", 1600, 943],
      ["moment-remote.webp", 1212, 909],
    ];
    const benefits = catalogue.discover.benefits
      .map(
        (item, index) =>
          `<article class="benefit"><figure><img src="${base}assets/img/${images[index][0]}" width="${images[index][1]}" height="${images[index][2]}" alt="${escapeHTML(item.alt)}" loading="lazy" decoding="async"><figcaption>${escapeHTML(item.label)}</figcaption></figure><h3>${escapeHTML(item.title)}</h3><p>${escapeHTML(item.body)}</p></article>`,
      )
      .join("\n");
    const times = [
      [0, "0:00"],
      [31, "0:31"],
      [68, "1:08"],
    ];
    const steps = catalogue.tour.steps
      .map(
        (item, index) =>
          `<li class="scene"><a class="scene-time" href="#demo" data-seek="${times[index][0]}" aria-label="${t("tour.from")} ${times[index][1]}"><bdi>${times[index][1]}</bdi></a><span class="scene-title">${escapeHTML(item.title)}</span><p>${escapeHTML(item.body)}</p></li>`,
      )
      .join("\n");
    const faqIds = [
      "faq-files",
      "faq-price",
      "limits",
      "faq-processor",
      "faq-install",
      "privacy",
      "faq-friends",
      "faq-help",
    ];
    const faq = catalogue.faq.items
      .map(
        (item, index) =>
          `<details class="faq-item" id="${faqIds[index]}"><summary>${escapeHTML(item.question)}<span class="faq-sign" aria-hidden="true"></span></summary><div class="faq-answer">${item.answer
            .split("\n\n")
            .map((paragraph) => `<p>${escapeHTML(paragraph)}</p>`)
            .join("")}</div></details>`,
      )
      .join("\n");
    const canonical = `${siteUrl}/${language.id}/`;
    const font =
      {
        "zh-hans": "noto-chinese",
        hi: "noto-devanagari",
        bn: "noto-bengali",
        ar: "noto-arabic",
        ur: "noto-arabic",
      }[language.id] || "cinzel-latin-wght-normal";
    const tokens = {
      lang: language.lang,
      dir: language.dir,
      entry: String(entry),
      language_ids: languages.map((item) => item.id).join(","),
      base,
      language_name: language.name,
      og_locale: language.og,
      site_url: siteUrl,
      canonical,
      tag: release.tag,
      licence,
      release_url: release.url,
      release_notes: `https://github.com/Benitoow/theia-media/blob/main/docs/releases/${release.tag}.md`,
      date_iso: release.publishedAt.slice(0, 10),
      date: new Intl.DateTimeFormat(language.intl, {
        dateStyle: "long",
        timeZone: "UTC",
      }).format(new Date(release.publishedAt)),
      font_preload: [...new Set([font, 'jost-latin-wght-normal'])].map(name => `<link rel="preload" href="${base}assets/fonts/${name}.woff2" as="font" type="font/woff2" crossorigin>`).join('\n'),
      alternates: [
        ...languages.map(
          (item) =>
            `<link rel="alternate" hreflang="${item.lang}" href="${siteUrl}/${item.id}/">`,
        ),
        `<link rel="alternate" hreflang="x-default" href="${siteUrl}/">`,
      ].join("\n"),
      navigation: [
        ["discover", "#discover"],
        ["download", "#download"],
        ["faq", "#faq"],
        ["github", "https://github.com/Benitoow/theia-media"],
      ]
        .map(([key, href]) => `<a href="${href}">${t(`nav.${key}`)}</a>`)
        .join(""),
      language_links: languages
        .map(
          (item) =>
            `<a href="${base}${item.id}/" data-language="${item.id}" lang="${item.lang}" dir="${item.dir}"${item.id === language.id ? ' aria-current="true"' : ""}>${escapeHTML(item.name)}</a>`,
        )
        .join("\n"),
      bootstrap,
      benefits,
      steps,
      faq,
      download_panels: downloadPanels,
      news: catalogue.news.items
        .map(
          (item, index) =>
            `<article class="news-item"><span aria-hidden="true">0${index + 1}</span><h3>${escapeHTML(item.title)}</h3><p>${escapeHTML(item.body)}</p></article>`,
        )
        .join("\n"),
      caption_tracks: languages
        .map(
          (item) =>
            `<track kind="captions" src="${base}assets/video/theia-install-tour.${item.id}.vtt" srclang="${item.lang}" label="${escapeHTML(item.name)}"${item.id === language.id ? " default" : ""}>`,
        )
        .join("\n"),
      structured_data: JSON.stringify({
        "@context": "https://schema.org",
        "@type": "SoftwareApplication",
        name: "Theia",
        applicationCategory: "MultimediaApplication",
        operatingSystem: "Windows, macOS, Linux (Ubuntu 24.04)",
        description: catalogue.meta.description,
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
      }).replaceAll("<", "\\u003c"),
    };
    const rawTokens = new Set([
      "alternates",
      "navigation",
      "language_links",
      "bootstrap",
      "benefits",
      "steps",
      "faq",
      "download_panels",
      "news",
      "caption_tracks",
      "structured_data",
      "font_preload",
    ]);
    const css = sourceCSS
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .replace(/url\((['"]?)\.\.\//g, `url($1${base}assets/`)
      .replace(/\s+/g, " ")
      .replace(/ ?([{};]) ?/g, "$1")
      .trim();
    let page = template
      .replace(
        '<link rel="stylesheet" href="assets/css/site.css">',
        () => `<style>${css}</style>`,
      )
      .replace(/\{\{\s*([\w.:-]+)\s*\}\}/g, (_, key) => {
        if (key.startsWith("t:")) return t(key.slice(2));
        if (!(key in tokens)) throw new Error(`unknown template token: ${key}`);
        return rawTokens.has(key) ? tokens[key] : escapeHTML(tokens[key]);
      });
    if (/\{\{/.test(page)) throw new Error("unresolved template token");
    for (const match of page.matchAll(
      /(?:src|href|poster|data-src)="(?:\.\.\/)?(assets\/[^"#]+)"|url\(['"]?(?:\.\.\/)?(assets\/[^)'" ]+)/g,
    ))
      usedAssets.add(match[1] || match[2]);
    rendered.push({
      path: entry ? "index.html" : `${language.id}/index.html`,
      page,
    });
  };
  const english = languages.find(language => language.id === 'en');
  if (!english) throw new Error('the English entry catalogue is required');
  render(english, true);
  languages.forEach((language) => render(language));

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
  writeFileSync(
    join(dist, "sitemap.xml"),
    readFileSync(join(here, "sitemap.xml"), "utf8").replace(
      "{{localized_urls}}",
      languages
        .map(
          (language) =>
            `  <url><loc>${escapeHTML(`${siteUrl}/${language.id}/`)}</loc><lastmod>${today}</lastmod>${languages.map((item) => `<xhtml:link rel="alternate" hreflang="${item.lang}" href="${escapeHTML(`${siteUrl}/${item.id}/`)}"/>`).join("")}<xhtml:link rel="alternate" hreflang="x-default" href="${siteUrl}/"/></url>`,
        )
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
  writeFileSync(join(dist, ".nojekyll"), "");
  console.log(
    `build: ${release.tag}; ${languages.length} languages and English root; ${siteUrl}/ -> ${relative(process.cwd(), dist)}`,
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
