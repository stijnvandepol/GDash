import type { TextKey } from '../i18n.svelte';
import { daysUntil, parseLocalDate } from './dates';

/** The kinds of waste the server recognises in event titles (internal/waste), in the order of a day's list. */
export const WASTE_KINDS = ['gft', 'pmd', 'paper', 'rest', 'glass', 'textile', 'other'] as const;
export type WasteKind = (typeof WASTE_KINDS)[number];

/** How many days ahead the widget looks. */
export const WASTE_DAYS = 14;

/** The plain label of each kind. Other has none: it shows the calendar's own title. */
export const WASTE_TEXT: Record<Exclude<WasteKind, 'other'>, TextKey> = {
  gft: 'wasteOrganic',
  pmd: 'wastePlastic',
  paper: 'wastePaper',
  rest: 'wasteGeneral',
  glass: 'wasteGlass',
  textile: 'wasteTextile',
};

/**
 * The dot colour of each kind, by a name of colors.ts, after the colour of
 * the container in most Dutch municipalities. General waste has none: it
 * takes the theme's muted text colour, as grey is nobody's accent.
 */
export const WASTE_COLOR: Record<WasteKind, string | undefined> = {
  gft: 'green',
  pmd: 'orange',
  paper: 'blue',
  rest: undefined,
  glass: 'teal',
  textile: 'purple',
  other: 'yellow',
};

/** One pickup as the server sends it. */
export interface Pickup {
  kind: WasteKind;
  title: string;
  /** The local day, YYYY-MM-DD. */
  date: string;
}

/** A pickup that comes up: the first of its kind, and in how many days. */
export interface NextPickup extends Pickup {
  days: number;
}

function isPickup(v: unknown): v is Pickup {
  if (typeof v !== 'object' || v === null) return false;
  const { kind, title, date } = v as Record<string, unknown>;
  return WASTE_KINDS.includes(kind as WasteKind) && typeof title === 'string' && typeof date === 'string';
}

/**
 * The pickups of one calendar out of the waste data, which holds the
 * calendars of the widgets by their number. Undefined means the data is not
 * there yet or unreadable; 'missing' means the data is there but this
 * calendar is not in it, because it was deleted or cannot be read.
 */
export function readWaste(value: unknown, calendar: unknown): Pickup[] | 'missing' | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  if (typeof calendar !== 'number') return 'missing';
  const list = (value as Record<string, unknown>)[String(calendar)];
  if (list === undefined) return 'missing';
  return Array.isArray(list) ? list.filter(isPickup) : undefined;
}

/**
 * The next pickup of each kind within the coming days, soonest first. A
 * kind comes once; unknown titles come once per title, as each may be a
 * different bin. The last good data can be older than today, so days that
 * have passed are skipped here and not trusted to the server.
 */
export function nextPickups(pickups: Pickup[], now: Date, horizon = WASTE_DAYS): NextPickup[] {
  const seen = new Set<string>();
  const next: NextPickup[] = [];
  for (const pickup of pickups) {
    const days = daysUntil(pickup.date, now);
    if (days === null || days < 0 || days > horizon) continue;
    const identity = pickup.kind === 'other' ? `other:${pickup.title.toLowerCase()}` : pickup.kind;
    if (seen.has(identity)) continue;
    seen.add(identity);
    next.push({ ...pickup, days });
  }
  // Same day: the order of the kinds, so a list does not jump between refreshes.
  return next.sort((a, b) => a.days - b.days || WASTE_KINDS.indexOf(a.kind) - WASTE_KINDS.indexOf(b.kind));
}

/**
 * When a pickup is, in words: today, tomorrow, the weekday within the week,
 * and from a week on the weekday with the date, since "Thursday" would then
 * be ambiguous.
 */
export function pickupDay(pickup: NextPickup, words: { today: string; tomorrow: string }, locale: string): string {
  if (pickup.days === 0) return words.today;
  if (pickup.days === 1) return words.tomorrow;
  const date = parseLocalDate(pickup.date);
  if (!date) return pickup.date;
  if (pickup.days < 7) return new Intl.DateTimeFormat(locale, { weekday: 'long' }).format(date);
  return new Intl.DateTimeFormat(locale, { weekday: 'short', day: 'numeric', month: 'short' }).format(date);
}
