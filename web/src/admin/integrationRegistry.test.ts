import { describe, expect, it } from 'vitest';
import type { Calendar, Settings } from './api';
import { INTEGRATIONS } from './integrationRegistry';
import type { IntegrationContext } from './integrations';

const calendar = (id: number, name: string): Calendar => ({ id, name, host: 'example.org', color: 'blue', unreadable: false });

function context(settings: Partial<Settings>): IntegrationContext {
  return {
    settings: { location: null, calendars: [], wasteCalendars: [], solar: null, knmi: { connected: false }, traffic: { enabled: false }, ...settings } as Settings,
    status: {},
    sources: [],
    photoCount: 0,
    tokenCount: 0,
    cameraCount: 0,
    webPageCount: 0,
    widgetTypes: ['waste', 'rain', 'fuel', 'solar', 'airquality', 'sun', 'warnings', 'traffic'],
  };
}

const card = (id: string) => INTEGRATIONS.find((i) => i.id === id)!;

describe('the Waste collection and Calendars cards', () => {
  it('each count only their own calendars', () => {
    const ctx = context({ calendars: [calendar(1, 'Family')], wasteCalendars: [calendar(2, 'Home')] });
    expect(card('waste').state(ctx)).toEqual({ kind: 'connected', detail: 'Home. Fetched once a dashboard shows it.' });
    expect(card('calendars').state(ctx).detail).toMatch(/^1 calendar\./);
    expect(card('waste').state(context({ calendars: [calendar(1, 'Family')] }))).toEqual({ kind: 'idle', detail: 'Not set up' });
  });

  it('opens a setup of its own for waste collection', () => {
    expect(card('waste').kind).toBe('service');
    expect(card('waste').panel).toBeDefined();
  });
});

describe('weatherLocation', () => {
  it('marks the cards that use the weather location', () => {
    const users = INTEGRATIONS.filter((i) => i.weatherLocation?.(context({}).settings)).map((i) => i.id);
    expect(users).toEqual(expect.arrayContaining(['airquality', 'sun', 'rain', 'fuel', 'warnings', 'solar', 'knmi', 'traffic', 'speedcameras']));
    expect(users).not.toContain('weather');
    expect(users).not.toContain('moon');
  });

  it('leaves solar panels with a place of their own out', () => {
    const own = context({ solar: { location: { name: 'Uden', latitude: 51.66, longitude: 5.62 } } as Settings['solar'] });
    expect(card('solar').weatherLocation?.(own.settings)).toBeNull();
  });
});
