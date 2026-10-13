import { currentLanguage, locale, t, type TextKey } from '../i18n.svelte';
import type { Entry } from '../protocol';
import { waitingText } from './waiting';

export type WarningLevel = 'yellow' | 'orange' | 'red';

/** One warning as the server sends it (internal/warnings.Warning). */
export interface WeatherWarning {
  id: string;
  level: WarningLevel;
  /** MeteoAlarm's awareness type, such as "wind", or empty when unknown. */
  kind: string;
  /** The KNMI's own texts by language, "nl" and "en". */
  event: Record<string, string>;
  headline: Record<string, string>;
  onset: string;
  expires: string;
}

/** The warnings of one region (internal/warnings.Area). */
export interface WarningArea {
  region: string;
  warnings: WeatherWarning[];
}

export const LEVEL_TEXT: Record<WarningLevel, TextKey> = {
  yellow: 'warningsYellow',
  orange: 'warningsOrange',
  red: 'warningsRed',
};

/** The names of MeteoAlarm's awareness types. */
export const KIND_TEXT: Record<string, TextKey> = {
  wind: 'warningsWind',
  'snow-ice': 'warningsSnowIce',
  thunderstorm: 'warningsThunderstorm',
  fog: 'warningsFog',
  'high-temperature': 'warningsHeat',
  'low-temperature': 'warningsCold',
  'coastal-event': 'warningsCoastal',
  'forest-fire': 'warningsForestFire',
  avalanches: 'warningsAvalanches',
  rain: 'warningsRain',
  flooding: 'warningsFlooding',
  'rain-flood': 'warningsRainFlood',
};

const LEVELS: readonly string[] = ['yellow', 'orange', 'red'];
const DAY_MS = 86_400_000;

/**
 * The key of the widget's warnings in the server's data
 * (internal/warnings.Query.Key): "here" without a region, else the region.
 */
export function warningsKey(settings: Record<string, unknown> | undefined): string | undefined {
  const region = settings?.region;
  if (region === undefined) return 'here';
  return typeof region === 'string' ? region : undefined;
}

function isTexts(v: unknown): v is Record<string, string> {
  return typeof v === 'object' && v !== null && Object.values(v).every((text) => typeof text === 'string');
}

function isWarning(v: unknown): v is WeatherWarning {
  if (typeof v !== 'object' || v === null) return false;
  const w = v as Record<string, unknown>;
  return (
    typeof w.id === 'string' &&
    typeof w.level === 'string' &&
    LEVELS.includes(w.level) &&
    typeof w.kind === 'string' &&
    isTexts(w.event) &&
    isTexts(w.headline) &&
    typeof w.onset === 'string' &&
    Number.isFinite(Date.parse(w.onset)) &&
    typeof w.expires === 'string' &&
    Number.isFinite(Date.parse(w.expires))
  );
}

/** The area under key in the server's value, or undefined when there is none. */
export function readArea(value: unknown, key: string | undefined): WarningArea | undefined {
  if (key === undefined || typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const area = (value as Record<string, unknown>)[key];
  if (typeof area !== 'object' || area === null) return undefined;
  const { region, warnings } = area as Record<string, unknown>;
  if (typeof region !== 'string' || !Array.isArray(warnings)) return undefined;
  return { region, warnings: warnings.filter(isWarning) };
}

/** The warnings that have not expired at now, as the server sorted them: soonest first. */
export function activeWarnings(area: WarningArea, nowMs: number): WeatherWarning[] {
  return area.warnings.filter((w) => Date.parse(w.expires) > nowMs);
}

/** The KNMI's text in the screen's language: Dutch on a Dutch screen, English on the others. */
function knmiText(texts: Record<string, string>): string | undefined {
  return texts[currentLanguage() === 'nl' ? 'nl' : 'en'];
}

/** What the warning is about, such as "Wind", in the screen's language. */
export function warningName(w: WeatherWarning): string {
  const key = KIND_TEXT[w.kind];
  if (key) return t(key);
  return knmiText(w.event) ?? w.event.en ?? w.event.nl ?? '';
}

/** The KNMI's headline in the screen's language, or undefined on a German or French screen. */
export function warningHeadline(w: WeatherWarning): string | undefined {
  return currentLanguage() === 'nl' || currentLanguage() === 'en' ? knmiText(w.headline) : undefined;
}

/** A time on this screen's clock, with the weekday when it is not today: "22:00" or "Wed 03:00". */
function moment(ms: number, nowMs: number): string {
  const sameDay = new Date(ms).toDateString() === new Date(nowMs).toDateString();
  const options: Intl.DateTimeFormatOptions = { hour: '2-digit', minute: '2-digit' };
  if (!sameDay || Math.abs(ms - nowMs) >= DAY_MS) options.weekday = 'short';
  return new Intl.DateTimeFormat(locale(), options).format(ms);
}

/** "until 22:00" while the warning is in effect, else "Wed 14:00 – 22:00". */
export function warningWindow(w: WeatherWarning, nowMs: number): string {
  const onset = Date.parse(w.onset);
  const expires = Date.parse(w.expires);
  if (onset <= nowMs) return t('warningsUntil', { time: moment(expires, nowMs) });
  return `${moment(onset, nowMs)} – ${moment(expires, onset)}`;
}

/**
 * What the widget says while it has no warnings to read. Without a region
 * the widget takes the province of the weather location; when other
 * widgets have warnings and this one has not, there is none to take.
 */
export function warningsWaitingText(entry: Entry | undefined, settings: Record<string, unknown> | undefined): string {
  if (entry?.value !== undefined && !entry.failing && settings?.region === undefined) return t('warningsNotSetUp');
  return waitingText(entry, 'warnings');
}
