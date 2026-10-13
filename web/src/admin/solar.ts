import type { Solar, SolarArray } from './api';

/** internal/solar allows two groups of panels, such as both sides of a roof. */
export const MAX_ARRAYS = 2;

export const SOLAR_SOURCES: { value: Solar['source']; label: string }[] = [
  { value: 'open-meteo', label: 'Open-Meteo (computed by GDash, the default)' },
  { value: 'forecast.solar', label: 'Forecast.Solar (12 free requests an hour)' },
];

/** A south-facing roof of 35°, the common case in the Netherlands. */
export function newArray(): SolarArray {
  return { tilt: 35, azimuth: 0, kwp: 4 };
}

/** The form for panels not set up yet: at the weather location, from Open-Meteo. */
export function newSolar(): Solar {
  return { location: null, arrays: [newArray()], performanceRatio: 0.85, source: 'open-meteo' };
}

/** A copy to edit, so the form never changes what the page shows. */
export function copySolar(panels: Solar): Solar {
  return { ...panels, location: panels.location ? { ...panels.location } : null, arrays: panels.arrays.map((a) => ({ ...a })) };
}

const DIRECTIONS = ['north', 'north-east', 'east', 'south-east', 'south', 'south-west', 'west', 'north-west'];

/** The compass direction of an azimuth in GDash's convention (0 south, -90 east, 90 west), such as "south-east". */
export function direction(azimuth: number): string {
  // Turned into a bearing from north: 0 north, 90 east, 180 south.
  const bearing = (((azimuth + 180) % 360) + 360) % 360;
  return DIRECTIONS[Math.round(bearing / 45) % 8]!;
}

/** "4.2 kWp facing south-east at 35°" for one group of panels. */
export function describeArray(a: SolarArray): string {
  return `${a.kwp} kWp facing ${direction(a.azimuth)} at ${a.tilt}°`;
}
