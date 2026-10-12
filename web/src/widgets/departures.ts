import { locale, t } from '../i18n.svelte';

/** One departure as the server sends it (internal/departures.Departure). */
export interface Departure {
  line: string;
  destination: string;
  /** When the vehicle leaves according to the real-time data, as an RFC 3339 instant. */
  expected: string;
  /** Whole minutes later than the timetable; absent when on time. */
  delayMinutes?: number;
  cancelled?: boolean;
}

/** What the screen shows for one departure. */
export interface Row {
  line: string;
  destination: string;
  /** Whole minutes from now until the departure; 0 or less is "now". */
  minutes: number;
  /** Minutes later than the timetable, 0 when on time. */
  delay: number;
  cancelled: boolean;
  /** The departure time on a Dutch clock, shown for a cancelled trip. */
  clock: string;
}

/** Timetables are in Dutch time whatever zone the screen is set to. */
const TIME_ZONE = 'Europe/Amsterdam';

const MS_PER_MINUTE = 60_000;

function isDeparture(v: unknown): v is Departure {
  if (typeof v !== 'object' || v === null) return false;
  const d = v as Record<string, unknown>;
  return typeof d.line === 'string' && typeof d.destination === 'string' && typeof d.expected === 'string';
}

/**
 * Reads the server's value: stop area code to its departures. Anything that
 * is not a list of departures is ignored, so a broken value shows nothing
 * instead of breaking the widget.
 */
export function departureLists(value: unknown): Record<string, Departure[]> | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const out: Record<string, Departure[]> = {};
  for (const [code, list] of Object.entries(value)) {
    if (Array.isArray(list)) out[code] = list.filter(isDeparture);
  }
  return out;
}

/**
 * Whole minutes from now to the instant, counted on the clock face: a bus
 * due at 11:48:36 is "1 min" at 11:47:50, as it is on a stop's own display.
 * Undefined when the instant cannot be read.
 */
export function minutesUntil(expected: string, now: Date): number | undefined {
  const at = Date.parse(expected);
  if (Number.isNaN(at)) return undefined;
  return Math.floor(at / MS_PER_MINUTE) - Math.floor(now.getTime() / MS_PER_MINUTE);
}

/**
 * The departures still to come at `now`, in the server's order, at most
 * `max`. The server refreshes every 30 seconds, but a vehicle leaves in
 * between: it must disappear from the screen on time.
 */
export function upcoming(list: Departure[], now: Date, max: number): Row[] {
  const clockFormat = new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit', timeZone: TIME_ZONE });
  const rows: Row[] = [];
  for (const d of list) {
    const minutes = minutesUntil(d.expected, now);
    if (minutes === undefined || minutes < 0) continue;
    rows.push({
      line: d.line,
      destination: d.destination,
      minutes,
      delay: Math.max(0, Math.trunc(d.delayMinutes ?? 0)),
      cancelled: d.cancelled === true,
      clock: clockFormat.format(new Date(d.expected)),
    });
    if (rows.length >= max) break;
  }
  return rows;
}

/** "now" or "3 min". */
export function whenText(minutes: number): string {
  return minutes <= 0 ? t('departuresNow') : t('departuresMinutes', { minutes });
}
