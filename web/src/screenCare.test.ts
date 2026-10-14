import { describe, expect, it } from 'vitest';
import { DRIFT_PX, driftAt, msToNextRefresh } from './screenCare';

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const start = Date.UTC(2026, 9, 14, 20, 0, 0);

describe('driftAt', () => {
  const day = Array.from({ length: 24 * 60 }, (_, i) => driftAt(start + i * MINUTE));

  it('stays within the drift either way', () => {
    for (const [x, y] of day) {
      expect(Math.abs(x)).toBeLessThanOrEqual(DRIFT_PX);
      expect(Math.abs(y)).toBeLessThanOrEqual(DRIFT_PX);
    }
  });

  it('moves a little each minute, never a jump', () => {
    for (let i = 1; i < day.length; i++) {
      const [[x0, y0], [x1, y1]] = [day[i - 1]!, day[i]!];
      expect(Math.hypot(x1 - x0, y1 - y0)).toBeLessThan(3);
    }
  });

  it('reaches every corner of its square in a day, so no edge stays on the same pixels', () => {
    const near = DRIFT_PX * 0.8;
    for (const [sx, sy] of [[1, 1], [1, -1], [-1, 1], [-1, -1]]) {
      expect(day.some(([x, y]) => x * sx! > near && y * sy! > near), `corner ${sx},${sy}`).toBe(true);
    }
  });

  it('is the same on every screen at the same time', () => {
    expect(driftAt(start + 5 * MINUTE)).toEqual(driftAt(start + 5 * MINUTE));
  });
});

describe('msToNextRefresh', () => {
  it('refreshes half a minute past every hour', () => {
    expect(msToNextRefresh(start)).toBe(30_000);
    expect(msToNextRefresh(start + 30_000)).toBe(HOUR);
    expect(msToNextRefresh(start + 10 * MINUTE)).toBe(HOUR - 10 * MINUTE + 30_000);
  });

  it('does not refresh twice when a timer fires a little early', () => {
    expect(msToNextRefresh(start + 30_000 - 5)).toBe(HOUR + 5);
  });

  it('is never more than an hour and a second away', () => {
    for (let t = start; t < start + 2 * HOUR; t += 7_919) {
      const left = msToNextRefresh(t);
      expect(left).toBeGreaterThanOrEqual(1000);
      expect(left).toBeLessThanOrEqual(HOUR + 1000);
    }
  });
});
