import { describe, expect, it } from 'vitest';
import { daylightChange, daylightMinutes, formatClock, pickDays, splitMinutes, sunDays, type SunDay } from './sun';

const day = (date: string, sunrise: string, sunset: string): SunDay => ({ date, sunrise, sunset });
const yesterday = day('2026-10-08', '2026-10-08T07:48', '2026-10-08T18:58');
const today = day('2026-10-09', '2026-10-09T07:50', '2026-10-09T18:56');
const noon = new Date(2026, 9, 9, 12, 0);

describe('sunDays', () => {
  it('keeps the complete days', () => {
    expect(sunDays({ sun: [today, { date: '2026-10-10' }, null, 'x'] })).toEqual([today]);
  });

  it.each([undefined, null, 5, {}, { sun: null }, { sun: 'x' }])('returns nothing for %j', (report) => {
    expect(sunDays(report)).toEqual([]);
  });
});

describe('pickDays', () => {
  it('finds today and yesterday by date', () => {
    const tomorrow = day('2026-10-10', '2026-10-10T07:51', '2026-10-10T18:53');
    expect(pickDays([yesterday, today, tomorrow], noon)).toEqual({ today, yesterday });
  });

  it('does not take the first entry of an old report for today', () => {
    expect(pickDays([yesterday], noon)).toEqual({ today: undefined, yesterday });
    expect(pickDays([yesterday], new Date(2026, 9, 12, 12, 0))).toEqual({ today: undefined, yesterday: undefined });
  });
});

describe('formatClock', () => {
  it('shows the wall-clock time in the language of the screen', () => {
    expect(formatClock('2026-10-09T07:50', 'en-GB')).toBe('07:50');
    expect(formatClock('2026-10-09T18:56', 'nl-NL')).toBe('18:56');
  });

  it('is not moved by a daylight saving change', () => {
    // 29 March 2026 02:30 does not exist in Amsterdam; as a wall time it is still 02:30.
    expect(formatClock('2026-03-29T02:30', 'en-GB')).toBe('02:30');
  });

  it.each(['', 'sunrise', '2026-10-09'])('returns null for %j', (text) => {
    expect(formatClock(text, 'en-GB')).toBeNull();
  });
});

describe('daylightMinutes', () => {
  it('counts the minutes between sunrise and sunset', () => {
    expect(daylightMinutes(today)).toBe(666);
    expect(daylightMinutes(yesterday)).toBe(670);
  });

  it('is null without a sunrise, a sunset or a day', () => {
    expect(daylightMinutes(day('2026-12-20', '', ''))).toBeNull();
    expect(daylightMinutes(day('2026-06-20', '2026-06-20T04:00', ''))).toBeNull();
    expect(daylightMinutes(day('2026-06-20', '2026-06-20T22:00', '2026-06-20T04:00'))).toBeNull();
    expect(daylightMinutes(undefined)).toBeNull();
  });
});

describe('splitMinutes', () => {
  it('splits into hours and minutes', () => {
    expect(splitMinutes(666)).toEqual({ hours: 11, minutes: 6 });
    expect(splitMinutes(60)).toEqual({ hours: 1, minutes: 0 });
    expect(splitMinutes(0)).toEqual({ hours: 0, minutes: 0 });
  });
});

describe('daylightChange', () => {
  it('says shorter when the days get shorter', () => {
    expect(daylightChange(today, yesterday)).toEqual({ kind: 'shorter', minutes: 4 });
  });

  it('says longer when the days get longer', () => {
    expect(daylightChange(yesterday, today)).toEqual({ kind: 'longer', minutes: 4 });
  });

  it('says same when nothing changed', () => {
    expect(daylightChange(today, { ...today, date: '2026-10-08' })).toEqual({ kind: 'same' });
  });

  it('has nothing to say without both days', () => {
    expect(daylightChange(today, undefined)).toBeNull();
    expect(daylightChange(undefined, yesterday)).toBeNull();
    expect(daylightChange(today, day('2026-10-08', '', ''))).toBeNull();
  });
});
