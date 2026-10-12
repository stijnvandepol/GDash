import { describe, expect, it } from 'vitest';
import { defaultSettings, isoDate, placeSetting } from './widgetSettings';
import type { WidgetType } from './api';

const today = new Date(2026, 9, 9);

describe('defaultSettings', () => {
  it('fills in every setting a type takes, and nothing else', () => {
    const countdown: WidgetType = {
      minW: 4,
      minH: 4,
      settings: { label: { kind: 'text', min: 1, max: 60, required: true }, date: { kind: 'date', required: true } },
    };
    expect(defaultSettings(countdown, today)).toEqual({ label: 'Holiday', date: '2026-11-08' });
  });

  it('gives a widget without settings an empty object', () => {
    expect(defaultSettings({ minW: 4, minH: 3, settings: {} }, today)).toEqual({});
  });

  it('leaves a calendar unset, so the person chooses one and no calendar is picked for them', () => {
    const waste: WidgetType = { minW: 6, minH: 4, settings: { calendar: { kind: 'calendar', required: true } } };
    expect(defaultSettings(waste, today)).toEqual({});
  });

  it('leaves the stop of a departures widget to be chosen', () => {
    const departures: WidgetType = {
      minW: 6,
      minH: 4,
      settings: {
        stop: { kind: 'stop', required: true },
        stopName: { kind: 'text', min: 1, max: 100, required: false },
        maxItems: { kind: 'integer', min: 1, max: 20, required: false },
      },
    };
    expect(defaultSettings(departures, today)).toEqual({ maxItems: 5 });
  });

  it('starts a fuel widget on Euro 95 within 5 km and leaves the place to follow the weather location', () => {
    const fuel: WidgetType = {
      minW: 6,
      minH: 4,
      settings: {
        place: { kind: 'place', required: false },
        radius: { kind: 'integer', min: 1, max: 25, required: false },
        fuel: { kind: 'choice', options: ['EURO95', 'DIESEL'], required: false },
        maxItems: { kind: 'integer', min: 1, max: 10, required: false },
      },
    };
    expect(defaultSettings(fuel, today)).toEqual({ radius: 5, fuel: 'EURO95', maxItems: 5 });
  });

  it('starts a Custom widget as one value, without borrowing the countdown label', () => {
    const custom: WidgetType = {
      minW: 4,
      minH: 3,
      settings: {
        source: { kind: 'customSource', required: true },
        mode: { kind: 'choice', options: ['value', 'list', 'keyValues'], required: true },
        label: { kind: 'text', max: 40, required: false },
      },
    };
    expect(defaultSettings(custom, today)).toEqual({ mode: 'value' });
  });

  it('skips settings it has no default for', () => {
    const odd: WidgetType = { minW: 1, minH: 1, settings: { colour: { kind: 'text', required: false } } };
    expect(defaultSettings(odd, today)).toEqual({});
  });
});

describe('placeSetting', () => {
  it('keeps the name and the place to about 100 m, which is how the server keys a request', () => {
    const oss = { name: 'Oss', region: 'North Brabant, Netherlands', latitude: 51.7654321, longitude: 5.5178765 };
    expect(placeSetting(oss)).toEqual({ name: 'Oss', latitude: 51.765, longitude: 5.518 });
  });

  it('cuts a name at the 100 characters the server accepts', () => {
    expect(placeSetting({ name: 'x'.repeat(130), region: '', latitude: 0, longitude: 0 }).name).toHaveLength(100);
  });
});

describe('isoDate', () => {
  it('pads months and days', () => {
    expect(isoDate(new Date(2026, 0, 5))).toBe('2026-01-05');
  });
});
