import { describe, expect, it } from 'vitest';
import { translate, type Language } from '../i18n.svelte';
import type { Entry } from '../protocol';
import { tileContent, tileTitle, titleFits, titleIsStale, type Text } from './titles';

const textIn =
  (language: Language): Text =>
  (key, values) =>
    translate(language, key, values);
const en = textIn('en');
const nl = textIn('nl');

const NOW = Date.parse('2026-10-14T12:00:00Z');
const entry = (value: unknown, staleAfter = '2026-10-14T13:00:00Z'): Entry => ({ value, fetchedAt: '2026-10-14T11:00:00Z', staleAfter, failing: false });

describe('tileTitle', () => {
  it('names the tiles that do not explain themselves, in the language of the TV', () => {
    expect(tileTitle({ type: 'homeypresence' }, undefined, en)).toBe('Who’s home');
    expect(tileTitle({ type: 'homeypresence' }, undefined, nl)).toBe('Wie is thuis');
    expect(tileTitle({ type: 'waste' }, undefined, en)).toBe('Waste collection');
    expect(tileTitle({ type: 'waste' }, undefined, nl)).toBe('Afval');
    expect(tileTitle({ type: 'moon' }, undefined, textIn('de'))).toBe('Mond');
    expect(tileTitle({ type: 'energyprice' }, undefined, en)).toBe('Electricity price');
    expect(tileTitle({ type: 'homeassistant' }, undefined, nl)).toBe('Home Assistant');
  });

  it('leaves the tiles that explain themselves without a title', () => {
    for (const type of ['clock', 'weather', 'calendar', 'forecast', 'sun', 'countdown', 'text', 'photos']) {
      expect(tileTitle({ type }, undefined, en), type).toBeNull();
    }
  });

  it('builds titles from the settings and the data', () => {
    expect(tileTitle({ type: 'departures', settings: { stop: '1234', stopName: 'Utrecht, Centraal' } }, undefined, en)).toBe('Utrecht, Centraal');
    expect(tileTitle({ type: 'departures', settings: {} }, undefined, nl)).toBe('Vertrektijden');
    const place = { name: 'Oss', latitude: 51.76, longitude: 5.52 };
    expect(tileTitle({ type: 'fuel', settings: { place, fuel: 'DIESEL' } }, undefined, en)).toBe('Fuel · Oss · Diesel');
    expect(tileTitle({ type: 'rain', settings: { place } }, undefined, en)).toBe('Rain · Oss');
    expect(tileTitle({ type: 'rain' }, undefined, en)).toBe('Rain');
    expect(tileTitle({ type: 'traffic', settings: { roads: ['a2', 'A67'] } }, undefined, en)).toBe('Traffic · A2, A67');
    const warnings = entry({ here: { region: 'Noord-Brabant', warnings: [] } });
    expect(tileTitle({ type: 'warnings' }, warnings, en)).toBe('Weather warnings · Noord-Brabant');
    const news = entry({ '1': { name: 'NOS Nieuws', items: [] }, '2': { name: 'Tweakers', items: [] } });
    expect(tileTitle({ type: 'news', settings: { feeds: [2] } }, news, en)).toBe('Tweakers');
    expect(tileTitle({ type: 'news' }, news, nl)).toBe('Nieuws');
    const pages = entry({ '3': { name: 'Buienradar', url: 'https://www.buienradar.nl/' } });
    expect(tileTitle({ type: 'webpage', settings: { page: 3 } }, pages, en)).toBe('Buienradar');
    expect(tileTitle({ type: 'webpage', settings: { page: 4 } }, pages, en)).toBe('Web page');
  });

  it('calls a Custom widget by its own title, and its badges Who’s home without one', () => {
    expect(tileTitle({ type: 'custom', settings: { mode: 'value', title: 'Stroom' } }, undefined, en)).toBe('Stroom');
    expect(tileTitle({ type: 'custom', settings: { mode: 'badges' } }, undefined, nl)).toBe('Wie is thuis');
    expect(tileTitle({ type: 'custom', settings: { mode: 'value' } }, undefined, en)).toBeNull();
  });

  it('shows the admin’s title instead, or none', () => {
    expect(tileTitle({ type: 'homeypresence', settings: { title: 'Thuis' } }, undefined, en)).toBe('Thuis');
    expect(tileTitle({ type: 'clock', settings: { title: 'Keuken' } }, undefined, en)).toBe('Keuken');
    expect(tileTitle({ type: 'waste', settings: { title: '   ' } }, undefined, en)).toBe('Waste collection');
    expect(tileTitle({ type: 'waste', settings: { hideTitle: true } }, undefined, en)).toBeNull();
    expect(tileTitle({ type: 'custom', settings: { mode: 'value', title: 'Stroom', hideTitle: true } }, undefined, en)).toBeNull();
  });
});

describe('titleIsStale', () => {
  it('follows the entry, and a Custom widget its own state', () => {
    expect(titleIsStale({ type: 'waste' }, entry([], '2026-10-14T11:30:00Z'), 'd/w', NOW)).toBe(true);
    expect(titleIsStale({ type: 'waste' }, entry([]), 'd/w', NOW)).toBe(false);
    expect(titleIsStale({ type: 'waste' }, undefined, 'd/w', NOW)).toBe(false);
    const custom = entry({ 'd/w': { state: 'ok', mode: 'value', stale: true } });
    expect(titleIsStale({ type: 'custom' }, custom, 'd/w', NOW)).toBe(true);
    expect(titleIsStale({ type: 'custom' }, custom, 'd/other', NOW)).toBe(false);
  });
});

describe('the room for a title', () => {
  const stage = { width: 1920, height: 1080 };
  const grid = { columns: 24, rows: 14 };
  const space = { gap: 24, padding: 48 };

  it('measures a tile as App lays it out', () => {
    // Fourteen rows of (1080 - 48 - 13 * 24) / 14 px.
    const content = tileContent(stage, grid, { w: 6, h: 4 }, space);
    expect(content.height).toBeCloseTo((720 / 14) * 4 + 3 * 24 - 96, 6);
    expect(content.width).toBeCloseTo((1320 / 24) * 6 + 5 * 24 - 96, 6);
  });

  it('fits the smallest tiles at 100 %, and is left out where the data needs the room', () => {
    expect(titleFits(tileContent(stage, grid, { w: 6, h: 4 }, space))).toBe(true);
    expect(titleFits(tileContent(stage, grid, { w: 4, h: 3 }, space))).toBe(true);
    // At 150 % text the stage is 1280 wide and a 3-row tile too low.
    const big = { width: 1280, height: 720 };
    expect(titleFits(tileContent(big, grid, { w: 6, h: 3 }, { gap: 16, padding: 32 }))).toBe(false);
    expect(titleFits(tileContent(big, grid, { w: 6, h: 4 }, { gap: 16, padding: 32 }))).toBe(true);
    expect(titleFits({ width: 120, height: 300 })).toBe(false);
  });
});
