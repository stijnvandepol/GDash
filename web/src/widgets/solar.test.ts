import { describe, expect, it } from 'vitest';
import { formatKWh, localDate, peakHour, readSolar, solarView, type SolarReport } from './solar';

// Today in Oss with sun from 08:00 to 18:00 (+02:00), and tomorrow.
const REPORT: SolarReport = {
  utcOffsetSeconds: 7200,
  days: [
    {
      date: '2026-10-13',
      totalKWh: 14.2,
      hours: [
        { start: '2026-10-13T08:00:00+02:00', kwh: 0.2 },
        { start: '2026-10-13T12:00:00+02:00', kwh: 2.4 },
        { start: '2026-10-13T13:00:00+02:00', kwh: 2.6 },
        { start: '2026-10-13T14:00:00+02:00', kwh: 2.6 },
        { start: '2026-10-13T17:00:00+02:00', kwh: 0.4 },
      ],
    },
    {
      date: '2026-10-14',
      totalKWh: 9.81,
      hours: [
        { start: '2026-10-14T09:00:00+02:00', kwh: 1 },
        { start: '2026-10-14T10:00:00+02:00', kwh: 2 },
      ],
    },
  ],
};

const at = (iso: string) => Date.parse(iso);

describe('readSolar', () => {
  it('reads a report and skips what is not a day or an hour', () => {
    const read = readSolar({
      utcOffsetSeconds: 3600,
      days: [{ date: '2026-01-05', totalKWh: 1, hours: [{ start: '2026-01-05T09:00:00+01:00', kwh: 1 }, { start: 'noon', kwh: 1 }] }, { date: 5 }, null],
    });
    expect(read).toEqual({ utcOffsetSeconds: 3600, days: [{ date: '2026-01-05', totalKWh: 1, hours: [{ start: '2026-01-05T09:00:00+01:00', kwh: 1 }] }] });
  });

  it('is undefined for what is no report', () => {
    expect(readSolar(undefined)).toBeUndefined();
    expect(readSolar({ days: [] })).toBeUndefined();
    expect(readSolar('14 kWh')).toBeUndefined();
  });
});

describe('localDate', () => {
  it('is the date at the panels, not on the screen', () => {
    // 23:30 UTC is already the next day in Amsterdam in summer.
    expect(localDate(at('2026-07-01T23:30:00Z'), 7200)).toBe('2026-07-02');
    expect(localDate(at('2026-07-01T23:30:00Z'), -3600)).toBe('2026-07-01');
  });
});

describe('solarView', () => {
  it('shows today, its best hour, tomorrow and the hours still to come', () => {
    const view = solarView(REPORT, at('2026-10-13T12:30:00+02:00'))!;
    expect(view.todayKWh).toBe(14.2);
    expect(view.peak).toBe('13:00');
    expect(view.tomorrowKWh).toBe(9.81);
    expect(view.chart?.tomorrow).toBe(false);
    // The hour from 12:00 is under way, so it is still to come.
    expect(view.chart?.bars.map((b) => b.now)).toEqual([true, false, false, false]);
    expect(view.chart?.bars.map((b) => Math.round(b.height * 100))).toEqual([92, 100, 100, 15]);
    expect(view.chart?.from).toBe('12:00');
    expect(view.chart?.to).toBe('18:00');
  });

  it('charts tomorrow once the sun is down today', () => {
    const view = solarView(REPORT, at('2026-10-13T19:00:00+02:00'))!;
    expect(view.todayKWh).toBe(14.2);
    expect(view.chart).toEqual({ tomorrow: true, bars: [{ height: 0.5, now: false }, { height: 1, now: false }], from: '09:00', to: '11:00' });
  });

  it('takes tomorrow as today after midnight, from the last good report', () => {
    const view = solarView(REPORT, at('2026-10-14T07:00:00+02:00'))!;
    expect(view.todayKWh).toBe(9.81);
    expect(view.tomorrowKWh).toBeNull();
    expect(view.chart?.from).toBe('09:00');
  });

  it('is undefined when the report has no today', () => {
    expect(solarView(REPORT, at('2026-10-16T12:00:00+02:00'))).toBeUndefined();
  });

  it('has no peak and no chart on a day without sun', () => {
    const dark: SolarReport = { utcOffsetSeconds: 0, days: [{ date: '2026-12-21', totalKWh: 0, hours: [] }] };
    expect(solarView(dark, at('2026-12-21T12:00:00Z'))).toEqual({ todayKWh: 0, peak: null, tomorrowKWh: null, chart: null });
  });
});

describe('peakHour', () => {
  it('takes the earliest of equal hours', () => {
    expect(peakHour(REPORT.days[0]!.hours)).toBe('13:00');
    expect(peakHour([])).toBeNull();
  });
});

describe('formatKWh', () => {
  it('rounds to whole kWh from 10 on and keeps one decimal below, in the language', () => {
    expect(formatKWh(14.2, 'en-GB')).toBe('14');
    expect(formatKWh(2.64, 'en-GB')).toBe('2.6');
    expect(formatKWh(2.64, 'nl-NL')).toBe('2,6');
    expect(formatKWh(0, 'de-DE')).toBe('0,0');
  });
});
