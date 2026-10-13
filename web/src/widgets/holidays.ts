import { t, type TextKey } from '../i18n.svelte';
import { daysUntil, parseLocalDate } from './dates';

/** The school holiday kinds of internal/holidays; other shows the feed's Dutch name. */
export const SCHOOL_KINDS = ['autumn', 'christmas', 'spring', 'may', 'summer', 'other'] as const;
export type SchoolKind = (typeof SCHOOL_KINDS)[number];

/** The public holidays of internal/holidays. */
export const PUBLIC_KINDS = [
  'newYear',
  'goodFriday',
  'easter',
  'easterMonday',
  'kingsDay',
  'liberationDay',
  'ascension',
  'whitSunday',
  'whitMonday',
  'christmas',
  'boxingDay',
] as const;
export type PublicKind = (typeof PUBLIC_KINDS)[number];

export const REGIONS = ['north', 'middle', 'south'] as const;
export type Region = (typeof REGIONS)[number];
export type Show = 'both' | 'school' | 'public';

/** The widget's defaults when a setting is not set, as internal/display has them. */
const DEFAULT_REGION: Region = 'middle';
const DEFAULT_SHOW: Show = 'both';
const DEFAULT_MAX_ITEMS = 5;
/** Up to this many days ahead a school holiday says when it starts. */
const STARTS_SOON_DAYS = 14;
/** Within this many days the end of a holiday is named by its weekday. */
const WEEKDAY_DAYS = 6;

export const SCHOOL_TEXT: Record<Exclude<SchoolKind, 'other'>, TextKey> = {
  autumn: 'holidayAutumn',
  christmas: 'holidayChristmas',
  spring: 'holidaySpring',
  may: 'holidayMay',
  summer: 'holidaySummer',
};

export const PUBLIC_TEXT: Record<PublicKind, TextKey> = {
  newYear: 'publicNewYear',
  goodFriday: 'publicGoodFriday',
  easter: 'publicEaster',
  easterMonday: 'publicEasterMonday',
  kingsDay: 'publicKingsDay',
  liberationDay: 'publicLiberationDay',
  ascension: 'publicAscension',
  whitSunday: 'publicWhitSunday',
  whitMonday: 'publicWhitMonday',
  christmas: 'publicChristmas',
  boxingDay: 'publicBoxingDay',
};

interface SchoolHoliday {
  kind: SchoolKind;
  name: string;
  start: string;
  end: string;
}

interface PublicHoliday {
  kind: PublicKind;
  date: string;
}

/** One line of the widget: a school holiday, or a public holiday as a holiday of one day. */
export type Holiday =
  | { type: 'school'; kind: SchoolKind; name: string; start: string; end: string }
  | { type: 'public'; kind: PublicKind; start: string; end: string };

/** What a Holidays widget shows, with the defaults for what is not set. */
export function holidaySettings(settings: Record<string, unknown> | undefined): { region: Region; show: Show; maxItems: number } {
  const region = REGIONS.find((r) => r === settings?.region) ?? DEFAULT_REGION;
  const show = (['both', 'school', 'public'] as const).find((s) => s === settings?.show) ?? DEFAULT_SHOW;
  const maxItems = typeof settings?.maxItems === 'number' && settings.maxItems >= 1 ? Math.floor(settings.maxItems) : DEFAULT_MAX_ITEMS;
  return { region, show, maxItems };
}

function isSchool(v: unknown): v is SchoolHoliday {
  if (typeof v !== 'object' || v === null) return false;
  const { kind, name, start, end } = v as Record<string, unknown>;
  return SCHOOL_KINDS.includes(kind as SchoolKind) && typeof name === 'string' && typeof start === 'string' && typeof end === 'string';
}

function isPublic(v: unknown): v is PublicHoliday {
  if (typeof v !== 'object' || v === null) return false;
  const { kind, date } = v as Record<string, unknown>;
  return PUBLIC_KINDS.includes(kind as PublicKind) && typeof date === 'string';
}

/**
 * The holidays a widget shows out of the holidays data, unsorted. Undefined
 * while a part it needs is not there yet or unreadable: the server adds a
 * region once it has read the school holidays.
 */
export function readHolidays(value: unknown, region: Region, show: Show): Holiday[] | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const data = value as Record<string, unknown>;
  const holidays: Holiday[] = [];
  if (show !== 'public') {
    const school = data[region];
    if (!Array.isArray(school)) return undefined;
    for (const h of school.filter(isSchool)) holidays.push({ type: 'school', kind: h.kind, name: h.name, start: h.start, end: h.end });
  }
  if (show !== 'school') {
    const list = data.public;
    if (!Array.isArray(list)) return undefined;
    for (const h of list.filter(isPublic)) holidays.push({ type: 'public', kind: h.kind, start: h.date, end: h.date });
  }
  return holidays;
}

/** A holiday that has not ended, with its days from today. */
export type UpcomingHoliday = Holiday & { daysToStart: number; daysToEnd: number };

/**
 * The holidays that have not ended by today, the running and soonest
 * first, at most max. The last good data can be days old, so what has
 * passed is dropped here and not trusted to the server.
 */
export function upcomingHolidays(holidays: Holiday[], now: Date, max: number): UpcomingHoliday[] {
  const upcoming: UpcomingHoliday[] = [];
  for (const h of holidays) {
    const daysToStart = daysUntil(h.start, now);
    const daysToEnd = daysUntil(h.end, now);
    if (daysToStart === null || daysToEnd === null || daysToEnd < 0) continue;
    upcoming.push({ ...h, daysToStart, daysToEnd });
  }
  // The same start: the school holiday first, the longer one.
  upcoming.sort((a, b) => a.daysToStart - b.daysToStart || b.daysToEnd - a.daysToEnd);
  return upcoming.slice(0, max);
}

/** Whether a holiday is on today. */
export function isNow(h: UpcomingHoliday): boolean {
  return h.daysToStart <= 0;
}

/** A span of days, such as "17–25 Oct" or "19 Dec – 3 Jan". */
export function dayRange(start: string, end: string, locale: string): string {
  const from = parseLocalDate(start);
  const to = parseLocalDate(end);
  if (!from || !to) return `${start} – ${end}`;
  const dayMonth = new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'short' });
  if (from.getFullYear() === to.getFullYear() && from.getMonth() === to.getMonth()) {
    // The first day as the locale writes a day with a month, without the
    // month: "17." in German, where the day alone would lose its dot.
    const day = dayMonth
      .formatToParts(from)
      .filter((part) => part.type !== 'month')
      .map((part) => part.value)
      .join('')
      .trim();
    return `${day}–${dayMonth.format(to)}`;
  }
  return `${dayMonth.format(from)} – ${dayMonth.format(to)}`;
}

/**
 * When a holiday is, in the screen's language. A public holiday: today,
 * tomorrow or its date with the weekday. A school holiday on now: until
 * when, by weekday within the week; one that starts within two weeks: in
 * how many days, with its dates; one further off: its dates.
 */
export function holidayWhen(h: UpcomingHoliday, locale: string): string {
  const start = parseLocalDate(h.start);
  const end = parseLocalDate(h.end);
  if (!start || !end) return h.start;
  if (h.type === 'public') {
    if (h.daysToStart === 0) return t('today');
    if (h.daysToStart === 1) return t('tomorrow');
    return new Intl.DateTimeFormat(locale, { weekday: 'short', day: 'numeric', month: 'short' }).format(start);
  }
  if (isNow(h)) {
    if (h.daysToEnd === 0) return t('holidayLastDay');
    if (h.daysToEnd <= WEEKDAY_DAYS) return t('holidayUntilDay', { day: new Intl.DateTimeFormat(locale, { weekday: 'long' }).format(end) });
    return t('holidayUntilDate', { date: new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'short' }).format(end) });
  }
  const range = dayRange(h.start, h.end, locale);
  if (h.daysToStart === 1) return t('holidayStartsTomorrow', { range });
  if (h.daysToStart <= STARTS_SOON_DAYS) return t('holidayStartsIn', { days: h.daysToStart, range });
  return range;
}

/** The name of a holiday in the screen's language; a school holiday GDash does not know keeps its Dutch name. */
export function holidayName(h: Holiday): string {
  if (h.type === 'public') return t(PUBLIC_TEXT[h.kind]);
  return h.kind === 'other' ? h.name : t(SCHOOL_TEXT[h.kind]);
}
