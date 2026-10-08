import type { CastStatus, SourceStatus } from './api';

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

const TIME = new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit' });

/** What a display's TV is doing, for the display list. */
export function describeCast(status: CastStatus | undefined): string {
  if (!status) return '';
  const since = TIME.format(new Date(status.since));
  switch (status.state) {
    case 'showing':
      return `On screen since ${since}`;
    case 'standby':
      return `TV in standby since ${since}`;
    case 'other_app':
      return 'Another app is on the TV; Daycast waits until it stops';
    case 'resting':
      return 'Outside active hours';
    case 'unreachable':
      return 'Cannot reach the TV; retrying';
    default:
      return 'Connecting to the TV…';
  }
}
