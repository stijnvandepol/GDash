import { describe, expect, it } from 'vitest';
import { agoText, areaName, coveredHours, feedKeys, readValue, rowsFor, type Alert } from './p2000';

const text = (key: string, values?: Record<string, string | number>) => `${key}${values ? JSON.stringify(values) : ''}`;
const now = new Date('2026-10-16T12:00:00Z');

function alert(id: string, minutesAgo: number, place = 'nuenen', discipline = 'brandweer'): Alert {
  return { id, at: new Date(now.getTime() - minutesAgo * 60_000).toISOString(), discipline, place, text: `Alert ${id}` };
}

describe('feedKeys', () => {
  it('takes the region with every chosen discipline, or the regions of the places', () => {
    expect(feedKeys({ region: 'twente', disciplines: ['politie'] })).toEqual(['twente/politie']);
    expect(feedKeys({ region: 'nowhere' })).toEqual([]);
    const places = [
      { name: 'Nuenen', slug: 'nuenen', region: 'brabant-zuidoost' },
      { name: 'Geldrop', slug: 'geldrop', region: 'brabant-zuidoost' },
    ];
    // Without disciplines: all but ambulance, as the server does.
    expect(feedKeys({ places })).toEqual(['brabant-zuidoost/brandweer', 'brabant-zuidoost/politie', 'brabant-zuidoost/knrm', 'brabant-zuidoost/trauma']);
  });
});

describe('rowsFor', () => {
  const settings = { region: 'brabant-zuidoost', disciplines: ['brandweer', 'politie'], hours: 2, maxItems: 3 };

  it('merges the feeds newest first, each alert once, within the hours', () => {
    const value = readValue({
      feeds: {
        'brabant-zuidoost/brandweer': [alert('a', 5), alert('b', 100), alert('old', 121)],
        'brabant-zuidoost/politie': [alert('c', 30, 'eindhoven', 'politie'), alert('a', 5)],
      },
      skipped: [],
    })!;
    expect(rowsFor(value, settings, now)).toEqual([
      { id: 'a', discipline: 'brandweer', text: 'Alert a', minutes: 5 },
      { id: 'c', discipline: 'politie', text: 'Alert c', minutes: 30 },
      { id: 'b', discipline: 'brandweer', text: 'Alert b', minutes: 100 },
    ]);
  });

  it('keeps only the places of a widget with places', () => {
    const value = readValue({ feeds: { 'brabant-zuidoost/brandweer': [alert('a', 5), alert('b', 6, 'eindhoven')] }, skipped: [] })!;
    const places = { places: [{ name: 'Nuenen', slug: 'nuenen', region: 'brabant-zuidoost' }], disciplines: ['brandweer'] };
    expect(rowsFor(value, places, now)).toEqual([{ id: 'a', discipline: 'brandweer', text: 'Alert a', minutes: 5 }]);
  });

  it('waits for every feed, and says when the server skips one', () => {
    const waiting = readValue({ feeds: { 'brabant-zuidoost/brandweer': [] }, skipped: [] })!;
    expect(rowsFor(waiting, settings, now)).toBeUndefined();
    const skipped = readValue({ feeds: { 'brabant-zuidoost/brandweer': [] }, skipped: ['brabant-zuidoost/politie'] })!;
    expect(rowsFor(skipped, settings, now)).toBe('skipped');
  });

  it('ignores what is broken', () => {
    expect(readValue([])).toBeUndefined();
    expect(readValue({ feeds: { 'twente/politie': [{ id: 1 }, alert('a', 1)] } })).toEqual({ feeds: { 'twente/politie': [alert('a', 1)] }, skipped: [] });
  });
});

describe('coveredHours', () => {
  const settings = { region: 'amsterdam-amstelland', disciplines: ['ambulance'], hours: 48 };

  it('claims no more hours than a full feed holds', () => {
    // 100 rides, one every 10 minutes: the oldest is 16.5 hours back.
    const full = Array.from({ length: 100 }, (_, i) => alert(String(i), i * 10, 'amsterdam', 'ambulance'));
    expect(coveredHours({ feeds: { 'amsterdam-amstelland/ambulance': full }, skipped: [] }, settings, now)).toBe(16);
    // A feed that is not full holds everything there was.
    expect(coveredHours({ feeds: { 'amsterdam-amstelland/ambulance': full.slice(0, 99) }, skipped: [] }, settings, now)).toBe(48);
  });
});

describe('texts', () => {
  it('says how long ago', () => {
    expect(agoText(0, text)).toBe('p2000Now');
    expect(agoText(12, text)).toBe('p2000Minutes{"minutes":12}');
    expect(agoText(185, text)).toBe('p2000Hours{"hours":3}');
  });

  it('names the region or the places', () => {
    expect(areaName({ region: 'limburg-zuid' })).toBe('Zuid-Limburg');
    expect(areaName({ places: [{ name: 'Nuenen', slug: 'nuenen', region: 'brabant-zuidoost' }, { name: 'Geldrop', slug: 'geldrop', region: 'brabant-zuidoost' }] })).toBe(
      'Nuenen, Geldrop',
    );
  });
});
