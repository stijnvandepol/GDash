import { describe, expect, it } from 'vitest';
import { layerAverage, parseColor, pictureLevel, rateTheme, themeTokens } from './oledRating';

const BLACK = { 'surface-0': '#000000', 'surface-1': '#000000', backdrop: 'none' };

describe('parseColor', () => {
  it('reads hex, rgb(), hsl() and the names themes use', () => {
    expect(parseColor('#fff')).toEqual([255, 255, 255, 1]);
    expect(parseColor('#00000080')[3]).toBeCloseTo(0.5, 2);
    expect(parseColor('rgba(10, 20, 30, 0.4)')).toEqual([10, 20, 30, 0.4]);
    expect(parseColor('hsl(0, 100%, 50%)').map(Math.round)).toEqual([255, 0, 0, 1]);
    expect(parseColor('transparent')).toEqual([0, 0, 0, 0]);
  });

  it('refuses what it cannot read, so a theme is never rated by a guess', () => {
    expect(() => parseColor('color-mix(in srgb, red, blue)')).toThrow(/not a colour/);
    expect(() => layerAverage('url(stars.png)', [100, 100])).toThrow();
  });
});

describe('layerAverage', () => {
  it('averages a linear gradient along its line', () => {
    const [r, g, b, a] = layerAverage('linear-gradient(90deg, #000000, #ffffff)', [100, 100]);
    expect([r, g, b].map(Math.round)).toEqual([128, 128, 128]);
    expect(a).toBe(1);
  });

  it('counts a repeating stripe by its share of the period', () => {
    const [, , , a] = layerAverage('repeating-linear-gradient(0deg, rgba(255, 255, 255, 1) 0 1px, transparent 1px 4px)', [100, 100]);
    expect(a).toBeCloseTo(0.25, 2);
  });

  it('counts a star by the area of its dot in each repeat', () => {
    const [, , , a] = layerAverage('radial-gradient(2px 2px at 10px 10px, #ffffff 50%, transparent 100%) 0 0 / 100px 100px', [1920, 1080]);
    expect(a).toBeGreaterThan(0);
    expect(a).toBeLessThan(0.002);
  });

  it('counts a band that does not repeat by its share of the box', () => {
    const [, , , a] = layerAverage('linear-gradient(#ffffff, #ffffff) 0 0 / 100% 3px no-repeat', [600, 300]);
    expect(a).toBeCloseTo(0.01, 4);
  });
});

describe('rateTheme', () => {
  it('calls a true black backdrop with black tiles OLED-friendly', () => {
    expect(rateTheme(BLACK)).toEqual({ rating: 'oled-friendly', level: 0 });
  });

  it('still does with a few faint stars', () => {
    const stars = 'radial-gradient(1px 1px at 9px 9px, rgba(255, 255, 255, 0.6) 50%, transparent 100%) 0 0 / 211px 197px';
    expect(rateTheme({ ...BLACK, backdrop: stars }).rating).toBe('oled-friendly');
  });

  it('calls a backdrop that is not pure black OK, however dark', () => {
    expect(rateTheme({ ...BLACK, 'surface-0': '#050505' }).rating).toBe('ok');
  });

  it('calls lit tiles on black OK', () => {
    expect(rateTheme({ ...BLACK, 'surface-1': '#1a1b1e' }).rating).toBe('ok');
  });

  it('calls a light theme bright, from tiles and backdrop together', () => {
    const light = rateTheme({ 'surface-0': '#e9e9e5', 'surface-1': '#ffffff', backdrop: 'none' });
    expect(light.rating).toBe('bright');
    expect(light.level).toBeCloseTo(0.6 * 1 + 0.4 * pictureLevel(parseColor('#e9e9e5')), 3);
  });

  it('follows var() to the theme\'s own properties', () => {
    expect(rateTheme({ ...BLACK, 'surface-1': 'var(--surface-0)' }).rating).toBe('oled-friendly');
  });
});

describe('themeTokens', () => {
  it('reads the custom properties of each theme and skips the typeface rules and comments', () => {
    const css = `/* --surface-0: #fff; */ .screen[data-theme='a'] { --surface-0: #000; --backdrop: none; }
      .screen[data-theme][data-font='x'] { --font-ui: x; }`;
    expect([...themeTokens(css)]).toEqual([['a', { 'surface-0': '#000', backdrop: 'none' }]]);
  });
});
