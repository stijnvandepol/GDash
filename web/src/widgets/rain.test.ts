import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Snapshot } from '../protocol';
import {
  intensityOf,
  rainKey,
  rainSentence,
  rainView,
  rainWaitingText,
  readForecast,
  summarize,
  upcoming,
  type RainForecast,
  type RainPoint,
} from './rain';

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

/** Five-minute steps from 14:05 Dutch summer time, with these intensities. */
function steps(mm: number[], from = '14:05'): RainPoint[] {
  const start = Date.parse(`2026-10-13T${from}:00+02:00`);
  return mm.map((mmPerHour, i) => {
    const at = new Date(start + i * 300_000 + 2 * 3_600_000).toISOString().slice(0, 16);
    return { time: `${at}:00+02:00`, mmPerHour };
  });
}

const at = (time: string) => Date.parse(`2026-10-13T${time}:00+02:00`);

// The steps of internal/rain/testdata/buienradar-shower.txt: dry until
// 14:25, a shower that peaks at 10 mm/h at 14:45, dry again from 15:05.
const shower = steps([0, 0, 0, 0, 0.1, 0.25, 0.75, 1.78, 10, 3.16, 0.52, 0.1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
const forecast: RainForecast = { source: 'buienradar', stepMinutes: 5, points: shower };

const entry = { fetchedAt: '0001-01-01T00:00:00Z', staleAfter: '0001-01-01T00:00:00Z', failing: false };

describe('intensityOf', () => {
  it('uses the documented bounds', () => {
    expect(intensityOf(0)).toBeUndefined();
    expect(intensityOf(0.09)).toBeUndefined();
    expect(intensityOf(0.1)).toBe('light');
    expect(intensityOf(2.49)).toBe('light');
    expect(intensityOf(2.5)).toBe('moderate');
    expect(intensityOf(9.99)).toBe('moderate');
    expect(intensityOf(10)).toBe('heavy');
  });
});

describe('rainKey', () => {
  // The same cases as TestKeysMatchTheWidget in internal/rain.
  it('rounds a place to two decimals like the server', () => {
    expect(rainKey({ place: { name: 'Oss', latitude: 51.765, longitude: 5.514 } })).toBe('51.77,5.51');
    expect(rainKey({ place: { name: 'Sea', latitude: 52.1, longitude: -0.001 } })).toBe('52.1,0');
  });

  it('follows the weather location without a place, and skips a place that is none', () => {
    expect(rainKey(undefined)).toBe('here');
    expect(rainKey({})).toBe('here');
    expect(rainKey({ place: 'Oss' })).toBeUndefined();
  });
});

describe('readForecast', () => {
  it('reads the forecast under the key and drops broken steps', () => {
    const value = { here: { source: 'open-meteo', stepMinutes: 15, points: [{ time: '2026-10-13T14:00:00+02:00', mmPerHour: 1 }, { time: 'soon' }, null] } };
    expect(readForecast(value, 'here')).toEqual({ source: 'open-meteo', stepMinutes: 15, points: [{ time: '2026-10-13T14:00:00+02:00', mmPerHour: 1 }] });
  });

  it('is undefined for a missing key or a value that is no forecast', () => {
    expect(readForecast({ here: forecast }, '51.77,5.53')).toBeUndefined();
    expect(readForecast({ here: { source: 'buienradar', stepMinutes: 0, points: [] } }, 'here')).toBeUndefined();
    expect(readForecast(null, 'here')).toBeUndefined();
    expect(readForecast([forecast], 'here')).toBeUndefined();
    expect(readForecast({ here: forecast }, undefined)).toBeUndefined();
  });
});

describe('upcoming', () => {
  it('keeps the step that holds now and two hours ahead', () => {
    const points = upcoming(forecast, at('14:12'));
    expect(points[0]!.time).toBe('2026-10-13T14:10:00+02:00');
    expect(points.length).toBe(23);
  });
});

describe('summarize', () => {
  it('says dry when no step reaches 0.1 mm/h', () => {
    expect(summarize(steps([0, 0.05, 0.09, 0]))).toEqual({ kind: 'dry' });
    expect(summarize([])).toEqual({ kind: 'dry' });
  });

  it('says when rain starts, with the heaviest step of the spell', () => {
    expect(summarize(shower)).toEqual({ kind: 'from', time: '14:25', intensity: 'heavy' });
    expect(summarize(steps([0, 0.3, 0.4, 0, 20]))).toEqual({ kind: 'from', time: '14:10', intensity: 'light' });
  });

  it('says until when the first dry step starts while it rains now', () => {
    expect(summarize(steps([3, 4, 0.2, 0, 0]))).toEqual({ kind: 'until', time: '14:20', intensity: 'moderate' });
  });

  it('says rain all along when no dry step comes', () => {
    expect(summarize(steps([0.2, 0.3, 0.4]))).toEqual({ kind: 'all', intensity: 'light' });
  });

  it('runs past midnight', () => {
    expect(summarize(steps([1, 1, 0], '23:55'))).toEqual({ kind: 'until', time: '00:05', intensity: 'light' });
  });
});

describe('rainSentence', () => {
  it('speaks the household language', () => {
    expect(rainSentence({ kind: 'dry' })).toBe('Dry for the next 2 hours');
    expect(rainSentence({ kind: 'from', time: '14:20', intensity: 'light' })).toBe('Rain from 14:20 (light)');
    expect(rainSentence({ kind: 'until', time: '15:05', intensity: 'heavy' })).toBe('Rain until 15:05 (heavy)');
    expect(rainSentence({ kind: 'all', intensity: 'moderate' })).toBe('Rain for the next 2 hours (moderate)');
    speak('nl');
    expect(rainSentence({ kind: 'dry' })).toBe('Droog de komende 2 uur');
    expect(rainSentence({ kind: 'from', time: '14:20', intensity: 'light' })).toBe('Regen vanaf 14:20 (licht)');
    speak('de');
    expect(rainSentence({ kind: 'until', time: '15:05', intensity: 'moderate' })).toBe('Regen bis 15:05 (mäßig)');
    speak('fr');
    expect(rainSentence({ kind: 'from', time: '14:20', intensity: 'heavy' })).toBe('Pluie à partir de 14:20 (forte)');
  });
});

describe('rainView', () => {
  it('draws a bar per step, flat when dry and full at heavy rain', () => {
    const view = rainView(forecast, at('14:05'))!;
    expect(view.bars.length).toBe(24);
    expect(view.bars[0]).toEqual({ height: 0, intensity: undefined });
    expect(view.bars[4]!.height).toBeGreaterThan(0);
    expect(view.bars[8]).toEqual({ height: 1, intensity: 'heavy' });
    expect(view.middle).toBe('15:05');
    expect(view.last).toBe('16:00');
    expect(view.source).toBe('Buienradar.nl');
  });

  it('is undefined when the whole forecast lies in the past', () => {
    expect(rainView(forecast, at('16:30'))).toBeUndefined();
  });
});

describe('rainWaitingText', () => {
  it('asks for a place or a weather location when the server has none to follow', () => {
    expect(rainWaitingText({ ...entry, value: {} }, undefined)).toBe('Set a weather location under Settings, or choose a place for this widget.');
    expect(rainWaitingText(undefined, undefined)).toBe('Set a weather location under Settings, or choose a place for this widget.');
    expect(rainWaitingText(entry, { place: { name: 'Oss', latitude: 51.77, longitude: 5.53 } })).toBe('Loading the rain forecast…');
    expect(rainWaitingText({ ...entry, failing: true }, undefined)).toBe('Could not load the rain forecast');
  });
});
