import { coordinate } from '../coordinates';
import { t, type TextKey } from '../i18n.svelte';
import type { Entry } from '../protocol';
import { clockTime } from './energyprice';
import { readPlace } from './fuel';
import { waitingText } from './waiting';

/** One step of the forecast as the server sends it (internal/rain.Point). */
export interface RainPoint {
  /** The start of the step, an RFC 3339 instant written in the place's time zone. */
  time: string;
  mmPerHour: number;
}

/** The forecast for one place (internal/rain.Forecast). */
export interface RainForecast {
  /** "buienradar" or "open-meteo". */
  source: string;
  stepMinutes: number;
  points: RainPoint[];
}

/**
 * How hard it rains, in millimetres per hour. Below 0.1 it is dry: that
 * is Buienradar's lowest step of drizzle. Light rain stays under 2.5 and
 * moderate rain under 10, the usual meteorological bounds; heavy is what
 * floods a gutter. docs/guide.md lists the same numbers.
 */
export const DRY_BELOW_MM = 0.1;
export const LIGHT_BELOW_MM = 2.5;
export const MODERATE_BELOW_MM = 10;

export type Intensity = 'light' | 'moderate' | 'heavy';

export const INTENSITY_TEXT: Record<Intensity, TextKey> = {
  light: 'rainLight',
  moderate: 'rainModerate',
  heavy: 'rainHeavy',
};

/** The names of the sources, as their terms ask to credit them. */
export const SOURCE_NAMES: Record<string, string> = {
  buienradar: 'Buienradar.nl',
  'open-meteo': 'Open-Meteo.com',
};

/** The widget looks this far ahead. */
const WINDOW_MS = 2 * 3_600_000;
const MINUTE_MS = 60_000;
/** A bar with rain never shrinks below this, so a drizzle still shows. */
const MIN_WET_BAR = 0.08;
/** Bar heights grow with the square root of the rain, full at heavy rain. */
const FULL_BAR_MM = MODERATE_BELOW_MM;

/** The intensity of a step, or undefined when it is dry. */
export function intensityOf(mmPerHour: number): Intensity | undefined {
  if (mmPerHour < DRY_BELOW_MM) return undefined;
  if (mmPerHour < LIGHT_BELOW_MM) return 'light';
  return mmPerHour < MODERATE_BELOW_MM ? 'moderate' : 'heavy';
}

/**
 * The key of the widget's forecast in the server's data
 * (internal/rain.Query.Key): "here" without a place, else the place rounded
 * to two decimals. Undefined when the place setting is no place, which the
 * server ignores too.
 */
export function rainKey(settings: Record<string, unknown> | undefined): string | undefined {
  if (settings?.place === undefined) return 'here';
  const place = readPlace(settings.place);
  return place ? `${coordinate(place.latitude, 2)},${coordinate(place.longitude, 2)}` : undefined;
}

function isPoint(v: unknown): v is RainPoint {
  if (typeof v !== 'object' || v === null) return false;
  const { time, mmPerHour } = v as Record<string, unknown>;
  return typeof time === 'string' && Number.isFinite(Date.parse(time)) && typeof mmPerHour === 'number' && Number.isFinite(mmPerHour);
}

/** The forecast under key in the server's value, or undefined when there is none. */
export function readForecast(value: unknown, key: string | undefined): RainForecast | undefined {
  if (key === undefined || typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const f = (value as Record<string, unknown>)[key];
  if (typeof f !== 'object' || f === null) return undefined;
  const { source, stepMinutes, points } = f as Record<string, unknown>;
  if (typeof source !== 'string' || typeof stepMinutes !== 'number' || stepMinutes <= 0 || !Array.isArray(points)) return undefined;
  return { source, stepMinutes, points: points.filter(isPoint) };
}

/** What the sentence under the bars says. */
export type RainSummary =
  | { kind: 'dry' }
  | { kind: 'from'; time: string; intensity: Intensity }
  | { kind: 'until'; time: string; intensity: Intensity }
  | { kind: 'all'; intensity: Intensity };

export interface RainBar {
  /** Height from 0 to 1; 0 is dry. */
  height: number;
  intensity: Intensity | undefined;
}

export interface RainView {
  bars: RainBar[];
  summary: RainSummary;
  /** Clock times under the bars: the middle and the start of the last step. */
  middle: string;
  last: string;
  source: string;
}

/** The steps from the one that holds now to two hours ahead. */
export function upcoming(forecast: RainForecast, nowMs: number): RainPoint[] {
  const stepMs = forecast.stepMinutes * MINUTE_MS;
  return forecast.points.filter((p) => {
    const start = Date.parse(p.time);
    return start + stepMs > nowMs && start < nowMs + WINDOW_MS;
  });
}

function strongest(points: RainPoint[]): Intensity {
  return intensityOf(Math.max(...points.map((p) => p.mmPerHour))) ?? 'light';
}

/**
 * Dry, rain from a time, rain until a time or rain all along, for the
 * first spell of rain in points. Its intensity is that of its heaviest
 * step. "Until" is when the first dry step starts.
 */
export function summarize(points: RainPoint[]): RainSummary {
  const wet = (p: RainPoint) => intensityOf(p.mmPerHour) !== undefined;
  const first = points.findIndex(wet);
  if (first < 0) return { kind: 'dry' };
  const afterSpell = points.findIndex((p, i) => i > first && !wet(p));
  const spell = points.slice(first, afterSpell < 0 ? undefined : afterSpell);
  const intensity = strongest(spell);
  if (first > 0) return { kind: 'from', time: clockTime(points[first]!.time), intensity };
  if (afterSpell < 0) return { kind: 'all', intensity };
  return { kind: 'until', time: clockTime(points[afterSpell]!.time), intensity };
}

/** The sentence for a summary in the screen's language, such as "Rain from 14:20 (light)". */
export function rainSentence(summary: RainSummary): string {
  if (summary.kind === 'dry') return t('rainDry');
  const intensity = t(INTENSITY_TEXT[summary.intensity]);
  if (summary.kind === 'all') return t('rainAll', { intensity });
  return t(summary.kind === 'from' ? 'rainFrom' : 'rainUntil', { time: summary.time, intensity });
}

/** The bars, the sentence and the times to show, or undefined when no step lies ahead. */
export function rainView(forecast: RainForecast, nowMs: number): RainView | undefined {
  const points = upcoming(forecast, nowMs);
  const last = points[points.length - 1];
  if (!last) return undefined;
  const bars = points.map((p) => {
    const intensity = intensityOf(p.mmPerHour);
    return { height: intensity ? Math.max(MIN_WET_BAR, Math.min(1, Math.sqrt(p.mmPerHour / FULL_BAR_MM))) : 0, intensity };
  });
  return {
    bars,
    summary: summarize(points),
    middle: clockTime(points[Math.floor(points.length / 2)]!.time),
    last: clockTime(last.time),
    source: SOURCE_NAMES[forecast.source] ?? forecast.source,
  };
}

/**
 * What the widget says while it has no forecast. A widget without a place
 * follows the weather location; when other widgets have forecasts and this
 * one has not, there is no weather location to follow.
 */
export function rainWaitingText(entry: Entry | undefined, settings: Record<string, unknown> | undefined): string {
  if (entry?.value !== undefined && !entry.failing && settings?.place === undefined) return t('rainNotSetUp');
  return waitingText(entry, 'rain');
}
