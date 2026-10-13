// The phase of the moon, computed on the screen from the clock alone: the
// instants of the principal phases by Meeus, Astronomical Algorithms (2nd
// ed., 1998), chapter 49, accurate to a few minutes, and the illuminated
// fraction by the low-precision formula of chapter 48.

import type { TextKey } from '../i18n.svelte';

const DAY_MS = 86_400_000;
/** The Julian Day of the Unix epoch. */
const UNIX_EPOCH_JD = 2_440_587.5;
const SYNODIC_MONTH = 29.530588861;
/** The new moon of 6 January 2000 that lunation 0 counts from (Meeus 49.1). */
const LUNATION_ZERO_JDE = 2_451_550.09766;
/**
 * Terrestrial time runs about 69 seconds ahead of the clock (ΔT, 2020s);
 * the phases come out in terrestrial time.
 */
const DELTA_T_DAYS = 69 / 86_400;
const RAD = Math.PI / 180;

/** The principal phases, as the fraction of a lunation after the new moon. */
export const NEW = 0;
export const FIRST_QUARTER = 0.25;
export const FULL = 0.5;
export const LAST_QUARTER = 0.75;
type Principal = typeof NEW | typeof FIRST_QUARTER | typeof FULL | typeof LAST_QUARTER;

export type PhaseName =
  | 'new'
  | 'waxingCrescent'
  | 'firstQuarter'
  | 'waxingGibbous'
  | 'full'
  | 'waningGibbous'
  | 'lastQuarter'
  | 'waningCrescent';

export const PHASE_TEXT: Record<PhaseName, TextKey> = {
  new: 'moonNew',
  waxingCrescent: 'moonWaxingCrescent',
  firstQuarter: 'moonFirstQuarter',
  waxingGibbous: 'moonWaxingGibbous',
  full: 'moonFull',
  waningGibbous: 'moonWaningGibbous',
  lastQuarter: 'moonLastQuarter',
  waningCrescent: 'moonWaningCrescent',
};

const PRINCIPAL_NAMES: [Principal, PhaseName][] = [
  [NEW, 'new'],
  [FIRST_QUARTER, 'firstQuarter'],
  [FULL, 'full'],
  [LAST_QUARTER, 'lastQuarter'],
];

export function julianDay(date: Date): number {
  return date.getTime() / DAY_MS + UNIX_EPOCH_JD;
}

function fromJulianDay(jd: number): Date {
  return new Date((jd - UNIX_EPOCH_JD) * DAY_MS);
}

const sin = (degrees: number) => Math.sin(degrees * RAD);
const cos = (degrees: number) => Math.cos(degrees * RAD);

/** Periodic terms of the new and the full moon (Meeus table 49.A): coefficient, power of E, M, M′, F, Ω. */
type Term = [number, number, number, number, number, number];
const NEW_TERMS: Term[] = [
  [-0.4072, 0, 0, 1, 0, 0],
  [0.17241, 1, 1, 0, 0, 0],
  [0.01608, 0, 0, 2, 0, 0],
  [0.01039, 0, 0, 0, 2, 0],
  [0.00739, 1, -1, 1, 0, 0],
  [-0.00514, 1, 1, 1, 0, 0],
  [0.00208, 2, 2, 0, 0, 0],
  [-0.00111, 0, 0, 1, -2, 0],
  [-0.00057, 0, 0, 1, 2, 0],
  [0.00056, 1, 1, 2, 0, 0],
  [-0.00042, 0, 0, 3, 0, 0],
  [0.00042, 1, 1, 0, 2, 0],
  [0.00038, 1, 1, 0, -2, 0],
  [-0.00024, 1, -1, 2, 0, 0],
  [-0.00017, 0, 0, 0, 0, 1],
  [-0.00007, 0, 2, 1, 0, 0],
  [0.00004, 0, 0, 2, -2, 0],
  [0.00004, 0, 3, 0, 0, 0],
  [0.00003, 0, 1, 1, -2, 0],
  [0.00003, 0, 0, 2, 2, 0],
  [-0.00003, 0, 1, 1, 2, 0],
  [0.00003, 0, -1, 1, 2, 0],
  [-0.00002, 0, -1, 1, -2, 0],
  [-0.00002, 0, 1, 3, 0, 0],
  [0.00002, 0, 0, 4, 0, 0],
];
const FULL_TERMS: Term[] = NEW_TERMS.map((term, i) => {
  const full = [-0.40614, 0.17302, 0.01614, 0.01043, 0.00734, -0.00515, 0.00209][i];
  return full === undefined ? term : [full, ...term.slice(1)] as Term;
});
/** The quarters (Meeus table 49.A, last part). */
const QUARTER_TERMS: Term[] = [
  [-0.62801, 0, 0, 1, 0, 0],
  [0.17172, 1, 1, 0, 0, 0],
  [-0.01183, 1, 1, 1, 0, 0],
  [0.00862, 0, 0, 2, 0, 0],
  [0.00804, 0, 0, 0, 2, 0],
  [0.00454, 1, -1, 1, 0, 0],
  [0.00204, 2, 2, 0, 0, 0],
  [-0.0018, 0, 0, 1, -2, 0],
  [-0.0007, 0, 0, 1, 2, 0],
  [-0.0004, 0, 0, 3, 0, 0],
  [-0.00034, 1, -1, 2, 0, 0],
  [0.00032, 1, 1, 0, 2, 0],
  [0.00032, 1, 1, 0, -2, 0],
  [-0.00028, 2, 2, 1, 0, 0],
  [0.00027, 1, 1, 2, 0, 0],
  [-0.00017, 0, 0, 0, 0, 1],
  [-0.00005, 0, -1, 1, -2, 0],
  [0.00004, 0, 0, 2, 2, 0],
  [-0.00004, 0, 1, 1, 2, 0],
  [0.00004, 0, -2, 1, 0, 0],
  [0.00003, 0, 1, 1, -2, 0],
  [0.00003, 0, 3, 0, 0, 0],
  [0.00002, 0, 0, 2, -2, 0],
  [0.00002, 0, -1, 1, 2, 0],
  [-0.00002, 0, 1, 3, 0, 0],
];
/** The planetary arguments A1 to A14 (constant and change per lunation) and their coefficients. */
const PLANETARY: [number, number, number][] = [
  [299.77, 0.107408, 0.000325],
  [251.88, 0.016321, 0.000165],
  [251.83, 26.651886, 0.000164],
  [349.42, 36.412478, 0.000126],
  [84.66, 18.206239, 0.00011],
  [141.74, 53.303771, 0.000062],
  [207.14, 2.453732, 0.00006],
  [154.84, 7.30686, 0.000056],
  [34.52, 27.261239, 0.000047],
  [207.19, 0.121824, 0.000042],
  [291.34, 1.844379, 0.00004],
  [161.72, 24.198154, 0.000037],
  [239.56, 25.513099, 0.000035],
  [331.55, 3.592518, 0.000023],
];

/**
 * The instant of a principal phase: lunation k (whole, counted from the
 * new moon of 6 January 2000) plus the phase's fraction.
 */
export function phaseInstant(lunation: number, phase: Principal): Date {
  const k = lunation + phase;
  const T = k / 1236.85;
  let jde = LUNATION_ZERO_JDE + SYNODIC_MONTH * k + 0.00015437 * T ** 2 - 0.00000015 * T ** 3 + 0.00000000073 * T ** 4;
  const E = 1 - 0.002516 * T - 0.0000074 * T ** 2;
  const M = 2.5534 + 29.1053567 * k - 0.0000014 * T ** 2 - 0.00000011 * T ** 3;
  const Mp = 201.5643 + 385.81693528 * k + 0.0107582 * T ** 2 + 0.00001238 * T ** 3 - 0.000000058 * T ** 4;
  const F = 160.7108 + 390.67050284 * k - 0.0016118 * T ** 2 - 0.00000227 * T ** 3 + 0.000000011 * T ** 4;
  const omega = 124.7746 - 1.56375588 * k + 0.0020672 * T ** 2 + 0.00000215 * T ** 3;

  const terms = phase === NEW ? NEW_TERMS : phase === FULL ? FULL_TERMS : QUARTER_TERMS;
  for (const [c, e, m, mp, f, o] of terms) jde += c * E ** e * sin(m * M + mp * Mp + f * F + o * omega);
  if (phase === FIRST_QUARTER || phase === LAST_QUARTER) {
    const W = 0.00306 - 0.00038 * E * cos(M) + 0.00026 * cos(Mp) - 0.00002 * cos(Mp - M) + 0.00002 * cos(Mp + M) + 0.00002 * cos(2 * F);
    jde += phase === FIRST_QUARTER ? W : -W;
  }
  PLANETARY.forEach(([a, perLunation, c], i) => {
    // A1 has a small term in T² as well.
    const argument = a + perLunation * k - (i === 0 ? 0.009173 * T ** 2 : 0);
    jde += c * sin(argument);
  });
  return fromJulianDay(jde - DELTA_T_DAYS);
}

/** The first instant of a principal phase after now. */
export function nextPhase(now: Date, phase: Principal): Date {
  let lunation = Math.floor((julianDay(now) - LUNATION_ZERO_JDE) / SYNODIC_MONTH) - 1;
  // The true phase is at most about 14 hours from the mean one, so this
  // takes two or three steps.
  for (;;) {
    const instant = phaseInstant(lunation, phase);
    if (instant.getTime() > now.getTime()) return instant;
    lunation++;
  }
}

/** The illuminated fraction of the moon's disc, 0 to 1 (Meeus 48.4 with the arguments of chapter 47). */
export function illumination(date: Date): number {
  const T = (julianDay(date) + DELTA_T_DAYS - 2_451_545) / 36_525;
  const D = 297.8501921 + 445267.1114034 * T - 0.0018819 * T ** 2 + T ** 3 / 545868 - T ** 4 / 113065000;
  const M = 357.5291092 + 35999.0502909 * T - 0.0001536 * T ** 2 + T ** 3 / 24490000;
  const Mp = 134.9633964 + 477198.8675055 * T + 0.0087414 * T ** 2 + T ** 3 / 69699 - T ** 4 / 14712000;
  const i = 180 - D - 6.289 * sin(Mp) + 2.1 * sin(M) - 1.274 * sin(2 * D - Mp) - 0.658 * sin(2 * D) - 0.214 * sin(2 * Mp) - 0.11 * sin(D);
  return (1 + cos(i)) / 2;
}

export interface MoonState {
  phase: PhaseName;
  /** The illuminated fraction, 0 to 1. */
  illumination: number;
  /** Whether the lit part is growing: the full moon comes before the new one. */
  waxing: boolean;
  nextFull: Date;
  nextNew: Date;
}

function sameLocalDay(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
}

/**
 * The moon at now. A principal phase names the whole local day it falls on,
 * as calendars do ("full moon on Monday"); between them the moon is a
 * crescent or gibbous by whether less or more than half of it is lit.
 */
export function moonAt(now: Date): MoonState {
  const nextFull = nextPhase(now, FULL);
  const nextNew = nextPhase(now, NEW);
  const waxing = nextFull.getTime() < nextNew.getTime();
  const lit = illumination(now);
  const midnight = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const today = PRINCIPAL_NAMES.find(([phase]) => sameLocalDay(nextPhase(midnight, phase), now));
  let phase: PhaseName;
  if (today) phase = today[1];
  else if (waxing) phase = lit < 0.5 ? 'waxingCrescent' : 'waxingGibbous';
  else phase = lit < 0.5 ? 'waningCrescent' : 'waningGibbous';
  return { phase, illumination: lit, waxing, nextFull, nextNew };
}

/**
 * The SVG path of the lit part of a moon of radius r around (cx, cy), seen
 * from the northern hemisphere: lit on the right while waxing. The edge of
 * the lit part is half a circle on the bright side and half an ellipse,
 * whose width follows the lit fraction, through the middle.
 */
export function litPath(fraction: number, waxing: boolean, cx: number, cy: number, r: number): string {
  const lit = Math.min(1, Math.max(0, fraction));
  const top = `${cx} ${cy - r}`;
  const bottom = `${cx} ${cy + r}`;
  const rx = Math.abs(1 - 2 * lit) * r;
  // Clockwise from the top passes the right side; from the bottom, the left.
  const outer = waxing ? 1 : 0;
  // A gibbous moon's edge bulges into the dark side, a crescent's into the lit one.
  const inner = lit > 0.5 ? outer : 1 - outer;
  const round = (n: number) => Math.round(n * 100) / 100;
  return `M${top} A${r} ${r} 0 0 ${outer} ${bottom} A${round(rx)} ${r} 0 0 ${inner} ${top} Z`;
}
