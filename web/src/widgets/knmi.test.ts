import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Snapshot } from '../protocol';
import { humidityText, measuredText, readObservation, showsMeasured } from './knmi';

beforeEach(() => {
  vi.stubGlobal('navigator', { language: 'en-US' });
});

afterEach(() => {
  display.snapshot = null;
  vi.unstubAllGlobals();
});

const volkel = { station: 'Volkel', distanceKm: 6.1, observedAt: '2026-10-13T10:10:00Z', temperatureC: 12.6, humidity: 86, windKmh: 15.1 };

describe('measured weather', () => {
  it('is used only when the widget asks for it', () => {
    expect(showsMeasured({ current: 'knmi' })).toBe(true);
    expect(showsMeasured({ current: 'model' })).toBe(false);
    expect(showsMeasured(undefined)).toBe(false);
  });

  it('reads an observation and leaves out what the station did not report', () => {
    expect(readObservation(volkel)).toEqual({ station: 'Volkel', observedAt: volkel.observedAt, temperatureC: 12.6, humidity: 86, windKmh: 15.1 });
    expect(readObservation({ ...volkel, humidity: null, windKmh: 'calm' })).toMatchObject({ humidity: null, windKmh: null });
  });

  it('needs a station, a time and a temperature', () => {
    expect(readObservation({ ...volkel, temperatureC: null })).toBeUndefined();
    expect(readObservation({ ...volkel, observedAt: 'yesterday' })).toBeUndefined();
    expect(readObservation({ temperatureC: 12 })).toBeUndefined();
    expect(readObservation(undefined)).toBeUndefined();
  });

  it('names the station and the time', () => {
    expect(measuredText(readObservation(volkel)!)).toMatch(/^Measured at Volkel \d\d:\d0$/);
    display.snapshot = { language: 'nl' } as Snapshot;
    expect(humidityText(86.4)).toBe('Luchtvochtigheid 86%');
  });
});
