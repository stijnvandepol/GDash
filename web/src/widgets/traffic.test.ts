import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Snapshot } from '../protocol';
import {
  camerasText,
  emptyText,
  lengthText,
  showOf,
  trafficKey,
  trafficResult,
  trafficRows,
  trafficWaitingText,
  type TrafficResult,
} from './traffic';

function speak(language: string) {
  display.snapshot = { language } as Snapshot;
}

beforeEach(() => {
  vi.stubGlobal('navigator', { language: 'en-US' });
});

afterEach(() => {
  display.snapshot = null;
  vi.unstubAllGlobals();
});

const now = new Date('2026-10-13T08:40:00Z');

const result: TrafficResult = {
  incidents: true,
  jams: [
    { road: 'A2', from: 'Born', to: 'Urmond', lengthKm: 5.4, delayMinutes: 11, reason: 'Stilstaand verkeer.' },
    { road: 'N3', from: 'Papendrecht', to: 'Papendrecht', closed: true },
  ],
  roadworks: [{ road: 'A2', from: 'Weert-Noord', to: 'Grathem', closed: true, start: '2026-10-13T19:00:00Z', stop: '2026-10-14T03:00:00Z' }],
  speedChecks: [{ road: 'A2', from: 'Urmond', to: 'Born', hectometre: 236.1 }],
  cameras: [{ kind: 'fixed', road: 'A2', hectometre: 236.4, maxSpeed: 100 }],
  cameraCount: 3,
  totals: { jams: 15, lengthKm: 41, delayMinutes: 54 },
};

const entry = { fetchedAt: '0001-01-01T00:00:00Z', staleAfter: '0001-01-01T00:00:00Z', failing: false };

describe('keys', () => {
  // The same cases as TestKeysMatchTheWidget in internal/traffic.
  it('lists the roads in capitals, sorted, once each', () => {
    expect(trafficKey({ roads: ['n279', 'A50', 'A2', 'a2'] })).toBe('roads:A2,A50,N279');
  });
  it('watches a radius without roads', () => {
    expect(trafficKey({})).toBe('near:25');
    expect(trafficKey({ radius: 10 })).toBe('near:10');
    expect(trafficKey(undefined)).toBe('near:25');
  });
});

describe('what a widget shows', () => {
  it('defaults to jams, roadworks and speed checks', () => {
    expect(showOf(undefined)).toEqual(['jams', 'roadworks', 'speedchecks']);
    expect(showOf({ show: ['cameras', 'jams', 'weather'] })).toEqual(['jams', 'cameras']);
  });

  it('reads only a result of the right shape', () => {
    expect(trafficResult({ 'near:25': result }, 'near:25')).toBe(result);
    expect(trafficResult({ 'near:25': { jams: 'none' } }, 'near:25')).toBeUndefined();
    expect(trafficResult({ 'near:10': result }, 'near:25')).toBeUndefined();
    expect(trafficResult(null, 'near:25')).toBeUndefined();
  });
});

describe('rows', () => {
  it('writes a jam as the household reads it', () => {
    const [jam, closed] = trafficRows(result, ['jams'], 5, now);
    expect(jam).toMatchObject({ road: 'A2', place: 'Born → Urmond', detail: '5 km · +11 min', note: 'Stilstaand verkeer.', severe: true });
    // A closure with the same place at both ends names it once.
    expect(closed).toMatchObject({ road: 'N3', place: 'Papendrecht', detail: 'Closed', severe: true });
  });

  it('says when roadworks start later today', () => {
    const [works] = trafficRows(result, ['roadworks'], 5, now);
    expect(works!.detail).toMatch(/^Closed · from \d\d:\d\d$/);
    // Not yet: not marked.
    expect(works!.severe).toBe(false);
  });

  it('names speed checks and cameras by their marker post', () => {
    const rows = trafficRows(result, ['speedchecks', 'cameras'], 5, now);
    expect(rows.map((r) => [r.road, r.place, r.detail])).toEqual([
      ['A2', 'hm 236.1', 'Speed check'],
      ['A2', 'Speed camera · hm 236.4', '100 km/h'],
    ]);
  });

  it('keeps the order of the kinds and stops at the maximum', () => {
    const rows = trafficRows(result, ['speedchecks', 'jams', 'roadworks'], 3, now);
    expect(rows.map((r) => r.kind)).toEqual(['jam', 'jam', 'roadworks']);
  });

  it('speaks Dutch, with the ANWB texts as sent', () => {
    speak('nl');
    const [jam] = trafficRows(result, ['jams'], 1, now);
    expect(jam!.note).toBe('Stilstaand verkeer.');
    expect(trafficRows(result, ['speedchecks'], 1, now)[0]!.detail).toBe('Snelheidscontrole');
    expect(trafficRows(result, ['cameras'], 1, now)[0]!.place).toBe('Flitspaal · hm 236,4');
    expect(lengthText(0.8)).toBe('0,8 km');
  });
});

describe('texts', () => {
  it('is calm when nothing is wrong', () => {
    expect(emptyText({ roads: ['A2'] }, ['jams'])).toBe('No jams on your roads');
    expect(emptyText({ radius: 10 }, ['jams', 'cameras'])).toBe('No jams within 10 km');
    expect(emptyText({ roads: ['A2'] }, ['speedchecks'])).toBe('Nothing on your roads');
  });

  it('counts cameras on the roads or around home', () => {
    expect(camerasText({ roads: ['A2'] }, 3)).toBe('Fixed cameras on your roads: 3');
    expect(camerasText({}, 2)).toBe('Fixed cameras within 25 km: 2');
  });

  it('says what to set up', () => {
    expect(trafficWaitingText(undefined, {})).toBe('Turn on ANWB traffic info under Integrations.');
    expect(trafficWaitingText({ ...entry, value: {} }, {})).toBe('Set a weather location under Integrations, or choose roads for this widget.');
    expect(trafficWaitingText({ ...entry, failing: true }, { roads: ['A2'] })).toBe('Could not load the traffic');
  });
});
