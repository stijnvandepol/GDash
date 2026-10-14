import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { over, parseColor as parse, rateThemes, readThemeCss, themeTokens, type Rgba } from '../build/oledRating';
import { FONTS, THEMES } from './admin/styles';

// Every theme's text colours against its tiles, as WCAG computes contrast.
// A translucent tile is laid over --measure-backdrop when the theme names
// one (the spot of its backdrop that is worst for the text), else over
// --surface-0. The stylesheets are themes.css and every file it imports.

const { css } = readThemeCss(new URL('./themes.css', import.meta.url));

function luminance([r, g, b]: Rgba): number {
  const lin = (c: number) => ((c /= 255) <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a: Rgba, b: Rgba): number {
  const [la, lb] = [luminance(a), luminance(b)];
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

const all = themeTokens(css);

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

describe('OLED ratings', () => {
  const ratings = rateThemes(css);
  const oled = Object.keys(THEMES).filter((id) => THEMES[id]?.group === 'oled');

  it('rate every theme', () => {
    expect(Object.keys(ratings).sort()).toEqual([...all.keys()].sort());
  });

  it('call a true black backdrop with unlit tiles OLED-friendly', () => {
    expect(ratings['night']?.rating).toBe('oled-friendly');
    expect(ratings['high-contrast']?.rating).toBe('oled-friendly');
  });

  it('call dark themes that light their backdrop OK, and light themes bright', () => {
    for (const id of ['minimal-dark', 'glass', 'walnut', 'terminal']) expect(ratings[id]?.rating, id).toBe('ok');
    for (const id of ['minimal-light', 'warm-light', 'frost', 'swiss', 'candy']) expect(ratings[id]?.rating, id).toBe('bright');
  });

  it('call every OLED and night preset OLED-friendly, Starlight with its stars too', () => {
    expect(oled).toHaveLength(10);
    for (const id of oled) expect(ratings[id]?.rating, `${id} at ${ratings[id]?.level}`).toBe('oled-friendly');
  });

  it('give the OLED and night presets a pure black backdrop and tiles without a fill', () => {
    for (const id of oled) {
      const props = all.get(id)!;
      expect(props['surface-0'], id).toBe('#000000');
      expect(luminance(over(parse(props['surface-1']), parse('#000000'))), `${id} fills its tiles`).toBe(0);
    }
  });
});
