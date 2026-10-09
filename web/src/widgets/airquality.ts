import type { TextKey } from '../i18n.svelte';

/** The levels of the European Air Quality Index, best first. */
export const AQI_LEVELS = ['good', 'fair', 'moderate', 'poor', 'veryPoor', 'extremelyPoor'] as const;
export type AqiLevel = (typeof AQI_LEVELS)[number];

/** The highest index of each level, from the European Environment Agency's bands. Above the last is extremely poor. */
const AQI_LIMITS = [20, 40, 60, 80, 100];

/** The level of a European AQI. */
export function aqiLevel(index: number): AqiLevel {
  const i = AQI_LIMITS.findIndex((limit) => index <= limit);
  return AQI_LEVELS[i === -1 ? AQI_LIMITS.length : i] ?? 'extremelyPoor';
}

/** The words for each level; the screen picks the language. */
export const AQI_TEXT: Record<AqiLevel, TextKey> = {
  good: 'aqiGood',
  fair: 'aqiFair',
  moderate: 'aqiModerate',
  poor: 'aqiPoor',
  veryPoor: 'aqiVeryPoor',
  extremelyPoor: 'aqiExtremelyPoor',
};

/** The kinds of pollen Open-Meteo models, in the order the server sends them. */
export const POLLEN_TYPES = ['alder', 'birch', 'grass', 'mugwort', 'olive', 'ragweed'] as const;
export type PollenType = (typeof POLLEN_TYPES)[number];

export const POLLEN_TEXT: Record<PollenType, TextKey> = {
  alder: 'pollenAlder',
  birch: 'pollenBirch',
  grass: 'pollenGrass',
  mugwort: 'pollenMugwort',
  olive: 'pollenOlive',
  ragweed: 'pollenRagweed',
};

/**
 * How much pollen is a lot differs per kind and per country, and Open-Meteo
 * gives none. These bands, in grains per cubic metre, are a rough common
 * scale for people who want to know whether to expect symptoms: they say
 * low, moderate, high and very high for every kind alike.
 */
const POLLEN_LEVELS = ['low', 'moderate', 'high', 'veryHigh'] as const;
export type PollenLevel = (typeof POLLEN_LEVELS)[number];
const POLLEN_LIMITS = [10, 50, 200];

/** The level of a pollen concentration above zero. */
export function pollenLevel(grainsPerM3: number): PollenLevel {
  const i = POLLEN_LIMITS.findIndex((limit) => grainsPerM3 < limit);
  return POLLEN_LEVELS[i === -1 ? POLLEN_LIMITS.length : i] ?? 'veryHigh';
}

export const POLLEN_LEVEL_TEXT: Record<PollenLevel, TextKey> = {
  low: 'pollenLow',
  moderate: 'pollenModerate',
  high: 'pollenHigh',
  veryHigh: 'pollenVeryHigh',
};

export interface Pollen {
  type: PollenType;
  grainsPerM3: number;
}

/** What the widget shows of the server's report. */
export interface AirQuality {
  index: number;
  /** Whether the report has pollen at all, which it does not outside Europe. */
  hasPollen: boolean;
  /** The pollen that is in the air, the most first. */
  pollen: Pollen[];
}

/** Reads the report, or returns undefined when it has no usable index. */
export function readAirQuality(report: unknown): AirQuality | undefined {
  if (typeof report !== 'object' || report === null) return undefined;
  const { europeanAqi, pollen } = report as { europeanAqi?: unknown; pollen?: unknown };
  if (typeof europeanAqi !== 'number' || !Number.isFinite(europeanAqi) || europeanAqi < 0) return undefined;
  const readings = Array.isArray(pollen) ? pollen.filter(isPollen) : [];
  return {
    index: Math.round(europeanAqi),
    hasPollen: readings.length > 0,
    pollen: readings.filter((p) => p.grainsPerM3 > 0).sort((a, b) => b.grainsPerM3 - a.grainsPerM3),
  };
}

function isPollen(v: unknown): v is Pollen {
  if (typeof v !== 'object' || v === null) return false;
  const { type, grainsPerM3 } = v as Record<string, unknown>;
  return (
    POLLEN_TYPES.includes(type as PollenType) && typeof grainsPerM3 === 'number' && Number.isFinite(grainsPerM3) && grainsPerM3 >= 0
  );
}
