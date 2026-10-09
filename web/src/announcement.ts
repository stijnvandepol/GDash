import type { Announcement } from './protocol';

// A device clock is trusted when it is within this of the server's. A
// snapshot that a screen gets when it connects was made up to a minute
// earlier, so its serverTime lags a little behind and says nothing about
// the clock. Only a clock that is really wrong, such as a TV that never
// synced, is corrected.
const CLOCK_TOLERANCE_MS = 90_000;

/**
 * How far the server's clock is ahead of this screen's, in milliseconds,
 * for a snapshot that has just arrived; 0 when the clocks agree closely.
 */
export function clockOffset(serverTime: string, localNowMs: number): number {
  const server = Date.parse(serverTime);
  if (Number.isNaN(server)) return 0;
  const difference = server - localNowMs;
  return Math.abs(difference) < CLOCK_TOLERANCE_MS ? 0 : difference;
}

/**
 * Checks the shape of an announcement from a snapshot. Messages arrive over
 * the network and, on Cast, from any sender on the LAN.
 */
function readAnnouncement(v: unknown): (Announcement & { expiresMs: number }) | undefined {
  if (typeof v !== 'object' || v === null) return undefined;
  const a = v as Record<string, unknown>;
  const expiresMs = typeof a.expiresAt === 'string' ? Date.parse(a.expiresAt) : NaN;
  if (typeof a.message !== 'string' || a.message === '' || Number.isNaN(expiresMs)) return undefined;
  const title = typeof a.title === 'string' && a.title !== '' ? a.title : undefined;
  return { title, message: a.message, expiresAt: a.expiresAt as string, expiresMs };
}

/**
 * Milliseconds until the announcement expires on the server's clock, or null
 * when there is none. Zero or less means it has expired.
 */
export function msUntilExpiry(announcement: unknown, offsetMs: number, localNowMs: number): number | null {
  const a = readAnnouncement(announcement);
  return a ? a.expiresMs - (localNowMs + offsetMs) : null;
}

/** The announcement to show now, or undefined when there is none or it has expired. */
export function visibleAnnouncement(announcement: unknown, offsetMs: number, localNowMs: number): Announcement | undefined {
  const a = readAnnouncement(announcement);
  if (!a || a.expiresMs - (localNowMs + offsetMs) <= 0) return undefined;
  return { title: a.title, message: a.message, expiresAt: a.expiresAt };
}
