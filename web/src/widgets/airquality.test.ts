import { describe, expect, it } from 'vitest';
import { aqiLevel, pollenLevel, readAirQuality } from './airquality';

describe('aqiLevel', () => {
  it.each([
    [0, 'good'],
    [20, 'good'],
    [21, 'fair'],
    [40, 'fair'],
    [41, 'moderate'],
    [60, 'moderate'],
    [61, 'poor'],
    [80, 'poor'],
    [81, 'veryPoor'],
    [100, 'veryPoor'],
    [101, 'extremelyPoor'],
    [350, 'extremelyPoor'],
  ])('maps index %i to %s', (index, level) => {
    expect(aqiLevel(index)).toBe(level);
  });
});

describe('pollenLevel', () => {
  it.each([
    [0.4, 'low'],
    [9.9, 'low'],
    [10, 'moderate'],
    [49, 'moderate'],
    [50, 'high'],
    [199, 'high'],
    [200, 'veryHigh'],
    [5000, 'veryHigh'],
  ])('maps %f grains per cubic metre to %s', (grains, level) => {
    expect(pollenLevel(grains)).toBe(level);
  });
});

describe('readAirQuality', () => {
  it('keeps the pollen that is in the air, the most first', () => {
    const report = {
      europeanAqi: 47.6,
      pollen: [
        { type: 'alder', grainsPerM3: 0 },
        { type: 'birch', grainsPerM3: 12 },
        { type: 'grass', grainsPerM3: 80.5 },
      ],
    };
    expect(readAirQuality(report)).toEqual({
      index: 48,
      hasPollen: true,
      pollen: [
        { type: 'grass', grainsPerM3: 80.5 },
        { type: 'birch', grainsPerM3: 12 },
      ],
    });
  });

  it('knows that there is pollen data but none in the air', () => {
    const report = { europeanAqi: 20, pollen: [{ type: 'alder', grainsPerM3: 0 }] };
    expect(readAirQuality(report)).toEqual({ index: 20, hasPollen: true, pollen: [] });
  });

  it('has no pollen outside Europe', () => {
    expect(readAirQuality({ europeanAqi: 54, pollen: [] })).toEqual({ index: 54, hasPollen: false, pollen: [] });
    expect(readAirQuality({ europeanAqi: 54 })).toEqual({ index: 54, hasPollen: false, pollen: [] });
    expect(readAirQuality({ europeanAqi: 54, pollen: null })).toEqual({ index: 54, hasPollen: false, pollen: [] });
  });

  it('drops pollen entries that are not a kind with a number', () => {
    const report = {
      europeanAqi: 30,
      pollen: [{ type: 'birch', grainsPerM3: null }, { type: 'cedar', grainsPerM3: 5 }, { type: 'olive', grainsPerM3: -1 }, 'x', null],
    };
    expect(readAirQuality(report)).toEqual({ index: 30, hasPollen: false, pollen: [] });
  });

  it('keeps an index of zero', () => {
    expect(readAirQuality({ europeanAqi: 0 })?.index).toBe(0);
  });

  it.each([undefined, null, 5, 'x', {}, { europeanAqi: null }, { europeanAqi: '20' }, { europeanAqi: -1 }, { europeanAqi: NaN }])(
    'is undefined for %j',
    (report) => {
      expect(readAirQuality(report)).toBeUndefined();
    },
  );
});
