import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { FONTS, THEMES } from './admin/styles';

// Every theme's text colours against its tiles, as WCAG computes contrast.
// A translucent tile is laid over --measure-backdrop when the theme names
// one (the spot of its backdrop that is worst for the text), else over
// --surface-0.

const css = ['./themes.css', './presets.css', './presets-screen.css', './presets-film.css', './presets-art.css'].map((f) => readFileSync(new URL(f, import.meta.url), 'utf8')).join('\n');

/** The custom properties of each theme, by name. */
function themes(): Map<string, Record<string, string>> {
  const result = new Map<string, Record<string, string>>();
  for (const [, selectors = '', body = ''] of css.matchAll(/([^{}]*\[data-theme='[^']+'\][^{}]*)\{([^}]*)\}/g)) {
    if (selectors.includes('data-font')) continue;
    const props: Record<string, string> = {};
    for (const [, name = '', value = ''] of body.matchAll(/--([\w-]+):\s*([^;]+);/g)) props[name] = value.trim();
    for (const [, name = ''] of selectors.matchAll(/\[data-theme='([^']+)'\]/g)) result.set(name, props);
  }
  return result;
}

type Rgba = [number, number, number, number];

function parse(color: string | undefined): Rgba {
  if (color === undefined) throw new Error('a colour is missing');
  const hex = color.match(/^#([0-9a-f]{6})$/i);
  if (hex) {
    const n = parseInt(hex[1] ?? '', 16);
    return [(n >> 16) & 255, (n >> 8) & 255, n & 255, 1];
  }
  const rgba = color.match(/^rgba?\(([^)]+)\)$/);
  if (rgba) {
    const [r = 0, g = 0, b = 0, a = 1] = (rgba[1] ?? '').split(/[\s,/]+/).filter(Boolean).map(Number);
    return [r, g, b, a];
  }
  throw new Error(`not a colour: ${color}`);
}

/** color laid over an opaque base. */
function over([r, g, b, a]: Rgba, [br, bg, bb]: Rgba): Rgba {
  return [r * a + br * (1 - a), g * a + bg * (1 - a), b * a + bb * (1 - a), 1];
}

function luminance([r, g, b]: Rgba): number {
  const lin = (c: number) => ((c /= 255) <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a: Rgba, b: Rgba): number {
  const [la, lb] = [luminance(a), luminance(b)];
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

const all = themes();

describe('themes', () => {
  it('are exactly the ones the server accepts', () => {
    const server = readFileSync(new URL('../../internal/display/themes.go', import.meta.url), 'utf8');
    const block = server.slice(server.indexOf('var Themes'), server.indexOf('var Fonts'));
    const listed = [...block.matchAll(/"([a-z-]+)"/g)].map((m) => m[1] ?? '');
    expect([...all.keys()].sort()).toEqual(listed.sort());
  });

  it('and the fonts are the ones the server accepts, each named in the editor', () => {
    const server = readFileSync(new URL('../../internal/display/themes.go', import.meta.url), 'utf8');
    const fonts = [...server.slice(server.indexOf('var Fonts')).matchAll(/"([a-z-]+)"/g)].map((m) => m[1] ?? '');
    expect(Object.keys(FONTS).sort()).toEqual([...fonts].sort());
    expect(Object.keys(THEMES).sort()).toEqual([...all.keys()].sort());
    for (const font of fonts) expect(css, `no face for ${font}`).toContain(`--face-${font}:`);
  });

  it('each use the typeface the editor names as their own', () => {
    for (const [name, props] of all) {
      const face = props['font-ui']?.match(/--face-([a-z-]+)/)?.[1] ?? 'geist';
      expect(face, name).toBe(THEMES[name]?.font);
    }
  });

  for (const [name, props] of all) {
    it(`${name} keeps every text colour at 4.5:1 on its tiles`, () => {
      const base = parse(props['measure-backdrop'] ?? props['surface-0']);
      const tile = over(parse(props['surface-1']), base);
      for (const role of ['text-primary', 'text-muted', 'accent', 'warning']) {
        const ratio = contrast(parse(props[role]), tile);
        expect(ratio, `${name} ${role} ${props[role]} is ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(4.5);
      }
    });
  }
});
