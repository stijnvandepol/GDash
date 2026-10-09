import { daysUntil } from './dates';

/** Sunrise and sunset on one day, as the server sends them. */
export interface SunDay {
  /** The local date, YYYY-MM-DD. */
  date: string;
  /** Local time at the weather location, YYYY-MM-DDTHH:MM; empty when the sun does not rise. */
  sunrise: string;
  /** Local time at the weather location, YYYY-MM-DDTHH:MM; empty when the sun does not set. */
  sunset: string;
}

const MINUTE_MS = 60_000;
const WALL_TIME = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/;

/** The days of a weather report that carry sunrise and sunset. */
export function sunDays(report: unknown): SunDay[] {
  if (typeof report !== 'object' || report === null) return [];
  const list = (report as { sun?: unknown }).sun;
  if (!Array.isArray(list)) return [];
  return list.filter(
    (d): d is SunDay =>
      typeof d === 'object' && d !== null && ['date', 'sunrise', 'sunset'].every((k) => typeof (d as Record<string, unknown>)[k] === 'string'),
  );
}

/**
 * Today and yesterday out of the days a report holds. The last good report
 * may be older than today, so the days are found by their date and never by
 * their place in the list.
 */
export function pickDays(days: SunDay[], now: Date): { today?: SunDay; yesterday?: SunDay } {
  return {
    today: days.find((d) => daysUntil(d.date, now) === 0),
    yesterday: days.find((d) => daysUntil(d.date, now) === -1),
  };
}

/** A wall-clock time as UTC milliseconds, so the viewer's own time zone never shifts it. */
function wallTime(text: string): number | null {
  const m = WALL_TIME.exec(text);
  if (!m) return null;
  const [year, month, day, hour, minute] = m.slice(1).map(Number) as [number, number, number, number, number];
  return Date.UTC(year, month - 1, day, hour, minute);
}

/** The time of day in text, such as "07:50" for 2026-10-09T07:50, or null when text is not a time. */
export function formatClock(text: string, language: string): string | null {
  const ms = wallTime(text);
  if (ms === null) return null;
  return new Intl.DateTimeFormat(language, { hour: '2-digit', minute: '2-digit', timeZone: 'UTC' }).format(ms);
}

/** Minutes between sunrise and sunset, or null when either is missing or they are out of order. */
export function daylightMinutes(day: SunDay | undefined): number | null {
  if (!day) return null;
  const rise = wallTime(day.sunrise);
  const set = wallTime(day.sunset);
  if (rise === null || set === null || set <= rise) return null;
  return Math.round((set - rise) / MINUTE_MS);
}

/** Whole hours and the minutes left over. */
export function splitMinutes(total: number): { hours: number; minutes: number } {
  return { hours: Math.floor(total / 60), minutes: total % 60 };
}

/** How today's daylight compares with yesterday's, in whole minutes. */
export type DaylightChange = { kind: 'longer' | 'shorter'; minutes: number } | { kind: 'same' };

/** The change in daylight since yesterday, or null when either day has no length. */
export function daylightChange(today: SunDay | undefined, yesterday: SunDay | undefined): DaylightChange | null {
  const now = daylightMinutes(today);
  const before = daylightMinutes(yesterday);
  if (now === null || before === null) return null;
  const diff = now - before;
  if (diff === 0) return { kind: 'same' };
  return { kind: diff > 0 ? 'longer' : 'shorter', minutes: Math.abs(diff) };
}
