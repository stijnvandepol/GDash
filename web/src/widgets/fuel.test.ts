import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Snapshot } from '../protocol';
import {
  distanceText,
  fuelKey,
  fuelOf,
  fuelRows,
  fuelWaitingText,
  priceText,
  radiusOf,
  readPlace,
  savingText,
  stationList,
  type Station,
} from './fuel';

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

const stations: Station[] = [
  { name: 'Argos, Heesch', price: 2.369, distanceKm: 2.9 },
  { name: 'Total Danenhoef, Oss', price: 2.389, distanceKm: 3 },
  { name: 'Shell Hanenberg (A59 zz), Heesch', price: 2.729, distanceKm: 4.4 },
];

const entry = { fetchedAt: '0001-01-01T00:00:00Z', staleAfter: '0001-01-01T00:00:00Z', failing: false };

describe('keys', () => {
  // The same cases as TestKeysMatchWhatTheWidgetBuilds in internal/fuel.
  it('equal the keys the server builds', () => {
    const oss = { name: 'Oss', latitude: 51.77, longitude: 5.52 };
    expect(fuelKey({ place: oss })).toBe('51.77,5.52,5,EURO95');
    expect(fuelKey({ place: { ...oss, latitude: 51.7654321, longitude: 5.5 }, radius: 10, fuel: 'DIESEL' })).toBe('51.765,5.5,10,DIESEL');
    expect(fuelKey({ place: { name: 'Sea', latitude: -0.0001, longitude: -33.8688 }, radius: 1, fuel: 'CNG' })).toBe('0,-33.869,1,CNG');
    expect(fuelKey({ place: { name: 'Whole', latitude: 52, longitude: 4 }, radius: 25, fuel: 'AUTOGAS' })).toBe('52,4,25,AUTOGAS');
  });

  it('follows the weather location without a place', () => {
    expect(fuelKey({ fuel: 'EURO98', radius: 5 })).toBe('here,5,EURO98');
    expect(fuelKey(undefined)).toBe('here,5,EURO95');
    expect(fuelKey({})).toBe('here,5,EURO95');
  });

  it('is undefined for a place that is no place, as the server skips such a widget', () => {
    expect(fuelKey({ place: 'Oss' })).toBeUndefined();
    expect(fuelKey({ place: { name: 'Oss', latitude: '51.7', longitude: 5 } })).toBeUndefined();
  });

  it('falls back to the defaults for settings that are not usable', () => {
    expect(radiusOf({ radius: '5' })).toBe(5);
    expect(fuelOf({ fuel: 'EURO100' })).toBe('EURO95');
    expect(fuelOf({ fuel: 'DIESEL' })).toBe('DIESEL');
  });
});

describe('stationList', () => {
  it('finds the list under the widget key', () => {
    const value = { 'here,5,EURO95': stations, 'here,5,DIESEL': [] };
    expect(stationList(value, 'here,5,EURO95')).toEqual(stations);
    expect(stationList(value, 'here,5,DIESEL')).toEqual([]);
    expect(stationList(value, 'here,9,DIESEL')).toBeUndefined();
  });

  it('shows nothing for a value that is not a map of lists', () => {
    expect(stationList(null, 'a')).toBeUndefined();
    expect(stationList([stations], 'a')).toBeUndefined();
    expect(stationList('stations', 'a')).toBeUndefined();
    expect(stationList({ a: stations }, undefined)).toBeUndefined();
    expect(stationList({ a: 'x' }, 'a')).toBeUndefined();
  });

  it('skips entries that are no station', () => {
    expect(stationList({ a: [stations[0], { name: 'x' }, null, 7, { ...stations[1], price: '2.4' }] }, 'a')).toEqual([stations[0]]);
  });
});

describe('fuelRows', () => {
  it('marks the first station as the cheapest and cuts the list at max', () => {
    const rows = fuelRows(stations, 2);
    expect(rows.map((r) => [r.name, r.cheapest])).toEqual([
      ['Argos, Heesch', true],
      ['Total Danenhoef, Oss', false],
    ]);
  });

  it('handles an empty list', () => {
    expect(fuelRows([], 5)).toEqual([]);
  });
});

describe('texts', () => {
  it('writes a price with three decimals', () => {
    expect(priceText(2.469)).toBe('€2.469');
    expect(priceText(2.4)).toBe('€2.400');
    expect(priceText(0.929)).toBe('€0.929');
  });

  it('writes the price in the language of the screen', () => {
    speak('de');
    expect(priceText(2.469).replace(/\s/g, ' ')).toBe('2,469 €');
    speak('nl');
    expect(priceText(2.469).replace(/\s/g, ' ')).toBe('€ 2,469');
  });

  it('writes a distance with one decimal', () => {
    expect(distanceText(1.8)).toBe('1.8 km');
    expect(distanceText(3)).toBe('3.0 km');
    expect(distanceText(0)).toBe('0.0 km');
    speak('nl');
    expect(distanceText(1.8)).toBe('1,8 km');
  });

  it('says how much the cheapest row saves against the dearest', () => {
    // 2.729 - 2.369 is 0.36000000000000014 in binary floating point.
    expect(savingText(fuelRows(stations, 5), 'EURO95')).toBe('save €0.36/l');
    expect(savingText(fuelRows(stations, 2), 'EURO95')).toBe('save €0.02/l');
    speak('nl');
    expect(savingText(fuelRows(stations, 5), 'DIESEL')?.replace(/\s/g, ' ')).toBe('bespaar € 0,36/l');
  });

  it('counts CNG per kilogram', () => {
    expect(savingText(fuelRows([{ ...stations[0]!, price: 1.699 }, { ...stations[1]!, price: 1.899 }], 5), 'CNG')).toBe('save €0.20/kg');
  });

  it('rounds a saving of half a cent up and says nothing for less than a cent', () => {
    const pair = (a: number, b: number) => fuelRows([{ ...stations[0]!, price: a }, { ...stations[1]!, price: b }], 5);
    expect(savingText(pair(2.4, 2.405), 'EURO95')).toBe('save €0.01/l');
    expect(savingText(pair(2.4, 2.404), 'EURO95')).toBeUndefined();
    expect(savingText(pair(2.4, 2.4), 'EURO95')).toBeUndefined();
    expect(savingText(fuelRows(stations, 1), 'EURO95')).toBeUndefined();
    expect(savingText([], 'EURO95')).toBeUndefined();
  });
});

describe('fuelWaitingText', () => {
  it('says what to set up when the server sends no entry', () => {
    expect(fuelWaitingText(undefined, {})).toBe('Set a weather location under Integrations, or choose a place for this widget.');
  });

  it('says loading while the first fetch runs, and that it failed', () => {
    expect(fuelWaitingText(entry, { place: { name: 'Oss', latitude: 51.7, longitude: 5.5 } })).toBe('Loading fuel prices…');
    expect(fuelWaitingText({ ...entry, failing: true }, {})).toBe('Could not load the fuel prices');
  });

  it('tells a widget without a place that there is no weather location when others have lists', () => {
    const others = { ...entry, value: { '51.77,5.52,5,EURO95': stations } };
    expect(fuelWaitingText(others, {})).toBe('Set a weather location under Integrations, or choose a place for this widget.');
    expect(fuelWaitingText(others, { place: { name: 'Oss', latitude: 51.7, longitude: 5.5 } })).toBe('Loading fuel prices…');
  });
});

describe('readPlace', () => {
  it('reads a place and nothing else', () => {
    expect(readPlace({ name: 'Oss', latitude: 51.77, longitude: 5.52 })).toEqual({ name: 'Oss', latitude: 51.77, longitude: 5.52 });
    expect(readPlace(undefined)).toBeUndefined();
    expect(readPlace({ name: 'Oss' })).toBeUndefined();
    expect(readPlace('Oss')).toBeUndefined();
  });
});
