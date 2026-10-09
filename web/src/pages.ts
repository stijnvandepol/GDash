import type { Dashboard, Snapshot } from './protocol';

/**
 * The page of a rotating display at a moment. Pages turn on the wall clock,
 * not on when the screen loaded, so every screen of a display shows the
 * same page.
 */
export function pageAt(snapshot: Snapshot | null, nowMs: number): Dashboard | undefined {
  if (!snapshot) return undefined;
  const pages = snapshot.pages ?? [];
  const seconds = snapshot.pageSeconds ?? 0;
  if (pages.length < 2 || seconds <= 0) return snapshot.dashboard;
  return pages[Math.floor(nowMs / 1000 / seconds) % pages.length];
}

/** Milliseconds until the next page turns. */
export function msToNextPage(nowMs: number, seconds: number): number {
  const period = seconds * 1000;
  return period - (nowMs % period);
}
