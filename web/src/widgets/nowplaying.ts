/** What one Cast device plays, as the server sends it (internal/nowplaying.Item). */
export interface Playing {
  /** The device ID, the key the server sent it under. */
  id: string;
  name: string;
  group: boolean;
  app: string;
  title: string;
  artist: string;
  album: string;
  /** The GDash address of the cover art, or empty. */
  image: string;
  state: 'playing' | 'paused' | 'buffering';
  /** Seconds, read at `at`. */
  position: number;
  /** Seconds; 0 for a live stream. */
  duration: number;
  /** When position was read, in milliseconds since the epoch on the server's clock. */
  at: number;
}

const STATES = new Set(['playing', 'paused', 'buffering']);
// The addresses the server gives covers; anything else in a snapshot is
// ignored rather than put in an image.
const ARTWORK = /^\/api\/now-playing\/artwork\/[0-9a-f]{32}$/;

const text = (v: unknown) => (typeof v === 'string' ? v : '');
const seconds = (v: unknown) => (typeof v === 'number' && Number.isFinite(v) && v > 0 ? v : 0);

function readItem(id: string, v: unknown): Playing | undefined {
  if (typeof v !== 'object' || v === null) return undefined;
  const d = v as Record<string, unknown>;
  const state = text(d.state);
  const at = Date.parse(text(d.at));
  if (!STATES.has(state) || Number.isNaN(at) || (text(d.title) === '' && text(d.artist) === '')) return undefined;
  return {
    id,
    name: text(d.name),
    group: d.group === true,
    app: text(d.app),
    title: text(d.title),
    artist: text(d.artist),
    album: text(d.album),
    image: ARTWORK.test(text(d.image)) ? text(d.image) : '',
    state: state as Playing['state'],
    position: seconds(d.position),
    duration: seconds(d.duration),
    at,
  };
}

/**
 * Reads the server's value: device ID to what it plays. Anything that is
 * not what a device plays is left out, so a broken value shows nothing
 * instead of breaking the widget.
 */
export function playingItems(value: unknown): Playing[] | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const out: Playing[] = [];
  for (const [id, item] of Object.entries(value)) {
    const read = readItem(id, item);
    if (read) out.push(read);
  }
  return out;
}

/** The device IDs a widget follows, or an empty list for any device. */
export function followedDevices(settings: Record<string, unknown> | undefined): string[] {
  const v = settings?.devices;
  return Array.isArray(v) ? v.filter((id): id is string => typeof id === 'string') : [];
}

const sameMedia = (a: Playing, b: Playing) => a.app === b.app && a.title === b.title && a.artist === b.artist;

/**
 * What a widget shows, the most recently active first: what plays before
 * what is paused, newer before older. A speaker that plays as part of a
 * playing group is left out, so a song on the whole house shows once.
 */
export function shownItems(items: Playing[], devices: string[]): Playing[] {
  const mine = devices.length === 0 ? items : items.filter((i) => devices.includes(i.id));
  const groups = mine.filter((i) => i.group);
  const shown = mine.filter((i) => i.group || !groups.some((g) => sameMedia(g, i)));
  const rank = (i: Playing) => (i.state === 'paused' ? 1 : 0);
  return shown.sort((a, b) => rank(a) - rank(b) || b.at - a.at || a.name.localeCompare(b.name));
}

/** Where a track is at `now` (server clock, ms): it moves on while playing, and stops at its end. */
export function positionAt(item: Playing, now: number): number {
  const moved = item.state === 'playing' ? Math.max(0, (now - item.at) / 1000) : 0;
  const position = item.position + moved;
  return item.duration > 0 ? Math.min(position, item.duration) : position;
}

/** How far a track is, from 0 to 1, or undefined for a live stream. */
export function progressAt(item: Playing, now: number): number | undefined {
  if (item.duration <= 0) return undefined;
  return Math.min(1, positionAt(item, now) / item.duration);
}

/** A time in a track: "0:07", "3:25" or "1:02:03". */
export function formatTime(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const hours = Math.floor(s / 3600);
  const minutes = Math.floor((s % 3600) / 60);
  const rest = String(s % 60).padStart(2, '0');
  return hours > 0 ? `${hours}:${String(minutes).padStart(2, '0')}:${rest}` : `${minutes}:${rest}`;
}
