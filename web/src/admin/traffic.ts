import type { TrafficInfo } from './types';

// The server's bounds (internal/traffic).
export const MAX_ROADS = 10;
const ROAD = /^[AN][1-9][0-9]{0,2}$/;

const TIME = new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit' });

/** How the requests to the ANWB go, in plain words. */
export function anwbStatusText(anwb: TrafficInfo['anwb']): string {
  const code = anwb.httpStatus ? ` (HTTP ${anwb.httpStatus})` : '';
  switch (anwb.state) {
    case 'ok':
      return 'The ANWB answers. Asked at most once every 3 minutes.';
    case 'failing': {
      const next = anwb.nextAttemptAt ? ` GDash asks again at ${TIME.format(new Date(anwb.nextAttemptAt))}.` : '';
      return `The last request failed${code}; screens keep the last traffic.${next}`;
    }
    case 'stopped':
      return `Stopped: the ANWB refused several requests in a row${code}.`;
    default:
      return 'Not asked yet: a Traffic widget on a dashboard starts it.';
  }
}

/** The roads typed in the editor ("a2, A50 n279"), and what is not a road. */
export function parseRoads(text: string): { roads: string[]; invalid: string[] } {
  const words = text
    .split(/[\s,;]+/)
    .map((w) => w.trim().toUpperCase())
    .filter(Boolean);
  const roads = [...new Set(words.filter((w) => ROAD.test(w)))];
  return { roads, invalid: words.filter((w) => !ROAD.test(w)) };
}

/** The kinds a Traffic widget can show, with their names in the editor. */
export const TRAFFIC_KINDS: [string, string][] = [
  ['jams', 'Jams'],
  ['roadworks', 'Roadworks and closures'],
  ['speedchecks', 'Mobile speed checks (ANWB)'],
  ['cameras', 'Fixed cameras (OpenStreetMap)'],
  ['totals', 'National totals'],
];
