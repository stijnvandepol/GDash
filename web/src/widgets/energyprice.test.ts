import { describe, expect, it } from 'vitest';
import { clockTime, formatPrice, priceLevel, priceView, readPrices, type PriceDay } from './energyprice';

/** A day of 24 hours on 12 October 2026 in Dutch summer time, from the given prices. */
function day(date: string, prices: number[], cheapest: [string, string] | null = null): PriceDay {
  return {
    date,
    hours: prices.map((price, h) => ({ start: `${date}T${String(h).padStart(2, '0')}:00:00+02:00`, price })),
    cheapest: cheapest ? { start: `${date}T${cheapest[0]}:00+02:00`, end: `${date}T${cheapest[1]}:00+02:00` } : null,
  };
}

// The recorded prices of 12 October 2026 (internal/energyprice/testdata/today.json).
const recorded = [
  0.22, 0.21, 0.21, 0.21, 0.21, 0.22, 0.27, 0.31, 0.31, 0.27, 0.23, 0.21, 0.18, 0.17, 0.18, 0.19, 0.22, 0.28, 0.33, 0.34,
  0.3, 0.24, 0.23, 0.22,
];
const today = day('2026-10-12', recorded, ['12:00', '15:00']);

/** An instant on 12 October 2026 at a Dutch wall-clock time. */
const at = (time: string) => Date.parse(`2026-10-12T${time}:00+02:00`);

describe('priceLevel', () => {
  const prices = [0.1, 0.2, 0.3, 0.4];

  it('splits the day into thirds between its cheapest and dearest hour', () => {
    expect(priceLevel(0.1, prices)).toBe('low');
    expect(priceLevel(0.19, prices)).toBe('low');
    expect(priceLevel(0.2, prices)).toBe('average');
    expect(priceLevel(0.3, prices)).toBe('average');
    expect(priceLevel(0.31, prices)).toBe('high');
    expect(priceLevel(0.4, prices)).toBe('high');
  });

  it('calls a flat day average', () => {
    expect(priceLevel(0.2, [0.2, 0.2, 0.2])).toBe('average');
  });

  it('works with negative prices', () => {
    expect(priceLevel(-0.05, [-0.05, 0.1, 0.3])).toBe('low');
  });
});

describe('priceView', () => {
  it('gives the price of the current hour, whatever the viewer\'s time zone', () => {
    // 11:48 in the Netherlands is 09:48 UTC; the hour is found by instant.
    expect(priceView([today], Date.parse('2026-10-12T09:48:00Z'))?.price).toBe(0.21);
    expect(priceView([today], at('00:00'))?.price).toBe(0.22);
    expect(priceView([today], at('23:59'))?.price).toBe(0.22);
  });

  it('compares the price with the rest of the day', () => {
    expect(priceView([today], at('12:30'))?.level).toBe('low');
    expect(priceView([today], at('06:30'))?.level).toBe('average');
    expect(priceView([today], at('18:30'))?.level).toBe('high');
  });

  it('draws a bar per hour and marks the current hour and the cheapest stretch', () => {
    const view = priceView([today], at('13:20'));
    expect(view?.bars).toHaveLength(24);
    expect(view?.bars.filter((b) => b.now).map((b) => b.hour)).toEqual(['13']);
    expect(view?.bars.filter((b) => b.cheap).map((b) => b.hour)).toEqual(['12', '13', '14']);
    // The dearest hour has the tallest bar; every bar stays visible.
    const heights = view?.bars.map((b) => b.height) ?? [];
    expect(Math.max(...heights)).toBe(1);
    expect(Math.min(...heights)).toBeGreaterThan(0);
    expect(heights[19]).toBe(1);
  });

  it('names the cheapest stretch while it has not ended', () => {
    expect(priceView([today], at('09:00'))?.cheapest).toEqual({ from: '12:00', to: '15:00', tomorrow: false });
    expect(priceView([today], at('13:00'))?.cheapest).toEqual({ from: '12:00', to: '15:00', tomorrow: false });
  });

  it('moves on to tomorrow once the stretch is over, and to nothing without tomorrow', () => {
    const tomorrow = day('2026-10-13', recorded.map((p) => p + 0.01), ['13:00', '16:00']);
    expect(priceView([today, tomorrow], at('16:00'))?.cheapest).toEqual({ from: '13:00', to: '16:00', tomorrow: true });
    expect(priceView([today], at('16:00'))?.cheapest).toBeNull();
  });

  it('shows the day of the current hour, not the first in the list', () => {
    const yesterday = day('2026-10-11', Array(24).fill(0.5));
    const view = priceView([yesterday, today], at('12:10'));
    expect(view?.price).toBe(0.18);
    expect(view?.cheapest?.from).toBe('12:00');
  });

  it('is undefined when the list has no price for this hour', () => {
    expect(priceView([today], Date.parse('2026-10-13T00:30:00+02:00'))).toBeUndefined();
    expect(priceView([today], Date.parse('2026-10-11T23:59:00+02:00'))).toBeUndefined();
    expect(priceView([], at('12:00'))).toBeUndefined();
  });

  it('gives a day without any price bars of half height, not a division by zero', () => {
    const free = day('2026-10-12', Array(24).fill(0));
    expect(new Set(priceView([free], at('10:00'))?.bars.map((b) => b.height))).toEqual(new Set([0.5]));
  });

  it('starts the bars below zero when the price is negative', () => {
    const view = priceView([day('2026-10-12', [0.2, -0.1, 0.1, ...Array(21).fill(0.1)])], at('01:00'));
    expect(view?.price).toBe(-0.1);
    expect(view?.level).toBe('low');
    const [first, second] = view?.bars ?? [];
    expect(first?.height).toBe(1);
    expect(second?.height).toBe(0.06);
  });
});

describe('readPrices', () => {
  it('reads the days of a report and drops what is broken', () => {
    const report = {
      days: [
        { date: '2026-10-12', hours: [{ start: '2026-10-12T00:00:00+02:00', price: 0.2 }, { start: 'later', price: 0.1 }, { price: 1 }], cheapest: { start: 'x', end: 'y' } },
        { date: '2026-10-13', hours: [] },
        'junk',
      ],
    };
    expect(readPrices(report)).toEqual([
      { date: '2026-10-12', hours: [{ start: '2026-10-12T00:00:00+02:00', price: 0.2 }], cheapest: null },
    ]);
  });

  it.each([undefined, null, 'x', {}, { days: 'x' }])('reads %j as no days', (report) => {
    expect(readPrices(report)).toEqual([]);
  });
});

describe('formatPrice', () => {
  it('writes euros with the decimal mark of the language', () => {
    expect(formatPrice(0.234, 'en-GB')).toBe('€0.23');
    expect(formatPrice(0.234, 'nl-NL')).toBe('€0,23');
    expect(formatPrice(1, 'en-GB')).toBe('€1.00');
  });

  it('keeps the sign of a negative price and drops it for zero', () => {
    expect(formatPrice(-0.02, 'en-GB')).toBe('-€0.02');
    expect(formatPrice(-0.001, 'en-GB')).toBe('€0.00');
  });
});

describe('clockTime', () => {
  it('reads the wall-clock time from the text', () => {
    expect(clockTime('2026-10-12T13:00:00+02:00')).toBe('13:00');
  });
});
