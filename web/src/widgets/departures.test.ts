import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Snapshot } from '../protocol';
import { departureLists, minutesUntil, upcoming, whenText, type Departure } from './departures';

beforeEach(() => {
  vi.stubGlobal('navigator', { language: 'en-US' });
});

afterEach(() => {
  display.snapshot = null;
  vi.unstubAllGlobals();
});

// 11:47:50 in the Netherlands on 12 October 2026 (summer time, UTC+2).
const now = new Date('2026-10-12T11:47:50+02:00');

function departure(expected: string, extra: Partial<Departure> = {}): Departure {
  return { line: '28', destination: 'Vleuterweide', expected, ...extra };
}

describe('minutesUntil', () => {
  it('counts on the clock face, like the display at the stop', () => {
    // Due 11:48:36 and it is 11:47:50: the next minute on the clock.
    expect(minutesUntil('2026-10-12T11:48:36+02:00', now)).toBe(1);
    expect(minutesUntil('2026-10-12T11:47:59+02:00', now)).toBe(0);
    expect(minutesUntil('2026-10-12T11:54:00+02:00', now)).toBe(7);
    expect(minutesUntil('2026-10-12T11:47:00+02:00', now)).toBe(0);
    expect(minutesUntil('2026-10-12T11:46:59+02:00', now)).toBe(-1);
  });

  it('reads the instant, not the zone it is written in', () => {
    expect(minutesUntil('2026-10-12T09:54:00Z', now)).toBe(7);
  });

  it('gives up on a text that is no instant', () => {
    expect(minutesUntil('tomorrow', now)).toBeUndefined();
    expect(minutesUntil('', now)).toBeUndefined();
  });
});

describe('upcoming', () => {
  it('drops what has left, keeps the server order and stops at max', () => {
    const list = [
      departure('2026-10-12T11:40:00+02:00'), // left already
      departure('2026-10-12T11:47:30+02:00', { destination: 'Now' }),
      departure('2026-10-12T11:50:00+02:00', { destination: 'Soon' }),
      departure('2026-10-12T12:05:00+02:00', { destination: 'Later' }),
    ];
    expect(upcoming(list, now, 2).map((r) => [r.destination, r.minutes])).toEqual([
      ['Now', 0],
      ['Soon', 3],
    ]);
    expect(upcoming(list, now, 10)).toHaveLength(3);
  });

  it('shows a bus the minute it leaves and drops it a minute later', () => {
    const list = [departure('2026-10-12T11:48:00+02:00')];
    expect(upcoming(list, new Date('2026-10-12T11:48:59+02:00'), 5)).toHaveLength(1);
    expect(upcoming(list, new Date('2026-10-12T11:49:00+02:00'), 5)).toHaveLength(0);
  });

  it('carries the delay and cancellation, and ignores nonsense', () => {
    const [late, cancelled] = upcoming(
      [
        departure('2026-10-12T12:05:00+02:00', { delayMinutes: 4 }),
        departure('2026-10-12T12:10:00+02:00', { cancelled: true, delayMinutes: -3 }),
        departure('not a time'),
      ],
      now,
      5,
    );
    expect(late).toMatchObject({ delay: 4, cancelled: false });
    expect(cancelled).toMatchObject({ delay: 0, cancelled: true });
  });

  it('writes the time on a Dutch clock, whatever the zone of the screen', () => {
    // 09:48 UTC is 11:48 in the Netherlands in summer and 10:48 in winter.
    expect(upcoming([departure('2026-10-12T09:48:00Z')], now, 1)[0]?.clock).toBe('11:48');
    const winter = new Date('2026-12-01T10:00:00+01:00');
    expect(upcoming([departure('2026-12-01T09:48:00Z')], winter, 1)[0]?.clock).toBe('10:48');
  });
});

describe('whenText', () => {
  it('says now for a bus that is leaving', () => {
    expect(whenText(0)).toBe('now');
    expect(whenText(-1)).toBe('now');
    expect(whenText(3)).toBe('3 min');
  });

  it('follows the language of the screen', () => {
    display.snapshot = { language: 'nl' } as Snapshot;
    expect(whenText(0)).toBe('nu');
    expect(whenText(12)).toBe('12 min');
  });
});

describe('departureLists', () => {
  it('reads the server value by stop code', () => {
    const value = { utrcsw: [departure('2026-10-12T12:00:00+02:00', { delayMinutes: 2 })], asdcs: [] };
    expect(departureLists(value)).toEqual(value);
  });

  it('keeps only well-formed departures and ignores a broken value', () => {
    expect(departureLists({ a: [{ line: '1' }, departure('2026-10-12T12:00:00+02:00'), null, 'x'], b: 'oops' })).toEqual({
      a: [departure('2026-10-12T12:00:00+02:00')],
    });
    expect(departureLists(null)).toBeUndefined();
    expect(departureLists([1, 2])).toBeUndefined();
    expect(departureLists('text')).toBeUndefined();
  });
});
