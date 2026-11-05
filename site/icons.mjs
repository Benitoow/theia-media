// The site's icons. Every icon is a vendored SVG file under assets/icons, pinned
// to an upstream commit and digest in assets/icons/sources.json. The build
// inlines the ones a page uses as <symbol>s, so no page requests an image, a
// font or a script for them (decision 164: no remote resource).
//
// An SVG from the web is untrusted data. Nothing is copied through: each file is
// parsed against a short whitelist of shapes and attributes, and the markup that
// reaches a page is rebuilt from the values that passed. Anything else - script,
// style, image, use, filter, event handler, link, transform - refuses the file.
import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
export const iconsDirectory = join(here, "assets/icons");

const SHAPES = new Set([
  "path",
  "circle",
  "rect",
  "line",
  "polyline",
  "polygon",
  "ellipse",
]);
const GEOMETRY = new Set([
  "d",
  "cx",
  "cy",
  "r",
  "rx",
  "ry",
  "x",
  "y",
  "width",
  "height",
  "x1",
  "y1",
  "x2",
  "y2",
  "points",
  "fill-rule",
  "clip-rule",
]);
// Presentation the page decides (colour, stroke), so the file's own is dropped.
const DROPPED = new Set([
  "fill",
  "stroke",
  "stroke-width",
  "stroke-linecap",
  "stroke-linejoin",
  "stroke-miterlimit",
  "stroke-opacity",
  "fill-opacity",
  "opacity",
  "class",
  "id",
]);
const ROOT_ALLOWED = new Set([
  "xmlns",
  "xmlns:xlink",
  "xml:space",
  "version",
  "width",
  "height",
  "viewbox",
  "preserveaspectratio",
  "fill",
  "stroke",
  "stroke-width",
  "stroke-linecap",
  "stroke-linejoin",
  "class",
  "role",
  "focusable",
  "aria-hidden",
  "aria-labelledby",
]);
const NUMBER = /^-?(?:\d+\.?\d*|\.\d+)(?:e[-+]?\d+)?$/i;
const PATH_DATA = /^[MmLlHhVvCcSsQqTtAaZz0-9eE+\-., \t\r\n]*$/;
const POINTS = /^[0-9eE+\-., \t\r\n]*$/;

const TAG = /<(\/?)([a-zA-Z][\w:-]*)((?:\s+[\w:.-]+\s*=\s*(?:"[^"]*"|'[^']*'))*)\s*(\/?)>/gy;
const ATTRIBUTE = /([\w:.-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')/g;

function attributesOf(source) {
  const found = {};
  for (const match of source.matchAll(ATTRIBUTE))
    found[match[1].toLowerCase()] = match[2] ?? match[3] ?? "";
  return found;
}

/**
 * Parses one SVG file and returns { viewBox, shapes, markup }, or throws.
 * `markup` is rebuilt from vetted attributes only.
 */
export function vetIcon(text, name = "icon") {
  const refuse = (why) => {
    throw new Error(`${name}: ${why}`);
  };
  let source = text.replace(/^﻿/, "");
  if (/<!(?!--)|<!\[CDATA\[|<\?(?!xml\b)/i.test(source))
    refuse("doctype, entity, CDATA or processing instruction");
  source = source.replace(/<\?xml[\s\S]*?\?>/i, "").replace(/<!--[\s\S]*?-->/g, "");
  if (/javascript:|&#|&(?!amp;|lt;|gt;|quot;|apos;)/i.test(source))
    refuse("script or entity reference");

  let index = 0;
  let root;
  const shapes = [];
  const stack = [];
  const skipText = (end) => {
    const gap = source.slice(index, end);
    if (gap.trim() && !(stack.at(-1) === "title")) refuse("unexpected text");
  };
  while (index < source.length) {
    const next = source.indexOf("<", index);
    if (next === -1) {
      skipText(source.length);
      break;
    }
    skipText(next);
    TAG.lastIndex = next;
    const match = TAG.exec(source);
    if (!match) refuse("malformed tag");
    index = TAG.lastIndex;
    const [, closing, rawName, rawAttributes, selfClosing] = match;
    const tag = rawName.toLowerCase();
    if (closing) {
      if (stack.pop() !== tag) refuse(`mismatched </${tag}>`);
      continue;
    }
    const attributes = attributesOf(rawAttributes);
    for (const key of Object.keys(attributes))
      if (/^on/i.test(key) || /href/i.test(key))
        refuse(`forbidden attribute ${key}`);
    if (!root) {
      if (tag !== "svg") refuse("the root is not <svg>");
      for (const key of Object.keys(attributes))
        if (!ROOT_ALLOWED.has(key)) refuse(`root attribute ${key} is not allowed`);
      let box = attributes.viewbox;
      if (!box) {
        // Some sources give only a size ("16", "256px"): read it as the box.
        const width = parseFloat(attributes.width);
        const height = parseFloat(attributes.height ?? attributes.width);
        if (Number.isFinite(width) && Number.isFinite(height))
          box = `0 0 ${width} ${height}`;
      }
      const parts = (box ?? "").trim().split(/[\s,]+/);
      if (parts.length !== 4 || !parts.every((part) => NUMBER.test(part)))
        refuse("no usable viewBox");
      root = { viewBox: parts.join(" ") };
    } else if (tag === "g" || tag === "title") {
      for (const key of Object.keys(attributes))
        if (!DROPPED.has(key)) refuse(`<${tag}> attribute ${key} is not allowed`);
    } else if (SHAPES.has(tag)) {
      const kept = {};
      for (const [key, value] of Object.entries(attributes)) {
        if (DROPPED.has(key)) continue;
        if (!GEOMETRY.has(key)) refuse(`<${tag}> attribute ${key} is not allowed`);
        if (key === "d" && !PATH_DATA.test(value)) refuse("unexpected path data");
        else if (key === "points" && !POINTS.test(value)) refuse("unexpected points");
        else if (key === "fill-rule" || key === "clip-rule") {
          if (!["evenodd", "nonzero"].includes(value)) refuse(`bad ${key}`);
        } else if (!["d", "points"].includes(key) && !NUMBER.test(value))
          refuse(`<${tag}> ${key} is not a plain number`);
        kept[key] = value.replace(/\s+/g, " ").trim();
      }
      if (tag === "path" && !kept.d) refuse("a path without data");
      shapes.push({ tag, attributes: kept });
    } else {
      refuse(`<${tag}> is not allowed`);
    }
    if (!selfClosing) stack.push(tag);
  }
  if (!root) refuse("no <svg> element");
  if (stack.length) refuse("unclosed element");
  if (!shapes.length) refuse("no shapes");
  const markup = shapes
    .map(
      ({ tag, attributes }) =>
        `<${tag} ${Object.entries(attributes)
          .map(([key, value]) => `${key}="${value}"`)
          .join(" ")}/>`,
    )
    .join("");
  return { viewBox: root.viewBox, shapes, markup };
}

export const sha256 = (bytes) =>
  createHash("sha256").update(bytes).digest("hex");

export function readSources() {
  return JSON.parse(readFileSync(join(iconsDirectory, "sources.json"), "utf8"));
}

/**
 * Loads every icon named in sources.json. With `verify` (the default) a file
 * whose digest differs from the recorded one refuses the whole build.
 * Returns a Map: id -> { id, set, file, kind, viewBox, markup }.
 */
export function loadIcons({ verify = true } = {}) {
  const sources = readSources();
  const registry = new Map();
  for (const entry of sources.icons) {
    const set = sources.sets[entry.set];
    if (!set) throw new Error(`icon ${entry.id}: unknown set ${entry.set}`);
    if (registry.has(entry.id)) throw new Error(`duplicate icon id ${entry.id}`);
    if (!/^[a-z][a-z0-9-]*$/.test(entry.id))
      throw new Error(`icon id ${entry.id} is not a plain name`);
    const path = join(iconsDirectory, entry.set, `${entry.file}.svg`);
    if (!existsSync(path))
      throw new Error(`icon ${entry.id}: ${entry.set}/${entry.file}.svg is missing; run scripts/site-icons.mjs`);
    const bytes = readFileSync(path);
    if (verify && entry.sha256 !== sha256(bytes))
      throw new Error(`icon ${entry.id}: ${entry.set}/${entry.file}.svg differs from its recorded digest`);
    const vetted = vetIcon(bytes.toString("utf8"), `${entry.set}/${entry.file}`);
    registry.set(entry.id, {
      id: entry.id,
      set: entry.set,
      file: entry.file,
      kind: set.kind,
      viewBox: vetted.viewBox,
      markup: vetted.markup,
    });
  }
  return registry;
}

/** One <symbol> per icon. Logos fill with the text colour; interface icons stroke. */
export function symbolFor(icon) {
  const paint = icon.kind === "logo" ? ' fill="currentColor" stroke="none"' : "";
  return `<symbol id="i-${icon.id}" viewBox="${icon.viewBox}"${paint}>${icon.markup}</symbol>`;
}

/** The inline sprite of exactly the icons a page uses. */
export function spriteFor(ids, registry) {
  const symbols = [...new Set(ids)]
    .sort()
    .map((id) => {
      const icon = registry.get(id);
      if (!icon) throw new Error(`unknown icon: ${id}`);
      return symbolFor(icon);
    })
    .join("");
  return `<svg class="sprite" aria-hidden="true" focusable="false">${symbols}</svg>`;
}
