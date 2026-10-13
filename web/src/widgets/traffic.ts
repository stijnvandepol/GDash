import { locale, t } from '../i18n.svelte';
import type { Entry } from '../protocol';
import { waitingText } from './waiting';

// These defaults and bounds are the server's too (internal/traffic).
export const DEFAULT_RADIUS_KM = 25;
export const DEFAULT_ITEMS = 5;
export const KINDS = ['jams', 'roadworks', 'speedchecks', 'cameras', 'totals'] as const;
export type Kind = (typeof KINDS)[number];
export const DEFAULT_SHOW: Kind[] = ['jams', 'roadworks', 'speedchecks'];

/** A jam, roadwork or speed check as the server sends it (internal/traffic.Incident). */
export interface Incident {
  road: string;
  from?: string;
  to?: string;
  lengthKm?: number;
  delayMinutes?: number;
  /** The ANWB's own text, in Dutch. */
  reason?: string;
  hectometre?: number;
  closed?: boolean;
  start?: string;
  stop?: string;
}

/** A fixed camera as the server sends it (internal/traffic.CameraItem). */
export interface Camera {
  kind: 'fixed' | 'section';
  road?: string;
  name?: string;
  maxSpeed?: number;
  hectometre?: number;
  distanceKm?: number;
}

export interface Totals {
  jams: number;
  lengthKm: number;
  delayMinutes: number;
}

/** What one widget's query gets (internal/traffic.Result). */
export interface TrafficResult {
  incidents: boolean;
  jams: Incident[];
  roadworks: Incident[];
  speedChecks: Incident[];
  /** Null until the camera list is loaded. */
  cameras: Camera[] | null;
  cameraCount: number;
  totals?: Totals;
}

/** One line on the screen. */
export interface Row {
  kind: 'jam' | 'roadworks' | 'speedcheck' | 'camera';
  road: string;
  /** "Born → Urmond", "hm 236.1" or a camera's name. */
  place: string;
  /** On the right: "5 km · +11 min", "Closed", "from 21:00". */
  detail: string;
  /** The ANWB's own description, small under the line. */
  note?: string;
  /** A closure or a long delay, marked. */
  severe: boolean;
}

// A jam this long or longer is marked.
const SEVERE_DELAY_MINUTES = 10;

/** The roads a widget watches: distinct capitals, sorted as the server sorts them. */
export function roadsOf(settings: Record<string, unknown> | undefined): string[] {
  const roads = settings?.roads;
  if (!Array.isArray(roads)) return [];
  const names = roads.filter((r): r is string => typeof r === 'string').map((r) => r.trim().toUpperCase());
  return [...new Set(names)].sort();
}

/** The radius in kilometres a widget without roads watches. */
export function radiusOf(settings: Record<string, unknown> | undefined): number {
  return typeof settings?.radius === 'number' ? settings.radius : DEFAULT_RADIUS_KM;
}

/** What a widget shows, in the server's codes. */
export function showOf(settings: Record<string, unknown> | undefined): Kind[] {
  const show = settings?.show;
  if (!Array.isArray(show)) return DEFAULT_SHOW;
  return KINDS.filter((kind) => show.includes(kind));
}

/** The key of the widget's result in the server's data (internal/traffic.Query.Key). */
export function trafficKey(settings: Record<string, unknown> | undefined): string {
  const roads = roadsOf(settings);
  return roads.length > 0 ? `roads:${roads.join(',')}` : `near:${radiusOf(settings)}`;
}

function isResult(v: unknown): v is TrafficResult {
  if (typeof v !== 'object' || v === null) return false;
  const r = v as Record<string, unknown>;
  return typeof r.incidents === 'boolean' && Array.isArray(r.jams) && Array.isArray(r.roadworks) && Array.isArray(r.speedChecks);
}

/** The result under key in the server's value, or undefined when there is none. */
export function trafficResult(value: unknown, key: string): TrafficResult | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const result = (value as Record<string, unknown>)[key];
  return isResult(result) ? result : undefined;
}

/** "Born → Urmond", or one of them when the other is missing or the same. */
function stretch(from: string | undefined, to: string | undefined): string {
  if (from && to && from !== to) return `${from} → ${to}`;
  return from || to || '';
}

function number(value: number, digits: number): string {
  return new Intl.NumberFormat(locale(), { maximumFractionDigits: digits }).format(value);
}

/** "6 km", or "0.8 km" for less than one. */
export function lengthText(km: number): string {
  return t('trafficLength', { km: number(km, km < 1 ? 1 : 0) });
}

/** "hm 236.1". */
export function hectometreText(hm: number): string {
  return t('trafficHectometre', { hm: new Intl.NumberFormat(locale(), { minimumFractionDigits: 1, maximumFractionDigits: 1 }).format(hm) });
}

/** "from 21:00", with the day when it is not today. */
function startText(start: string, now: Date): string | undefined {
  const at = new Date(start);
  if (Number.isNaN(at.getTime()) || at <= now) return undefined;
  const sameDay = at.toDateString() === now.toDateString();
  const format = new Intl.DateTimeFormat(locale(), sameDay ? { hour: '2-digit', minute: '2-digit' } : { weekday: 'short', hour: '2-digit', minute: '2-digit' });
  return t('trafficFrom', { time: format.format(at) });
}

function jamRow(jam: Incident): Row {
  const parts: string[] = [];
  if (jam.lengthKm) parts.push(lengthText(jam.lengthKm));
  if (jam.delayMinutes) parts.push(t('trafficDelay', { minutes: jam.delayMinutes }));
  if (parts.length === 0 && jam.closed) parts.push(t('trafficClosed'));
  return {
    kind: 'jam',
    road: jam.road,
    place: stretch(jam.from, jam.to),
    detail: parts.join(' · '),
    note: jam.reason,
    severe: !!jam.closed || (jam.delayMinutes ?? 0) >= SEVERE_DELAY_MINUTES,
  };
}

function roadworksRow(works: Incident, now: Date): Row {
  const what = t(works.closed ? 'trafficClosed' : 'trafficRoadworks');
  const from = works.start ? startText(works.start, now) : undefined;
  return {
    kind: 'roadworks',
    road: works.road,
    place: stretch(works.from, works.to),
    detail: from ? `${what} · ${from}` : what,
    note: works.reason,
    severe: !!works.closed && !from,
  };
}

function speedCheckRow(check: Incident): Row {
  return {
    kind: 'speedcheck',
    road: check.road,
    place: check.hectometre ? hectometreText(check.hectometre) : stretch(check.from, check.to),
    detail: t('trafficSpeedCheck'),
    severe: false,
  };
}

function cameraRow(camera: Camera): Row {
  const kind = t(camera.kind === 'section' ? 'trafficSection' : 'trafficCamera');
  const place = camera.hectometre ? hectometreText(camera.hectometre) : camera.name && camera.kind === 'section' ? camera.name : '';
  const details: string[] = [];
  if (camera.maxSpeed) details.push(t('trafficSpeed', { speed: camera.maxSpeed }));
  if (camera.distanceKm !== undefined) details.push(lengthText(camera.distanceKm));
  return { kind: 'camera', road: camera.road ?? '', place: place ? `${kind} · ${place}` : kind, detail: details.join(' · '), severe: false };
}

/**
 * The lines to show, at most max, in the order of the kinds: jams first,
 * then roadworks, speed checks and fixed cameras.
 */
export function trafficRows(result: TrafficResult, show: Kind[], max: number, now: Date): Row[] {
  const rows: Row[] = [];
  if (show.includes('jams')) rows.push(...result.jams.map(jamRow));
  if (show.includes('roadworks')) rows.push(...result.roadworks.map((w) => roadworksRow(w, now)));
  if (show.includes('speedchecks')) rows.push(...result.speedChecks.map(speedCheckRow));
  if (show.includes('cameras')) rows.push(...(result.cameras ?? []).map(cameraRow));
  return rows.slice(0, max);
}

/** "Netherlands: 15 jams, 41 km". */
export function totalsText(totals: Totals): string {
  return t('trafficTotals', { jams: totals.jams, km: totals.lengthKm });
}

/** Whether the widget shows something that comes from the ANWB. */
export function showsIncidents(show: Kind[]): boolean {
  return show.some((kind) => kind !== 'cameras');
}

/** The calm line when there is nothing to show. */
export function emptyText(settings: Record<string, unknown> | undefined, show: Kind[]): string {
  const near = roadsOf(settings).length === 0;
  if (show.includes('jams')) return near ? t('trafficNoJamsNear', { km: radiusOf(settings) }) : t('trafficNoJams');
  return near ? t('trafficNothingNear', { km: radiusOf(settings) }) : t('trafficNothing');
}

/** "Fixed cameras on your roads: 3", or near home. */
export function camerasText(settings: Record<string, unknown> | undefined, count: number): string {
  return roadsOf(settings).length === 0 ? t('trafficCamerasNear', { km: radiusOf(settings), count }) : t('trafficCameras', { count });
}

/**
 * What the widget says while it has no result. When other widgets have
 * results and this one, watching a radius, has not, there is no weather
 * location to watch around.
 */
export function trafficWaitingText(entry: Entry | undefined, settings: Record<string, unknown> | undefined): string {
  if (entry?.value !== undefined && !entry.failing && roadsOf(settings).length === 0) return t('trafficNoHome');
  return waitingText(entry, 'traffic');
}
