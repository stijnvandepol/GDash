const HOUR_MS = 3_600_000;
const DAY_MS = 24 * HOUR_MS;

/** One hour of the estimate as the server sends it (internal/solar). */
export interface SolarHour {
  /** The start of the hour, an RFC 3339 instant written in the location's UTC offset. */
  start: string;
  kwh: number;
}

/** The expected production of one local day. */
export interface SolarDay {
  date: string;
  totalKWh: number;
  /** From the first to the last hour with production. */
  hours: SolarHour[];
}

export interface SolarReport {
  utcOffsetSeconds: number;
  days: SolarDay[];
}

/** One bar of the chart. */
export interface SolarBar {
  /** Height from 0 to 1 of the tallest bar's room. */
  height: number;
  now: boolean;
}

/** What the widget shows at a moment. */
export interface SolarView {
  todayKWh: number;
  /** The start of today's best hour, such as "13:00", or null on a day without sun. */
  peak: string | null;
  /** Tomorrow's total, or null when the report does not reach it. */
  tomorrowKWh: number | null;
  /** The hours of today still to come, or tomorrow's once today's sun is over; null when neither has any. */
  chart: { tomorrow: boolean; bars: SolarBar[]; from: string; to: string } | null;
}

/** A bar never vanishes: an hour with a little sun still produces. */
const MIN_BAR = 0.04;

function isHour(v: unknown): v is SolarHour {
  if (typeof v !== 'object' || v === null) return false;
  const { start, kwh } = v as Record<string, unknown>;
  return typeof start === 'string' && Number.isFinite(Date.parse(start)) && typeof kwh === 'number' && Number.isFinite(kwh);
}

/** The estimate in a data entry, skipping what is not a day; undefined when it is no estimate at all. */
export function readSolar(value: unknown): SolarReport | undefined {
  if (typeof value !== 'object' || value === null) return undefined;
  const { utcOffsetSeconds, days } = value as Record<string, unknown>;
  if (typeof utcOffsetSeconds !== 'number' || !Array.isArray(days)) return undefined;
  const read: SolarDay[] = [];
  for (const d of days as unknown[]) {
    if (typeof d !== 'object' || d === null) continue;
    const { date, totalKWh, hours } = d as Record<string, unknown>;
    if (typeof date !== 'string' || typeof totalKWh !== 'number' || !Number.isFinite(totalKWh) || !Array.isArray(hours)) continue;
    read.push({ date, totalKWh, hours: hours.filter(isHour) });
  }
  return { utcOffsetSeconds, days: read };
}

/** The local date at the panels, such as "2026-10-13", at nowMs. */
export function localDate(nowMs: number, utcOffsetSeconds: number): string {
  return new Date(nowMs + utcOffsetSeconds * 1000).toISOString().slice(0, 10);
}

/**
 * Today's total and best hour, tomorrow's total and the hours to chart.
 * Today is the day of the panels' own clock, found by date and not by
 * place in the list, since the last good report may be a day old.
 * Undefined when the report has no today.
 */
export function solarView(report: SolarReport, nowMs: number): SolarView | undefined {
  const date = localDate(nowMs, report.utcOffsetSeconds);
  const today = report.days.find((d) => d.date === date);
  if (!today) return undefined;
  const tomorrow = report.days.find((d) => d.date === localDate(nowMs + DAY_MS, report.utcOffsetSeconds));
  const rest = today.hours.filter((h) => Date.parse(h.start) + HOUR_MS > nowMs && h.kwh > 0);
  return {
    todayKWh: today.totalKWh,
    peak: peakHour(today.hours),
    tomorrowKWh: tomorrow?.totalKWh ?? null,
    chart: rest.length > 0 ? chart(rest, false, nowMs) : tomorrow && tomorrow.hours.length > 0 ? chart(tomorrow.hours, true, nowMs) : null,
  };
}

/** The start of the hour with the most energy, as "13:00"; the earliest of equals. */
export function peakHour(hours: SolarHour[]): string | null {
  let best: SolarHour | undefined;
  for (const h of hours) if (h.kwh > 0 && (!best || h.kwh > best.kwh)) best = h;
  return best ? best.start.slice(11, 16) : null;
}

function chart(hours: SolarHour[], tomorrow: boolean, nowMs: number): NonNullable<SolarView['chart']> {
  const top = Math.max(...hours.map((h) => h.kwh));
  const last = hours[hours.length - 1]!;
  return {
    tomorrow,
    bars: hours.map((h) => {
      const start = Date.parse(h.start);
      return { height: top > 0 ? Math.max(MIN_BAR, h.kwh / top) : MIN_BAR, now: start <= nowMs && nowMs < start + HOUR_MS };
    }),
    from: hours[0]!.start.slice(11, 16),
    // The chart ends when the last hour does.
    to: `${String((Number(last.start.slice(11, 13)) + 1) % 24).padStart(2, '0')}:${last.start.slice(14, 16)}`,
  };
}

/** "14" from 10 kWh on, "2.6" or "2,6" below: an estimate has no more precision than that. */
export function formatKWh(kwh: number, locale: string): string {
  const digits = kwh >= 10 ? 0 : 1;
  return new Intl.NumberFormat(locale, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(kwh);
}
