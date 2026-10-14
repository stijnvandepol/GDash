import { describe, expect, it } from 'vitest';
import { WASTE_COLOR, WASTE_KINDS, nextPickups, pickupDay, readWaste, type NextPickup, type Pickup } from './waste';
import { CALENDAR_COLORS } from '../colors';

// Monday 12 October 2026, late in the evening: the day must not shift.
const now = new Date(2026, 9, 12, 23, 40);

const pickup = (kind: Pickup['kind'], date: string, title = kind.toUpperCase()): Pickup => ({ kind, date, title });

describe('readWaste', () => {
  const value = { '3': [pickup('gft', '2026-10-13'), { kind: 'plasma', title: 'x', date: '2026-10-14' }, 'junk'], '4': [] };

  it('reads the pickups of the chosen calendar and skips what it does not know', () => {
    expect(readWaste(value, 3)).toEqual([pickup('gft', '2026-10-13')]);
    expect(readWaste(value, 4)).toEqual([]);
  });

  it('says missing for a calendar the data does not hold', () => {
    expect(readWaste(value, 9)).toBe('missing');
    expect(readWaste(value, '3')).toBe('missing');
  });

  it('reads the only waste calendar for a widget without one, and asks for a choice when there is none', () => {
    expect(readWaste({ ...value, default: [pickup('pmd', '2026-10-16')] }, undefined)).toEqual([pickup('pmd', '2026-10-16')]);
    expect(readWaste(value, undefined)).toBe('choose');
    expect(readWaste({}, undefined)).toBe('choose');
  });

  it('is undefined while there is no usable data', () => {
    expect(readWaste(undefined, 3)).toBeUndefined();
    expect(readWaste(null, 3)).toBeUndefined();
    expect(readWaste([], 3)).toBeUndefined();
    expect(readWaste({ '3': 'soon' }, 3)).toBeUndefined();
  });
});

describe('nextPickups', () => {
  it('keeps the next pickup of each kind, soonest first', () => {
    const list = [
      pickup('gft', '2026-10-13'),
      pickup('rest', '2026-10-15'),
      pickup('gft', '2026-10-20'),
      pickup('pmd', '2026-10-14'),
      pickup('rest', '2026-10-29'),
    ];
    expect(nextPickups(list, now).map((p) => `${p.kind} ${p.days}`)).toEqual(['gft 1', 'pmd 2', 'rest 3']);
  });

  it('looks 14 days ahead, today included', () => {
    const list = [pickup('gft', '2026-10-12'), pickup('pmd', '2026-10-26'), pickup('paper', '2026-10-27')];
    expect(nextPickups(list, now).map((p) => p.kind)).toEqual(['gft', 'pmd']);
  });

  it('skips days that have passed, as the last good data can be old', () => {
    const list = [pickup('gft', '2026-10-09'), pickup('gft', '2026-10-16'), pickup('rest', 'soon')];
    expect(nextPickups(list, now).map((p) => `${p.kind} ${p.days}`)).toEqual(['gft 4']);
  });

  it('shows unknown titles as they are, once per title', () => {
    const list = [
      pickup('other', '2026-10-15', 'Grofvuil'),
      pickup('other', '2026-10-16', 'Kerstbomen'),
      pickup('other', '2026-10-22', 'grofvuil'),
    ];
    expect(nextPickups(list, now).map((p) => p.title)).toEqual(['Grofvuil', 'Kerstbomen']);
  });

  it('orders kinds on the same day the same way every time', () => {
    const list = [pickup('textile', '2026-10-13'), pickup('gft', '2026-10-13'), pickup('glass', '2026-10-13')];
    expect(nextPickups(list, now).map((p) => p.kind)).toEqual(['gft', 'glass', 'textile']);
  });

  it('is empty without pickups', () => {
    expect(nextPickups([], now)).toEqual([]);
  });
});

describe('pickupDay', () => {
  const words = { today: 'Today', tomorrow: 'Tomorrow' };
  const at = (days: number, date: string): NextPickup => ({ kind: 'gft', title: 'GFT', date, days });

  it('says today and tomorrow in words', () => {
    expect(pickupDay(at(0, '2026-10-12'), words, 'en-GB')).toBe('Today');
    expect(pickupDay(at(1, '2026-10-13'), words, 'en-GB')).toBe('Tomorrow');
  });

  it('names the weekday within the week', () => {
    expect(pickupDay(at(3, '2026-10-15'), words, 'en-GB')).toBe('Thursday');
    expect(pickupDay(at(3, '2026-10-15'), words, 'nl-NL')).toBe('donderdag');
  });

  it('adds the date from a week on', () => {
    expect(pickupDay(at(10, '2026-10-22'), words, 'en-GB')).toMatch(/^Thu,? 22 Oct$/);
    expect(pickupDay(at(10, '2026-10-22'), words, 'nl-NL')).toMatch(/^do 22 okt\.?$/);
  });
});

describe('WASTE_COLOR', () => {
  it('names a kind of every kind, in colours that exist', () => {
    expect(Object.keys(WASTE_COLOR).sort()).toEqual([...WASTE_KINDS].sort());
    for (const name of Object.values(WASTE_COLOR)) {
      if (name !== undefined) expect(CALENDAR_COLORS).toHaveProperty(name);
    }
  });
});
