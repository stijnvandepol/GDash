import { coordinate } from '../coordinates';
import { locale, t, type TextKey } from '../i18n.svelte';
import type { Entry } from '../protocol';
import { waitingText } from './waiting';

/** One station as the server sends it (internal/fuel.Station). */
export interface Station {
  name: string;
  /** Euro per litre, or per kilogram for CNG. */
  price: number;
  distanceKm: number;
}

/** A place chosen with the place search. */
export interface Place {
  name: string;
  latitude: number;
  longitude: number;
}

/** What the screen shows for one station. */
export interface Row extends Station {
  /** The cheapest of the rows; it is marked. */
  cheapest: boolean;
}

// These defaults are the server's too (internal/fuel).
export const DEFAULT_RADIUS_KM = 5;
export const DEFAULT_FUEL = 'EURO95';
export const DEFAULT_ITEMS = 5;

/** The names of the fuels, by the ANWB's codes. */
export const FUEL_TEXT: Record<string, TextKey> = {
  EURO95: 'fuelEuro95',
  EURO98: 'fuelEuro98',
  DIESEL: 'fuelDiesel',
  AUTOGAS: 'fuelAutogas',
  CNG: 'fuelCng',
};

const CENTS_PER_EURO = 100;
const MILLS_PER_CENT = 10;
const MILLS_PER_EURO = 1000;

/** A place setting, or undefined when there is none or it is not a place. */
export function readPlace(value: unknown): Place | undefined {
  if (typeof value !== 'object' || value === null) return undefined;
  const { name, latitude, longitude } = value as Record<string, unknown>;
  if (typeof name !== 'string' || typeof latitude !== 'number' || typeof longitude !== 'number') return undefined;
  return { name, latitude, longitude };
}

/** The radius in kilometres a widget searches. */
export function radiusOf(settings: Record<string, unknown> | undefined): number {
  return typeof settings?.radius === 'number' ? settings.radius : DEFAULT_RADIUS_KM;
}

/** The ANWB code of the fuel a widget shows. */
export function fuelOf(settings: Record<string, unknown> | undefined): string {
  return typeof settings?.fuel === 'string' && settings.fuel in FUEL_TEXT ? settings.fuel : DEFAULT_FUEL;
}

/**
 * The key of the widget's list in the server's data (internal/fuel.Query.Key).
 * Undefined when the place setting is there but is no place, which the
 * server ignores too.
 */
export function fuelKey(settings: Record<string, unknown> | undefined): string | undefined {
  const radius = radiusOf(settings);
  const fuel = fuelOf(settings);
  if (settings?.place === undefined) return `here,${radius},${fuel}`;
  const place = readPlace(settings.place);
  return place ? `${coordinate(place.latitude)},${coordinate(place.longitude)},${radius},${fuel}` : undefined;
}

function isStation(v: unknown): v is Station {
  if (typeof v !== 'object' || v === null) return false;
  const s = v as Record<string, unknown>;
  return typeof s.name === 'string' && typeof s.price === 'number' && typeof s.distanceKm === 'number';
}

/**
 * The list under key in the server's value, or undefined when the value has
 * none. Anything that is not a station is ignored, so a broken value shows
 * fewer rows instead of breaking the widget.
 */
export function stationList(value: unknown, key: string | undefined): Station[] | undefined {
  if (key === undefined || typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const list = (value as Record<string, unknown>)[key];
  return Array.isArray(list) ? list.filter(isStation) : undefined;
}

/** The stations to show, cheapest first as the server sends them, at most `max`. */
export function fuelRows(list: Station[], max: number): Row[] {
  return list.slice(0, max).map((station, i) => ({ ...station, cheapest: i === 0 }));
}

/** A price with three decimals as pumps show it, such as "€2.469". */
export function priceText(price: number): string {
  return new Intl.NumberFormat(locale(), {
    style: 'currency',
    currency: 'EUR',
    minimumFractionDigits: 3,
    maximumFractionDigits: 3,
  }).format(price);
}

/** "1.8 km". */
export function distanceText(km: number): string {
  const number = new Intl.NumberFormat(locale(), { minimumFractionDigits: 1, maximumFractionDigits: 1 }).format(km);
  return t('fuelDistance', { km: number });
}

/** "/l", or "/kg" for CNG. */
export function unitText(fuel: string): string {
  return t(fuel === 'CNG' ? 'fuelPerKg' : 'fuelPerLitre');
}

/**
 * "save €0.26/l": what the cheapest row saves against the dearest of the
 * rows, in whole cents. Undefined when that is less than a cent, which is
 * the case for a list of one.
 */
export function savingText(rows: Row[], fuel: string): string | undefined {
  const first = rows[0];
  const last = rows[rows.length - 1];
  if (!first || !last) return undefined;
  // Whole thousandths of a euro, so rounding does not depend on binary fractions.
  const mills = Math.round(last.price * MILLS_PER_EURO) - Math.round(first.price * MILLS_PER_EURO);
  const cents = Math.round(mills / MILLS_PER_CENT);
  if (cents <= 0) return undefined;
  const amount = new Intl.NumberFormat(locale(), { style: 'currency', currency: 'EUR' }).format(cents / CENTS_PER_EURO);
  return t('fuelSave', { amount, unit: unitText(fuel) });
}

/**
 * What the widget says while it has no list. A widget without a place
 * follows the weather location; when other widgets have lists and this one
 * has not, there is no weather location to follow.
 */
export function fuelWaitingText(entry: Entry | undefined, settings: Record<string, unknown> | undefined): string {
  if (entry?.value !== undefined && !entry.failing && settings?.place === undefined) return t('fuelNotSetUp');
  return waitingText(entry, 'fuel');
}
