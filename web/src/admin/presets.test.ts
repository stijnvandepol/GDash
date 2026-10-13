import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import type { CustomSource } from './api';
import { AUTH_KINDS, INTERVALS } from './customSources';
import { CATEGORIES, TAGS } from './integrations';
import {
  PRESETS,
  answersFrom,
  fillAddress,
  madeFrom,
  ownSources,
  placeholderProblem,
  placeholdersIn,
  presetInput,
  presetOf,
  shownList,
  shownValue,
  sourcesFrom,
  type Preset,
} from './presets';

const icons = readFileSync(new URL('./Icon.svelte', import.meta.url), 'utf8');

const p1 = PRESETS.find((p) => p.id === 'homewizard-p1')!;

function source(url: string, mode: 'fetch' | 'push' = 'fetch', preset = ''): CustomSource {
  return {
    id: 1,
    name: 'P1',
    mode,
    url,
    method: 'GET',
    body: '',
    auth: { kind: 'none', name: '', secretSet: false },
    headers: [],
    intervalSeconds: 30,
    sample: null,
    values: [],
    lists: [],
    preset,
    unreadable: false,
  };
}

describe('the presets', () => {
  it('are bundled', () => {
    expect(PRESETS.length).toBeGreaterThan(0);
    expect(p1).toBeDefined();
  });

  // The server checks the request, values and rules (internal/custom
  // presets_test.go); these are what only the web interface reads.
  it.each(PRESETS.map((p) => [p.id, p] as [string, Preset]))('%s is complete', (_, preset) => {
    expect(CATEGORIES.map((c) => c.id)).toContain(preset.category);
    for (const tag of preset.tags) expect(Object.keys(TAGS)).toContain(tag);
    expect(icons).toContain(`    ${preset.icon}: '`);
    expect(preset.docs).toMatch(/^https:\/\//);
    expect(placeholdersIn(preset.source.url)).toEqual(preset.placeholders.map((p) => p.key));
    for (const p of preset.placeholders) expect(placeholderProblem(p.kind, p.example)).toBe('');
    expect(AUTH_KINDS.map((a) => a.kind)).toContain(preset.source.auth.kind);
    expect(INTERVALS.map((i) => i.seconds)).toContain(preset.source.intervalSeconds);
    expect(preset.instance).toMatch(/^[a-z][a-z ]{1,20}$/);
  });

  // The server checks that each view makes a widget it takes and that shows
  // the sample; these are what the editor needs to offer it.
  it.each(PRESETS.map((p) => [p.id, p] as [string, Preset]))('%s has views the editor can offer', (_, preset) => {
    expect(preset.views.length).toBeGreaterThan(0);
    expect(preset.views.length).toBeLessThanOrEqual(3);
    expect(new Set(preset.views.map((v) => v.id)).size).toBe(preset.views.length);
    for (const view of preset.views) {
      // The Custom widget is at least 4 by 3 on a grid of 24 by 14.
      expect(view.w).toBeGreaterThanOrEqual(4);
      expect(view.h).toBeGreaterThanOrEqual(3);
      expect(view.w).toBeLessThanOrEqual(12);
      expect(view.h).toBeLessThanOrEqual(7);
      expect(view.name.length).toBeGreaterThan(0);
      for (const shown of view.values ?? []) expect(preset.values.map((v) => v.name)).toContain(shown.name);
      if (view.mode === 'list') expect(preset.lists.map((l) => l.name)).toContain(view.list);
      else expect(view.values?.length).toBeGreaterThan(0);
    }
  });
});

describe('answersFrom', () => {
  it('reads the placeholders back from an address, decoded as they were filled in', () => {
    expect(answersFrom(p1, 'http://192.168.1.50/api/v1/data')).toEqual({ ip: '192.168.1.50' });
    const todoist = PRESETS.find((p) => p.id === 'todoist')!;
    const url = fillAddress(todoist, { filter: 'today | #Home' });
    expect(answersFrom(todoist, url)).toEqual({ filter: 'today | #Home' });
    const piHole = PRESETS.find((p) => p.id === 'pi-hole')!;
    expect(answersFrom(piHole, 'http://192.168.1.2:8080/api/stats/summary')).toEqual({ server: 'http://192.168.1.2:8080' });
  });

  it('gives null for an address of another shape', () => {
    expect(answersFrom(p1, 'http://192.168.1.50/api/v2/data')).toBeNull();
  });
});

describe('the sources of a preset', () => {
  it('are those that keep its id; the rest are the household’s own', () => {
    const made = { ...source('http://192.168.1.50/api/v1/data', 'fetch', 'homewizard-p1'), id: 2 };
    const own = source('https://api.example.com/x');
    const other = { ...source('http://192.168.1.51/api/v1/data', 'fetch', 'homewizard-kwh'), id: 3 };
    expect(madeFrom(p1, [own, made, other])).toEqual([made]);
    expect(ownSources([own, made, other])).toEqual([own]);
    expect(presetOf(made)).toBe(p1);
    expect(presetOf(own)).toBeUndefined();
    expect(presetOf({ preset: 'not-a-preset' })).toBeUndefined();
  });
});

describe('shownValue and shownList', () => {
  it('say what a named value or list holds, with its unit', () => {
    expect(shownValue({ a: 1234 }, { name: 'Power', path: 'a', unit: 'W' })).toBe('1234 W');
    expect(shownValue({ a: '12 Mbps' }, { name: 'Down', path: 'a' })).toBe('12 Mbps');
    expect(shownValue({}, { name: 'Down', path: 'a' })).toBe('Not in the answer');
    expect(shownList({ l: [1, 2] }, { name: 'L', path: 'l', columns: [] })).toBe('2 items');
    expect(shownList({ l: {} }, { name: 'L', path: 'l', columns: [] })).toBe('Not in the answer');
  });
});

describe('placeholderProblem', () => {
  it('takes an IP address or host name, with a port, for a host', () => {
    expect(placeholderProblem('host', '192.168.1.50')).toBe('');
    expect(placeholderProblem('host', 'p1meter.local:80')).toBe('');
    expect(placeholderProblem('host', 'http://192.168.1.50')).toMatch(/without http/);
    expect(placeholderProblem('host', '192.168.1.50/api')).not.toBe('');
    expect(placeholderProblem('host', '  ')).toBe('Fill this in.');
  });

  it('takes a whole http address for an origin', () => {
    expect(placeholderProblem('origin', 'http://192.168.1.20:8096')).toBe('');
    expect(placeholderProblem('origin', 'https://media.example.com/jellyfin')).toBe('');
    expect(placeholderProblem('origin', '192.168.1.20:8096')).not.toBe('');
    expect(placeholderProblem('origin', 'ftp://192.168.1.20')).toMatch(/http/);
    expect(placeholderProblem('origin', 'http://h/?a=1')).toMatch(/without/);
  });

  it('takes letters, digits, - and _ for an id', () => {
    expect(placeholderProblem('id', '1234567')).toBe('');
    expect(placeholderProblem('id', '12/34')).not.toBe('');
  });

  it('takes any text up to 200 characters', () => {
    expect(placeholderProblem('text', 'today | overdue')).toBe('');
    expect(placeholderProblem('text', 'x'.repeat(201))).not.toBe('');
  });
});

describe('fillAddress', () => {
  it('puts a host in as typed and encodes text', () => {
    expect(fillAddress(p1, { ip: ' 192.168.1.50 ' })).toBe('http://192.168.1.50/api/v1/data');
    const todoist = PRESETS.find((p) => p.id === 'todoist')!;
    expect(fillAddress(todoist, { filter: 'today | #Home' })).toBe('https://api.todoist.com/api/v1/tasks/filter?query=today%20%7C%20%23Home');
  });

  it('drops the trailing slash of an origin', () => {
    const plex = PRESETS.find((p) => p.id === 'plex')!;
    expect(fillAddress(plex, { server: 'http://192.168.1.20:32400/' })).toBe('http://192.168.1.20:32400/status/sessions');
  });
});

describe('presetInput', () => {
  it('makes a fetched custom API with the sample, and the secret only when it has one', () => {
    const input = presetInput(p1, { name: ' Meter ', answers: { ip: '10.0.0.2' }, secret: 'ignored', user: '' });
    expect(input).toMatchObject({ name: 'Meter', mode: 'fetch', url: 'http://10.0.0.2/api/v1/data', method: 'GET', intervalSeconds: 30 });
    expect(input.auth).toEqual({ kind: 'none', name: '', secret: '' });
    expect(input.sample).toEqual(p1.source.sample);
    expect(input.preset).toBe('homewizard-p1');
    expect(presetInput(p1, { name: 'M', answers: { ip: '10.0.0.2' }, secret: '', user: '', intervalSeconds: 60 }).intervalSeconds).toBe(60);

    const tibber = PRESETS.find((p) => p.id === 'tibber')!;
    const post = presetInput(tibber, { name: 'Tibber', answers: {}, secret: ' token ', user: 'ignored' });
    expect(post.auth).toEqual({ kind: 'bearer', name: '', secret: 'token' });
    expect(JSON.parse(post.body)).toHaveProperty('query');

    const adguard = PRESETS.find((p) => p.id === 'adguard-home')!;
    const basic = presetInput(adguard, { name: 'DNS', answers: { server: 'http://10.0.0.3:3000' }, secret: 'pw', user: ' admin ' });
    expect(basic.auth).toEqual({ kind: 'basic', name: 'admin', secret: 'pw' });
  });

  it('logs in to Pi-hole on the address the household fills in', () => {
    const piHole = PRESETS.find((p) => p.id === 'pi-hole')!;
    const input = presetInput(piHole, { name: 'Pi-hole', answers: { server: 'http://192.168.1.2:8080/' }, secret: ' app-pw ', user: '' });
    expect(input.url).toBe('http://192.168.1.2:8080/api/stats/summary');
    expect(input.auth).toEqual({
      kind: 'login',
      name: 'X-FTL-SID',
      secret: 'app-pw',
      login: { url: 'http://192.168.1.2:8080/api/auth', body: '{"password": "{secret}"}', tokenPath: 'session.sid', place: 'header', validityPath: 'session.validity' },
    });
    // The preset itself keeps its placeholder for the next one.
    expect(piHole.source.auth.login?.url).toBe('{server}/api/auth');
  });
});

describe('sourcesFrom', () => {
  it('finds the custom APIs made from a preset by their address and named paths', () => {
    const made = { ...source('http://192.168.1.50/api/v1/data'), values: [...p1.values].reverse() };
    const otherAddress = { ...made, url: 'http://192.168.1.50/api/v2/data' };
    expect(sourcesFrom(p1, [made, otherAddress, source('', 'push')])).toEqual([made]);
  });

  it('tells presets with the same address apart by their values', () => {
    const kwh = PRESETS.find((p) => p.id === 'homewizard-kwh')!;
    const made = { ...source('http://192.168.1.50/api/v1/data'), values: p1.values };
    expect(sourcesFrom(kwh, [made])).toEqual([]);
    expect(sourcesFrom(p1, [made])).toEqual([made]);
  });

  it('only looks at own APIs: a source that keeps a preset is that preset’s', () => {
    const kept = { ...source('http://192.168.1.50/api/v1/data', 'fetch', 'homewizard-p1'), values: p1.values };
    expect(sourcesFrom(p1, [kept])).toEqual([]);
  });
});
