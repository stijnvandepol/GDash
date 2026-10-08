import type { Snapshot } from './protocol';

export type ConnectionState = 'connecting' | 'live' | 'offline';

const STORAGE_PREFIX = 'daycast.snapshot.';

/**
 * Display state shared by all components. The last snapshot is kept in
 * localStorage, so a display that reloads while the server is down still
 * shows the dashboard instead of a blank screen.
 */
export const display = $state<{ snapshot: Snapshot | null; connection: ConnectionState }>({
  snapshot: null,
  connection: 'connecting',
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

export function applySnapshot(displayId: string, snapshot: Snapshot): void {
  if (display.snapshot && snapshot.version === display.snapshot.version) return;
  display.snapshot = snapshot;
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
