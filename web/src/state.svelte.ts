import { clockOffset } from './announcement';
import type { PairingCode } from './pairing';
import type { Snapshot } from './protocol';

export type ConnectionState = 'connecting' | 'live' | 'offline';

const STORAGE_PREFIX = 'gdash.snapshot.';

/**
 * Display state shared by all components. The last snapshot is kept in
 * localStorage, so a display that reloads while the server is down still
 * shows the dashboard instead of a blank screen.
 */
export const display = $state<{
  snapshot: Snapshot | null;
  connection: ConnectionState;
  /** The code to show while this screen waits to be paired. */
  pairingCode: PairingCode | null;
  /** A problem the screen cannot solve itself, shown instead of a dashboard. */
  problem: string | null;
  /** How far the server's clock is ahead of this screen's, for expiring announcements. */
  serverOffsetMs: number;
  /**
   * Whether Web page widgets show their page. False on the Cast receiver:
   * any device on the network can send it a snapshot, so it never opens an
   * address from one.
   */
  framesAllowed: boolean;
  /** Milliseconds from loading the page until a dashboard was first on screen. */
  firstRenderMs: number | null;
}>({
  snapshot: null,
  connection: 'connecting',
  pairingCode: null,
  problem: null,
  serverOffsetMs: 0,
  framesAllowed: true,
  firstRenderMs: null,
});

/** Minute-resolution clock shared by widgets. One timer for the whole page. */
export const clock = $state({ now: new Date() });

function scheduleNextMinute(): void {
  const msToNextMinute = 60_000 - (Date.now() % 60_000);
  setTimeout(() => {
    clock.now = new Date();
    scheduleNextMinute();
  }, msToNextMinute + 50);
}
scheduleNextMinute();

/**
 * Shows a snapshot, unless the screen already shows that version. Returns
 * whether the screen changed.
 */
export function showSnapshot(snapshot: Snapshot): boolean {
  if (display.snapshot && snapshot.version === display.snapshot.version) return false;
  display.snapshot = snapshot;
  display.serverOffsetMs = clockOffset(snapshot.serverTime, Date.now());
  return true;
}

export function applySnapshot(displayId: string, snapshot: Snapshot): void {
  if (!showSnapshot(snapshot)) return;
  try {
    localStorage.setItem(STORAGE_PREFIX + displayId, JSON.stringify(snapshot));
  } catch {
    // Storage can be full or blocked; the display still works, it just
    // cannot restore after a reload without the server.
  }
}

export function restoreSnapshot(displayId: string, isValid: (v: unknown) => v is Snapshot): void {
  try {
    const stored = localStorage.getItem(STORAGE_PREFIX + displayId);
    if (!stored) return;
    const parsed: unknown = JSON.parse(stored);
    if (isValid(parsed)) display.snapshot = parsed;
  } catch {
    // A corrupt or inaccessible entry is ignored; the server sends a fresh
    // snapshot as soon as it is reachable.
  }
}

/**
 * Removes the stored snapshot, for a screen the server says is not paired:
 * an unpaired screen must not keep showing a dashboard it no longer may see.
 */
export function forgetSnapshot(displayId: string): void {
  display.snapshot = null;
  try {
    localStorage.removeItem(STORAGE_PREFIX + displayId);
  } catch {
    // Nothing stored or storage blocked: nothing to remove.
  }
}
