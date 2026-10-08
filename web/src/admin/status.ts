import type { SourceStatus } from './api';

const MINUTE = 60_000;

/** "Updated 4 minutes ago", or why there is no fresh data. */
export function describeStatus(status: SourceStatus | undefined, now = Date.now()): string {
  if (!status) return 'Not fetched yet.';
  const updated = status.updatedAt ? Date.parse(status.updatedAt) : NaN;
  if (Number.isNaN(updated)) {
    return status.failing ? 'Could not be fetched yet. Check the address.' : 'Fetching…';
  }
  const ago = describeAge(Math.max(0, Math.round((now - updated) / MINUTE)));
  return status.failing ? `The last update failed; showing data from ${ago}.` : `Updated ${ago}.`;
}

function describeAge(minutes: number): string {
  if (minutes === 0) return 'just now';
  if (minutes === 1) return '1 minute ago';
  if (minutes < 120) return `${minutes} minutes ago`;
  return `${Math.round(minutes / 60)} hours ago`;
}
