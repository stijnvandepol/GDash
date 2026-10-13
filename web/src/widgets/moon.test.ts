import { describe, expect, it } from 'vitest';
import { FIRST_QUARTER, FULL, LAST_QUARTER, NEW, illumination, litPath, moonAt, nextPhase, phaseInstant } from './moon';

const MINUTE_MS = 60_000;
// Meeus' own phases are good to a few minutes; this leaves room for ΔT.
const TOLERANCE_MS = 10 * MINUTE_MS;

function expectNear(actual: Date, expected: string) {
  const diff = Math.abs(actual.getTime() - Date.parse(expected));
  expect(diff, `${actual.toISOString()} vs ${expected}`).toBeLessThan(TOLERANCE_MS);
}

describe('phaseInstant', () => {
  it('matches the worked example of Meeus', () => {
    // Example 49.a: the new moon of February 1977, k = -283, at 1977 Feb 18
    // 3:37:42 TD; ΔT was about 48 s then, against the 69 s used now.
    expectNear(phaseInstant(-283, NEW), '1977-02-18T03:37:00Z');
  });
});

describe('nextPhase', () => {
  // Phases of the Moon 2001 to 2100, Fred Espenak, NASA/GSFC
  // (eclipse.gsfc.nasa.gov/phase/phases2001.html), in UT.
  const nasa: [string, number, string][] = [
    ['2024-04-01T00:00:00Z', NEW, '2024-04-08T18:21:00Z'], // total solar eclipse
    ['2025-03-01T00:00:00Z', FULL, '2025-03-14T06:55:00Z'], // total lunar eclipse
    ['2026-03-01T00:00:00Z', FULL, '2026-03-03T11:38:00Z'],
    ['2026-08-01T00:00:00Z', NEW, '2026-08-12T17:37:00Z'],
    ['2026-10-13T08:00:00Z', NEW, '2026-11-09T07:02:00Z'],
    ['2026-10-13T08:00:00Z', FIRST_QUARTER, '2026-10-18T16:13:00Z'],
    ['2026-10-13T08:00:00Z', FULL, '2026-10-26T04:12:00Z'],
    ['2026-10-13T08:00:00Z', LAST_QUARTER, '2026-11-01T20:28:00Z'],
    ['2027-02-01T00:00:00Z', NEW, '2027-02-06T15:56:00Z'],
  ];

  it.each(nasa)('after %s finds phase %s at %s', (after, phase, expected) => {
    expectNear(nextPhase(new Date(after), phase as typeof NEW), expected);
  });

  it('never returns the instant itself or one before it', () => {
    const full = nextPhase(new Date('2026-10-13T08:00:00Z'), FULL);
    expect(nextPhase(full, FULL).getTime() - full.getTime()).toBeGreaterThan(29 * 86_400_000);
  });
});

describe('illumination', () => {
  it('matches the worked example of Meeus', () => {
    // Example 48.a: 1992 April 12, 0h TD, k = 0.6786; the simple formula gives 0.68.
    expect(illumination(new Date('1992-04-12T00:00:00Z'))).toBeCloseTo(0.68, 2);
  });

  it('is full at a full moon, dark at a new moon and half at a quarter', () => {
    expect(illumination(new Date('2026-10-26T04:12:00Z'))).toBeGreaterThan(0.995);
    expect(illumination(new Date('2026-10-10T15:50:00Z'))).toBeLessThan(0.005);
    expect(illumination(new Date('2026-10-18T16:13:00Z'))).toBeCloseTo(0.5, 1);
  });
});

describe('moonAt', () => {
  it('names the local day of a principal phase after it', () => {
    // The full moon is at 05:12 Amsterdam time on 26 October 2026; the
    // whole day is the day of the full moon, the evening included.
    expect(moonAt(new Date(2026, 9, 26, 1, 0)).phase).toBe('full');
    expect(moonAt(new Date(2026, 9, 26, 22, 0)).phase).toBe('full');
    expect(moonAt(new Date(2026, 9, 18, 9, 0)).phase).toBe('firstQuarter');
    expect(moonAt(new Date(2026, 10, 1, 23, 0)).phase).toBe('lastQuarter');
    expect(moonAt(new Date(2026, 9, 10, 12, 0)).phase).toBe('new');
  });

  it('calls the days between crescent or gibbous, waxing or waning', () => {
    const tuesday = moonAt(new Date(2026, 9, 13, 10, 0));
    expect(tuesday).toMatchObject({ phase: 'waxingCrescent', waxing: true });
    expect(tuesday.illumination).toBeGreaterThan(0.05);
    expect(tuesday.illumination).toBeLessThan(0.2);
    expectNear(tuesday.nextFull, '2026-10-26T04:12:00Z');
    expectNear(tuesday.nextNew, '2026-11-09T07:02:00Z');

    expect(moonAt(new Date(2026, 9, 22, 20, 0)).phase).toBe('waxingGibbous');
    expect(moonAt(new Date(2026, 9, 29, 20, 0))).toMatchObject({ phase: 'waningGibbous', waxing: false });
    expect(moonAt(new Date(2026, 10, 5, 20, 0))).toMatchObject({ phase: 'waningCrescent', waxing: false });
  });
});

describe('litPath', () => {
  it('draws a waxing crescent on the right, its edge bulging to the right', () => {
    expect(litPath(0.25, true, 50, 50, 40)).toBe('M50 10 A40 40 0 0 1 50 90 A20 40 0 0 0 50 10 Z');
  });

  it('draws a waning gibbous on the left, its edge bulging to the right', () => {
    expect(litPath(0.75, false, 50, 50, 40)).toBe('M50 10 A40 40 0 0 0 50 90 A20 40 0 0 0 50 10 Z');
  });

  it('draws a half moon with a straight edge and a full moon as a circle', () => {
    expect(litPath(0.5, true, 50, 50, 40)).toBe('M50 10 A40 40 0 0 1 50 90 A0 40 0 0 0 50 10 Z');
    expect(litPath(1, true, 50, 50, 40)).toBe('M50 10 A40 40 0 0 1 50 90 A40 40 0 0 1 50 10 Z');
    expect(litPath(1.4, true, 50, 50, 40)).toBe(litPath(1, true, 50, 50, 40));
  });
});
