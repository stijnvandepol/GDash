import type { TextKey } from '../i18n.svelte';

const HOUR_MS = 3_600_000;

/** One hour of the price list as the server sends it (internal/energyprice). */
interface PriceHour {
  /** The start of the hour, an RFC 3339 instant written in the Dutch time zone. */
  start: string;
  /** Euro per kWh including VAT; negative when the market pays to use power. */
  price: number;
}

/** The cheapest stretch of consecutive hours of a day. */
interface PriceWindow {
  start: string;
  /** When the last hour ends. */
  end: string;
}

/** The hourly prices of one Dutch day. */
export interface PriceDay {
  date: string;
  hours: PriceHour[];
  cheapest: PriceWindow | null;
}

export type PriceLevel = 'low' | 'average' | 'high';

export const LEVEL_TEXT: Record<PriceLevel, TextKey> = {
  low: 'energyPriceLow',
  average: 'energyPriceAverage',
  high: 'energyPriceHigh',
};

/** One bar of the chart. */
export interface Bar {
  /** Height from 0 to 1 of the tallest bar's room. */
  height: number;
  now: boolean;
  /** Whether the hour is in the day's cheapest stretch. */
  cheap: boolean;
  /** The hour, such as "13", for a label under the bar. */
  hour: string;
}

/** What the widget shows at a moment. */
export interface PriceView {
  price: number;
  level: PriceLevel;
  bars: Bar[];
  /** The cheapest stretch still to come, or none when it is over and tomorrow's prices are not out. */
  cheapest: { from: string; to: string; tomorrow: boolean } | null;
}

/** A bar never vanishes: the cheapest hour of the day is still an hour with a price. */
const MIN_BAR = 0.06;

function isHour(v: unknown): v is PriceHour {
  if (typeof v !== 'object' || v === null) return false;
  const { start, price } = v as Record<string, unknown>;
  return typeof start === 'string' && Number.isFinite(Date.parse(start)) && typeof price === 'number' && Number.isFinite(price);
}

function isWindow(v: unknown): v is PriceWindow {
  if (typeof v !== 'object' || v === null) return false;
  const { start, end } = v as Record<string, unknown>;
  return typeof start === 'string' && Number.isFinite(Date.parse(start)) && typeof end === 'string' && Number.isFinite(Date.parse(end));
}

/** The days of a price report, skipping what is not a day with hours. */
export function readPrices(report: unknown): PriceDay[] {
  if (typeof report !== 'object' || report === null) return [];
  const days = (report as { days?: unknown }).days;
  if (!Array.isArray(days)) return [];
  const read: PriceDay[] = [];
  for (const d of days) {
    if (typeof d !== 'object' || d === null || !Array.isArray(d.hours) || typeof d.date !== 'string') continue;
    const hours = d.hours.filter(isHour);
    if (hours.length > 0) read.push({ date: d.date, hours, cheapest: isWindow(d.cheapest) ? d.cheapest : null });
  }
  return read;
}

/**
 * Low, average or high among the day's prices: the third of the range
 * between its cheapest and dearest hour that the price falls in. A day
 * with one price all day is average.
 */
export function priceLevel(price: number, prices: number[]): PriceLevel {
  const lo = Math.min(...prices);
  const hi = Math.max(...prices);
  if (hi - lo < 1e-9) return 'average';
  const position = (price - lo) / (hi - lo);
  if (position < 1 / 3) return 'low';
  return position > 2 / 3 ? 'high' : 'average';
}

/** The clock time in an RFC 3339 text, such as "13:00" for 2026-10-12T13:00:00+02:00; the server wrote it in Dutch time. */
export function clockTime(text: string): string {
  return text.slice(11, 16);
}

/** "€0.23" or "€0,23": the decimal mark follows the language; a negative price keeps its sign in front. */
export function formatPrice(price: number, locale: string): string {
  const cents = Math.round(price * 100);
  const digits = new Intl.NumberFormat(locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(Math.abs(cents) / 100);
  // A price that rounds to zero has no sign: "-€0.00" reads as a mistake.
  return `${cents < 0 ? '-' : ''}€${digits}`;
}

/**
 * The price now, how it compares with the rest of the day, the day's bars
 * and the cheapest stretch still to come. The day is the one that holds the
 * current hour, found by time and not by place in the list, since the last
 * good report may be a day old. Undefined when the report has no price for
 * this hour.
 */
export function priceView(days: PriceDay[], nowMs: number): PriceView | undefined {
  const index = days.findIndex((d) => d.hours.some((h) => isCurrent(h, nowMs)));
  const day = days[index];
  const current = day?.hours.find((h) => isCurrent(h, nowMs));
  if (!day || !current) return undefined;

  const prices = day.hours.map((h) => h.price);
  const hi = Math.max(...prices);
  // The bars start at zero, or at the lowest price when it is below zero.
  const lo = Math.min(0, ...prices);
  const cheap = day.cheapest ? { from: Date.parse(day.cheapest.start), to: Date.parse(day.cheapest.end) } : null;
  const bars = day.hours.map((h) => {
    const start = Date.parse(h.start);
    return {
      height: hi - lo < 1e-9 ? 0.5 : Math.max(MIN_BAR, (h.price - lo) / (hi - lo)),
      now: isCurrent(h, nowMs),
      cheap: cheap !== null && start >= cheap.from && start < cheap.to,
      hour: h.start.slice(11, 13),
    };
  });
  return { price: current.price, level: priceLevel(current.price, prices), bars, cheapest: upcomingWindow(days, index, nowMs) };
}

function isCurrent(h: PriceHour, nowMs: number): boolean {
  const start = Date.parse(h.start);
  return start <= nowMs && nowMs < start + HOUR_MS;
}

/** Today's cheapest stretch while it has not ended, otherwise the next day's, otherwise none. */
function upcomingWindow(days: PriceDay[], today: number, nowMs: number): PriceView['cheapest'] {
  const todays = days[today]?.cheapest;
  if (todays && Date.parse(todays.end) > nowMs) return { from: clockTime(todays.start), to: clockTime(todays.end), tomorrow: false };
  const next = days[today + 1]?.cheapest;
  return next ? { from: clockTime(next.start), to: clockTime(next.end), tomorrow: true } : null;
}
