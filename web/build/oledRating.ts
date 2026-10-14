// The OLED rating of every theme, worked out from its colours when the web
// interface is built, so no one has to write it down or keep it up to date.
//
// An OLED pixel makes its own light: on pure black (#000) it is off, draws
// no power and cannot burn in. A theme's average picture level (APL) is
// estimated from its backdrop and its tiles, the tiles covering about 60 %
// of the screen in a typical layout. Text and thin borders light too few
// pixels to count. Colours are averaged as CSS draws gradients (in sRGB,
// premultiplied), and a level is the luma of that average, 0 for black and
// 1 for white.
//
// The ratings reach the style picker as the module virtual:oled-ratings.
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import type { Plugin } from 'vite';

export type OledRating = 'oled-friendly' | 'ok' | 'bright';

export interface ThemeRating {
  rating: OledRating;
  /** The estimated average picture level, 0 to 1. */
  level: number;
}

/** How much of the screen the tiles cover in a typical layout. */
const TILE_SHARE = 0.6;
/** The most a "true black" backdrop may light, for a few stars. */
const TRUE_BLACK_LEVEL = 0.01;
/** The brightest a near-black tile may be: about #0b0b0b. */
const NEAR_BLACK_TILE_LEVEL = 0.045;
/** Above this a theme lights most of the screen. */
const BRIGHT_LEVEL = 0.25;
/** The stage and a typical tile on it, in pixels, for bands and spots of a fixed size. */
const SCREEN: [number, number] = [1920, 1080];
const TILE: [number, number] = [600, 360];
/** Points sampled along a gradient to average it. */
const SAMPLES = 512;

/** An sRGB colour with alpha, channels 0 to 255 and alpha 0 to 1. */
export type Rgba = [number, number, number, number];

const NAMED: Record<string, Rgba> = {
  transparent: [0, 0, 0, 0],
  black: [0, 0, 0, 1],
  white: [255, 255, 255, 1],
};

/** A CSS colour as hex, rgb(), rgba(), hsl(), hsla() or a few names; throws for anything else. */
export function parseColor(color: string | undefined): Rgba {
  if (color === undefined) throw new Error('a colour is missing');
  const text = color.trim().toLowerCase();
  const named = NAMED[text];
  if (named) return named;
  const hex = text.match(/^#([0-9a-f]{3,8})$/);
  if (hex && [3, 6, 8].includes(hex[1]!.length)) {
    const digits = hex[1]!.length === 3 ? [...hex[1]!].map((d) => d + d).join('') : hex[1]!;
    const n = (i: number) => parseInt(digits.slice(i, i + 2), 16);
    return [n(0), n(2), n(4), digits.length === 8 ? n(6) / 255 : 1];
  }
  const fn = text.match(/^(rgba?|hsla?)\(([^)]+)\)$/);
  if (fn) {
    const parts = fn[2]!.split(/[\s,/]+/).filter(Boolean);
    const alpha = parts[3] === undefined ? 1 : parts[3].endsWith('%') ? parseFloat(parts[3]) / 100 : Number(parts[3]);
    if (fn[1]!.startsWith('rgb')) {
      const [r = 0, g = 0, b = 0] = parts.slice(0, 3).map((p) => (p.endsWith('%') ? (parseFloat(p) * 255) / 100 : Number(p)));
      return [r, g, b, alpha];
    }
    return [...hslToRgb(parseFloat(parts[0] ?? '0'), parseFloat(parts[1] ?? '0') / 100, parseFloat(parts[2] ?? '0') / 100), alpha];
  }
  throw new Error(`not a colour this rating can read: ${color}`);
}

function hslToRgb(h: number, s: number, l: number): [number, number, number] {
  const k = (n: number) => (n + h / 30) % 12;
  const a = s * Math.min(l, 1 - l);
  const f = (n: number) => 255 * (l - a * Math.max(-1, Math.min(k(n) - 3, 9 - k(n), 1)));
  return [f(0), f(8), f(4)];
}

/** color laid over base; the result is opaque when base is. */
export function over([r, g, b, a]: Rgba, [br, bg, bb, ba]: Rgba): Rgba {
  const alpha = a + ba * (1 - a);
  if (alpha === 0) return [0, 0, 0, 0];
  const mix = (c: number, bc: number) => (c * a + bc * ba * (1 - a)) / alpha;
  return [mix(r, br), mix(g, bg), mix(b, bb), alpha];
}

/** The picture level of an opaque colour: its luma, 0 to 1. */
export function pictureLevel([r, g, b]: Rgba): number {
  return (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
}

/** Splits a value on a separator that is not inside parentheses. */
function splitTopLevel(value: string, separator: ',' | ' '): string[] {
  const parts: string[] = [];
  let depth = 0;
  let current = '';
  for (const ch of value) {
    if (ch === '(') depth++;
    if (ch === ')') depth--;
    if (depth === 0 && (separator === ' ' ? /\s/.test(ch) : ch === separator)) {
      if (current.trim()) parts.push(current.trim());
      current = '';
      continue;
    }
    current += ch;
  }
  if (current.trim()) parts.push(current.trim());
  return parts;
}

function isColor(token: string): boolean {
  return /^(#|rgba?\(|hsla?\()/i.test(token) || token.toLowerCase() in NAMED;
}

interface Stop {
  color: Rgba;
  /** A fraction of the gradient for a percentage, else a length in px; null when the stop has none. */
  at: number | null;
}

function position(token: string): number {
  return token.endsWith('%') ? parseFloat(token) / 100 : parseFloat(token);
}

/** The stops of a gradient, each colour once per position it names. */
function stops(args: string[]): Stop[] {
  const out: Stop[] = [];
  for (const arg of args) {
    const [color, ...positions] = splitTopLevel(arg, ' ');
    if (!color || !isColor(color)) continue;
    const rgba = parseColor(color);
    if (positions.length === 0) out.push({ color: rgba, at: null });
    for (const p of positions) out.push({ color: rgba, at: position(p) });
  }
  // Stops without a position are spread evenly between their neighbours, as CSS does.
  if (out[0] && out[0].at === null) out[0].at = 0;
  const last = out[out.length - 1];
  if (last && last.at === null) last.at = Math.max(1, ...out.map((s) => s.at ?? 0));
  for (let i = 1; i < out.length; i++) {
    if (out[i]!.at !== null) continue;
    let j = i;
    while (out[j]!.at === null) j++;
    const from = out[i - 1]!.at!;
    const step = (out[j]!.at! - from) / (j - i + 1);
    for (let k = i; k < j; k++) out[k]!.at = from + step * (k - i + 1);
  }
  return out;
}

/** The premultiplied colour of a gradient at t. */
function colorAt(list: Stop[], t: number): Rgba {
  const premultiplied = ([r, g, b, a]: Rgba): Rgba => [r * a, g * a, b * a, a];
  if (t <= list[0]!.at!) return premultiplied(list[0]!.color);
  for (let i = 1; i < list.length; i++) {
    const [a, b] = [list[i - 1]!, list[i]!];
    if (t <= b.at!) {
      const f = b.at! === a.at! ? 1 : (t - a.at!) / (b.at! - a.at!);
      const [pa, pb] = [premultiplied(a.color), premultiplied(b.color)];
      return pa.map((c, k) => c + (pb[k]! - c) * f) as Rgba;
    }
  }
  return premultiplied(list[list.length - 1]!.color);
}

/** Back from premultiplied to a plain colour with alpha. */
function unpremultiply([r, g, b, a]: Rgba): Rgba {
  return a === 0 ? [0, 0, 0, 0] : [r / a, g / a, b / a, a];
}

/**
 * A layer as [whole, name, arguments, rest], cut at the bracket that closes
 * the function, so a position or size that holds brackets of its own, such
 * as "right min(12px, 4%)", stays in the rest.
 */
function splitFunction(layer: string): [string, string, string, string] | null {
  const open = layer.match(/^([a-z-]+)\(/i);
  if (!open) return null;
  let depth = 0;
  for (let i = open[0].length - 1; i < layer.length; i++) {
    if (layer[i] === '(') depth++;
    else if (layer[i] === ')' && --depth === 0) {
      return [layer, open[1]!, layer.slice(open[0].length, i), layer.slice(i + 1).trim()];
    }
  }
  return null;
}

/**
 * The average colour of one background layer, such as
 * "radial-gradient(1px 1px at 9px 9px, #fff 50%, transparent 100%) 0 0 / 97px 89px".
 */
export function layerAverage(layer: string, box: [number, number]): Rgba {
  const fn = splitFunction(layer.trim());
  if (!fn) {
    if (layer.trim() === 'none') return [0, 0, 0, 0];
    if (isColor(layer.trim())) return parseColor(layer);
    throw new Error(`not a background layer this rating can read: ${layer}`);
  }
  const [, name = '', body = '', rest = ''] = fn;
  if (!name.endsWith('gradient')) throw new Error(`only gradients are rated, not ${name}()`);
  const args = splitTopLevel(body, ',');
  const list = stops(args);
  if (list.length === 0) throw new Error(`a gradient without colours: ${layer}`);
  const radial = name.includes('radial');
  const average = gradientAverage(list, name.startsWith('repeating-'), radial);
  const size = (rest.split('/')[1] ?? '').split(/\s+/).filter((t) => /^(\d*\.?\d+(px|%)|0|auto)$/.test(t));
  let coverage = 1;
  let outside: Rgba = [0, 0, 0, 0];
  if (/no-repeat/.test(rest) && size.length > 0) {
    // One band or spot, such as a rule along a tile's edge.
    coverage = share(size[0], box[0]) * share(size[1], box[1]);
  } else if (radial && size.length === 2) {
    // A small dot repeated over the box, as stars are drawn: it covers only
    // its own ellipse of each repeat.
    const config = isColor(splitTopLevel(args[0] ?? '', ' ')[0] ?? '') ? '' : (args[0] ?? '');
    const [rx, ry = rx] = config.split(/\bat\b/)[0]!.match(/(\d*\.?\d+)px/g)?.map(parseFloat) ?? [];
    if (rx !== undefined && ry !== undefined) {
      coverage = Math.min(1, (Math.PI * rx * ry) / (parseFloat(size[0]!) * parseFloat(size[1]!)));
      outside = colorAt(list, Number.POSITIVE_INFINITY);
    }
  }
  return unpremultiply(average.map((v, k) => v * coverage + outside[k]! * (1 - coverage)) as Rgba);
}

/** The premultiplied average colour along a gradient's line, or over its rings. */
function gradientAverage(list: Stop[], repeating: boolean, radial: boolean): Rgba {
  const from = repeating ? list[0]!.at! : 0;
  const to = repeating ? list[list.length - 1]!.at! : Math.max(1, list[list.length - 1]!.at!);
  let sum: Rgba = [0, 0, 0, 0];
  let weights = 0;
  for (let i = 0; i < SAMPLES; i++) {
    const t = from + ((to - from) * (i + 0.5)) / SAMPLES;
    // A radial gradient has more area further out: each ring counts by its radius.
    const weight = radial && !repeating ? (i + 0.5) / SAMPLES : 1;
    const c = colorAt(list, t);
    sum = sum.map((v, k) => v + c[k]! * weight) as Rgba;
    weights += weight;
  }
  return sum.map((v) => v / weights) as Rgba;
}

/** A background size such as "100%" or "2px" as a share of length. */
function share(size: string | undefined, length: number): number {
  if (size === undefined || size === 'auto') return 1;
  return Math.min(1, size.endsWith('%') ? parseFloat(size) / 100 : parseFloat(size) / length);
}

/** The average colour of a background over a box, its layers over base. */
function backgroundAverage(value: string | undefined, base: Rgba, box: [number, number]): Rgba {
  if (!value || value.trim() === 'none') return base;
  // The first layer is drawn on top.
  return splitTopLevel(value, ',').reverse().reduce((under, layer) => over(layerAverage(layer, box), under), base);
}

/** A theme's custom properties, with var() references to its own resolved. */
function resolve(props: Record<string, string>, name: string): string | undefined {
  let value = props[name];
  for (let i = 0; i < 5 && value?.startsWith('var('); i++) value = props[value.match(/^var\(--([\w-]+)/)?.[1] ?? ''];
  return value;
}

/** The rating of one theme, from its custom properties. */
export function rateTheme(props: Record<string, string>): ThemeRating {
  const surface0 = parseColor(resolve(props, 'surface-0'));
  const backdrop = backgroundAverage(resolve(props, 'backdrop'), surface0, SCREEN);
  const tile = backgroundAverage(resolve(props, 'tile-image'), over(parseColor(resolve(props, 'surface-1')), backdrop), TILE);
  const backdropLevel = pictureLevel(backdrop);
  const tileLevel = pictureLevel(tile);
  const level = TILE_SHARE * tileLevel + (1 - TILE_SHARE) * backdropLevel;
  const trueBlack = surface0.slice(0, 3).every((c) => c === 0) && backdropLevel <= TRUE_BLACK_LEVEL;
  if (trueBlack && tileLevel <= NEAR_BLACK_TILE_LEVEL) return { rating: 'oled-friendly', level };
  return { rating: level > BRIGHT_LEVEL ? 'bright' : 'ok', level };
}

/**
 * The custom properties of each theme in css, by name: the rules on
 * .screen[data-theme='x']. The typeface rules ([data-font]) are left out.
 */
export function themeTokens(css: string): Map<string, Record<string, string>> {
  const result = new Map<string, Record<string, string>>();
  const plain = css.replace(/\/\*[\s\S]*?\*\//g, '');
  for (const [, selectors = '', body = ''] of plain.matchAll(/([^{}]*\[data-theme='[^']+'\][^{}]*)\{([^}]*)\}/g)) {
    if (selectors.includes('data-font')) continue;
    const props: Record<string, string> = {};
    for (const [, name = '', value = ''] of body.matchAll(/--([\w-]+):\s*([^;]+);/g)) props[name] = value.trim().replace(/\s+/g, ' ');
    for (const [, name = ''] of selectors.matchAll(/\[data-theme='([^']+)'\]/g)) result.set(name, props);
  }
  return result;
}

/** A stylesheet with the files it @imports, in order, and the paths it read. */
export function readThemeCss(entry: URL): { css: string; files: string[] } {
  const files: string[] = [];
  const read = (url: URL): string => {
    files.push(fileURLToPath(url));
    const css = readFileSync(url, 'utf8');
    return css.replace(/@import\s+['"](\.[^'"]+)['"];/g, (_, path: string) => read(new URL(path, url)));
  };
  return { css: read(entry), files };
}

/** The ratings of every theme in css. */
export function rateThemes(css: string): Record<string, ThemeRating> {
  const all = themeTokens(css);
  // A theme that leaves a token out takes the default theme's, as the cascade does.
  const defaults = all.get('minimal-dark') ?? {};
  return Object.fromEntries([...all].map(([name, props]) => [name, rateTheme({ ...defaults, ...props })]));
}

const MODULE_ID = 'virtual:oled-ratings';
const RESOLVED_ID = '\0' + MODULE_ID;

/** The Vite plugin: serves virtual:oled-ratings, worked out from src/themes.css. */
export function oledRatings(): Plugin {
  const entry = new URL('../src/themes.css', import.meta.url);
  return {
    name: 'gdash-oled-ratings',
    resolveId: (id) => (id === MODULE_ID ? RESOLVED_ID : undefined),
    load(id) {
      if (id !== RESOLVED_ID) return undefined;
      const { css, files } = readThemeCss(entry);
      for (const file of files) this.addWatchFile(file);
      return `export const OLED_RATINGS = ${JSON.stringify(rateThemes(css))};`;
    },
  };
}
