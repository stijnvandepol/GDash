import type { TextKey } from '../i18n.svelte';

/** One alert as the server sends it (internal/p2000.Alert). */
export interface Alert {
  id: string;
  /** When the alert went out, as an RFC 3339 instant. */
  at: string;
  discipline: string;
  /** The place's slug, as in alarmeringen.nl's links. */
  place: string;
  /** The readable sentence, such as "Woningbrand op Dorpsstraat in Nuenen". */
  text: string;
}

/** The server's value (internal/p2000.Value). */
export interface P2000Value {
  /** Alerts by feed key, region/discipline, newest first. */
  feeds: Record<string, Alert[]>;
  /** Feed keys the server does not fetch: the dashboards ask for too many. */
  skipped: string[];
}

/** The disciplines in the editor's order; internal/p2000 holds the same. */
export const DISCIPLINES = ['brandweer', 'politie', 'ambulance', 'knrm', 'trauma'] as const;

/** All but ambulance, whose rides are most alerts and say little; internal/p2000 holds the same. */
export const DEFAULT_DISCIPLINES: readonly string[] = ['brandweer', 'politie', 'knrm', 'trauma'];

export const DISCIPLINE_TEXT: Record<string, TextKey> = {
  brandweer: 'p2000Brandweer',
  politie: 'p2000Politie',
  ambulance: 'p2000Ambulance',
  knrm: 'p2000Knrm',
  trauma: 'p2000Trauma',
};

/** The safety regions by slug, in internal/p2000's order. */
export const REGION_NAMES: Record<string, string> = {
  groningen: 'Groningen',
  friesland: 'Fryslân',
  drenthe: 'Drenthe',
  ijsselland: 'IJsselland',
  twente: 'Twente',
  'noord-en-oost-gelderland': 'Noord- en Oost-Gelderland',
  'gelderland-midden': 'Gelderland-Midden',
  'gelderland-zuid': 'Gelderland-Zuid',
  utrecht: 'Utrecht',
  'noord-holland-noord': 'Noord-Holland Noord',
  'zaanstreek-waterland': 'Zaanstreek-Waterland',
  kennemerland: 'Kennemerland',
  'amsterdam-amstelland': 'Amsterdam-Amstelland',
  'gooi-en-vechtstreek': 'Gooi en Vechtstreek',
  haaglanden: 'Haaglanden',
  'hollands-midden': 'Hollands Midden',
  'rotterdam-rijnmond': 'Rotterdam-Rijnmond',
  'zuid-holland-zuid': 'Zuid-Holland Zuid',
  zeeland: 'Zeeland',
  'midden-en-west-brabant': 'Midden- en West-Brabant',
  'brabant-noord': 'Brabant-Noord',
  'brabant-zuidoost': 'Brabant-Zuidoost',
  'limburg-noord': 'Limburg-Noord',
  'limburg-zuid': 'Zuid-Limburg',
  flevoland: 'Flevoland',
};

/** A place in a widget's settings, found with the place test. */
export interface Place {
  name: string;
  slug: string;
  region: string;
}

const DEFAULT_HOURS = 12;
const DEFAULT_ITEMS = 8;
const MS_PER_MINUTE = 60_000;
const MINUTES_PER_HOUR = 60;
// An alert a little ahead of the screen's clock is still shown: the clocks
// of the server and the TV differ by seconds.
const FUTURE_SLACK_MS = 2 * MS_PER_MINUTE;

function isAlert(v: unknown): v is Alert {
  if (typeof v !== 'object' || v === null) return false;
  const a = v as Record<string, unknown>;
  return (
    typeof a.id === 'string' && typeof a.at === 'string' && typeof a.discipline === 'string' && typeof a.place === 'string' && typeof a.text === 'string'
  );
}

/** Reads the server's value; anything broken is left out, so the widget shows nothing instead of breaking. */
export function readValue(value: unknown): P2000Value | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const v = value as Record<string, unknown>;
  if (typeof v.feeds !== 'object' || v.feeds === null || Array.isArray(v.feeds)) return undefined;
  const feeds: Record<string, Alert[]> = {};
  for (const [key, list] of Object.entries(v.feeds)) {
    if (Array.isArray(list)) feeds[key] = list.filter(isAlert);
  }
  const skipped = Array.isArray(v.skipped) ? v.skipped.filter((k): k is string => typeof k === 'string') : [];
  return { feeds, skipped };
}

/** Reads a places setting; anything that is not a place is left out. */
export function readPlaces(value: unknown): Place[] {
  if (!Array.isArray(value)) return [];
  return value.filter(
    (p): p is Place =>
      typeof p === 'object' && p !== null && typeof p.name === 'string' && typeof p.slug === 'string' && typeof p.region === 'string',
  );
}

function regionOf(settings: Record<string, unknown> | undefined): string | undefined {
  return typeof settings?.region === 'string' && settings.region in REGION_NAMES ? settings.region : undefined;
}

function disciplinesOf(settings: Record<string, unknown> | undefined): readonly string[] {
  const chosen = settings?.disciplines;
  if (!Array.isArray(chosen)) return DEFAULT_DISCIPLINES;
  return chosen.filter((d): d is string => typeof d === 'string' && (DISCIPLINES as readonly string[]).includes(d));
}

function numberOf(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : fallback;
}

/** The hours of alerts a widget shows. */
export function hoursOf(settings: Record<string, unknown> | undefined): number {
  return numberOf(settings?.hours, DEFAULT_HOURS);
}

/** The feed keys a widget shows, as internal/display.p2000Feeds makes them. */
export function feedKeys(settings: Record<string, unknown> | undefined): string[] {
  const region = regionOf(settings);
  const regions = region ? [region] : [...new Set(readPlaces(settings?.places).map((p) => p.region))];
  return regions.flatMap((r) => disciplinesOf(settings).map((d) => `${r}/${d}`));
}

/** What the screen shows for one alert. */
export interface Row {
  id: string;
  discipline: string;
  text: string;
  /** Whole minutes since the alert went out, 0 for the first minute. */
  minutes: number;
}

/**
 * The rows a widget shows at `now`: the alerts of its feeds, of its places
 * when it has places, of the last hours, each once, newest first, at most
 * maxItems. 'skipped' when the server does not fetch one of its feeds, and
 * undefined until every feed has arrived.
 */
export function rowsFor(value: P2000Value, settings: Record<string, unknown> | undefined, now: Date): Row[] | 'skipped' | undefined {
  const keys = feedKeys(settings);
  if (keys.some((k) => value.skipped.includes(k))) return 'skipped';
  if (keys.length === 0 || keys.some((k) => !(k in value.feeds))) return undefined;
  const places = regionOf(settings) ? undefined : new Set(readPlaces(settings?.places).map((p) => p.slug));
  const from = now.getTime() - hoursOf(settings) * MINUTES_PER_HOUR * MS_PER_MINUTE;
  const until = now.getTime() + FUTURE_SLACK_MS;
  const seen = new Set<string>();
  const found: { row: Row; at: number }[] = [];
  for (const key of keys) {
    for (const alert of value.feeds[key]!) {
      const at = Date.parse(alert.at);
      if (Number.isNaN(at) || at < from || at > until || seen.has(alert.id)) continue;
      if (places && !places.has(alert.place)) continue;
      seen.add(alert.id);
      const minutes = Math.max(0, Math.floor((now.getTime() - at) / MS_PER_MINUTE));
      found.push({ row: { id: alert.id, discipline: alert.discipline, text: alert.text, minutes }, at });
    }
  }
  found.sort((a, b) => b.at - a.at);
  return found.slice(0, numberOf(settings?.maxItems, DEFAULT_ITEMS)).map((f) => f.row);
}

/** How many alerts a feed holds at most; internal/p2000.MaxPerFeed. */
const MAX_PER_FEED = 100;

/**
 * How many whole hours back the widget's feeds reach, at most its hours. A
 * full feed of a busy region and service, such as ambulance in Amsterdam,
 * holds less than a day: a quiet tile must not claim more than that.
 */
export function coveredHours(value: P2000Value, settings: Record<string, unknown> | undefined, now: Date): number {
  const hours = hoursOf(settings);
  let from = now.getTime() - hours * MINUTES_PER_HOUR * MS_PER_MINUTE;
  for (const key of feedKeys(settings)) {
    const list = value.feeds[key];
    if (!list || list.length < MAX_PER_FEED) continue;
    const times = list.map((a) => Date.parse(a.at)).filter((at) => !Number.isNaN(at));
    if (times.length > 0) from = Math.max(from, Math.min(...times));
  }
  return Math.max(1, Math.min(hours, Math.floor((now.getTime() - from) / (MINUTES_PER_HOUR * MS_PER_MINUTE))));
}

type Text = (key: TextKey, values?: Record<string, string | number>) => string;

/** How long ago an alert went out: "now", "12 min", "3 h". */
export function agoText(minutes: number, text: Text): string {
  if (minutes < 1) return text('p2000Now');
  if (minutes < MINUTES_PER_HOUR) return text('p2000Minutes', { minutes });
  return text('p2000Hours', { hours: Math.floor(minutes / MINUTES_PER_HOUR) });
}

/** What a widget covers, for its title: the region's name, or the places' names. */
export function areaName(settings: Record<string, unknown> | undefined): string {
  const region = regionOf(settings);
  return region ? REGION_NAMES[region]! : readPlaces(settings?.places).map((p) => p.name).join(', ');
}
