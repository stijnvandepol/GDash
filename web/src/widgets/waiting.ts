import type { Entry } from '../protocol';

/**
 * What a widget shows while it has nothing to show. The server sends no
 * entry for a source that is not set up, and an entry without a value while
 * the first fetch runs or fails.
 */
export function waitingText(entry: Entry | undefined, what: string, notSetUp: string): string {
  if (entry === undefined) return notSetUp;
  if (entry.failing) return `Could not load the ${what}`;
  return `Loading ${what}…`;
}
