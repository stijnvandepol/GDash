import { describe, expect, it } from 'vitest';
import { daysUntil, parseLocalDate } from './dates';

describe('parseLocalDate', () => {
  it('reads a date as local midnight', () => {
    const d = parseLocalDate('2026-10-09');
    expect(d?.getFullYear()).toBe(2026);
    expect(d?.getMonth()).toBe(9);
    expect(d?.getDate()).toBe(9);
    expect(d?.getHours()).toBe(0);
  });

  it.each(['', '24-12-2026', '2026-1-5', '2026-02-30', '2026-13-01', 'tomorrow'])('rejects %j', (text) => {
    expect(parseLocalDate(text)).toBeNull();
  });
});

describe('daysUntil', () => {
  const evening = new Date(2026, 9, 9, 23, 30);

  it('counts calendar days, not 24-hour periods', () => {
    expect(daysUntil('2026-10-09', evening)).toBe(0);
    expect(daysUntil('2026-10-10', evening)).toBe(1);
    expect(daysUntil('2026-10-08', evening)).toBe(-1);
  });

  it('is not thrown off by daylight saving changes', () => {
    // In Amsterdam 29 March 2026 had 23 hours: two days are then 47 hours,
    // which a division without rounding counts as one.
    expect(daysUntil('2026-03-30', new Date(2026, 2, 28, 12, 0))).toBe(2);
    // 25 October 2026 had 25 hours.
    expect(daysUntil('2026-10-26', new Date(2026, 9, 24, 12, 0))).toBe(2);
    expect(daysUntil('2026-12-24', new Date(2026, 9, 24, 12, 0))).toBe(61);
  });

  it('returns null for an invalid date', () => {
    expect(daysUntil('2026-02-30', evening)).toBeNull();
  });
});
