import { describe, expect, it } from 'vitest';
import { WIDGETS, defaultSettings, isoDate, paletteGroups, placeSetting, settingLabel } from './widgetSettings';
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

  it('starts a holidays widget on both kinds in the middle region', () => {
    const holidays: WidgetType = {
      minW: 6,
      minH: 4,
      settings: {
        region: { kind: 'choice', options: ['north', 'middle', 'south'], required: false },
        show: { kind: 'choice', options: ['both', 'school', 'public'], required: false },
        maxItems: { kind: 'integer', min: 1, max: 20, required: false },
      },
    };
    expect(defaultSettings(holidays, today)).toEqual({ region: 'middle', show: 'both', maxItems: 5 });
    expect(settingLabel('holidays', 'maxItems')).toBe('Holidays at most');
  });

  it('starts a weather warnings widget quiet, on the province of the weather location', () => {
    const warnings: WidgetType = {
      minW: 6,
      minH: 3,
      settings: {
        region: { kind: 'choice', options: ['Utrecht', 'Zeeland'], required: false },
        announce: { kind: 'boolean', required: false },
      },
    };
    expect(defaultSettings(warnings, today)).toEqual({ announce: false });
  });

  it('starts a Custom widget as one value, without borrowing the countdown label', () => {
    const custom: WidgetType = {
      minW: 4,
      minH: 3,
      settings: {
        source: { kind: 'customSource', required: true },
        mode: { kind: 'choice', options: ['value', 'list', 'keyValues', 'badges'], required: true },
        label: { kind: 'text', max: 40, required: false },
      },
    };
    expect(defaultSettings(custom, today)).toEqual({ mode: 'value' });
  });

  it('starts a Camera widget on a 10-second refresh showing the whole image, with the camera to be chosen', () => {
    const camera: WidgetType = {
      minW: 4,
      minH: 3,
      settings: {
        camera: { kind: 'camera', required: true },
        seconds: { kind: 'integer', min: 5, max: 300, required: false },
        fit: { kind: 'choice', options: ['contain', 'cover'], required: false },
      },
    };
    expect(defaultSettings(camera, today)).toEqual({ seconds: 10, fit: 'contain' });
  });

  it('starts a Photos widget on the uploaded photos', () => {
    const photos: WidgetType = {
      minW: 4,
      minH: 3,
      settings: {
        seconds: { kind: 'integer', min: 5, max: 600, required: false },
        source: { kind: 'choice', options: ['uploaded', 'immich'], required: false },
      },
    };
    expect(defaultSettings(photos, today)).toEqual({ seconds: 30, source: 'uploaded' });
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

describe('paletteGroups', () => {
  const types = Object.keys(WIDGETS);
  const labels = (query: string, from = types) => paletteGroups(from, query).map((g) => [g.label, g.items.map((i) => i.label)]);

  it('lists every type under its category, in order', () => {
    const groups = paletteGroups(types);
    expect(groups.map((g) => g.label)).toEqual(['Time & date', 'Weather & nature', 'Home', 'Agenda & household', 'Travel', 'Media & news']);
    expect(groups[0]!.items.map((i) => i.type)).toEqual(['clock', 'countdown', 'holidays']);
    expect(groups.flatMap((g) => g.items)).toHaveLength(types.length);
  });

  it('finds a widget by every word of its name or description, ignoring case', () => {
    expect(labels('CLOCK')).toEqual([['Time & date', ['Clock']]]);
    expect(labels('  pollen  ')).toEqual([['Weather & nature', ['Air quality']]]);
    expect(labels('next')).toEqual([
      ['Agenda & household', ['Calendar', 'Waste collection']],
      ['Travel', ['Departures']],
    ]);
    expect(labels('the next')).toEqual([['Agenda & household', ['Waste collection']]]);
    expect(labels('nothing like this')).toEqual([]);
  });

  it('leaves out types the server does not have and lists unknown ones under Other', () => {
    expect(labels('', ['clock', 'aquarium'])).toEqual([
      ['Time & date', ['Clock']],
      ['Other', ['aquarium']],
    ]);
  });
});

describe('settingLabel', () => {
  it('words a count after what the widget shows', () => {
    expect(settingLabel('news', 'maxItems')).toBe('Headlines at most');
    expect(settingLabel('calendar', 'maxItems')).toBe('Appointments at most');
    expect(settingLabel('forecast', 'days')).toBe('Days ahead');
    expect(settingLabel('camera', 'seconds')).toBe('Refresh every (seconds)');
    expect(settingLabel('photos', 'seconds')).toBe('Seconds per photo');
  });

  it('falls back to the general label, then to the key', () => {
    expect(settingLabel('aquarium', 'maxItems')).toBe('How many at most');
    expect(settingLabel('fuel', 'radius')).toBe('Radius (km)');
    expect(settingLabel('aquarium', 'depth')).toBe('depth');
  });
});
