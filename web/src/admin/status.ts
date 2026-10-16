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

export function describeAge(minutes: number): string {
  if (minutes === 0) return 'just now';
  if (minutes === 1) return '1 minute ago';
  if (minutes < 120) return `${minutes} minutes ago`;
  return `${Math.round(minutes / 60)} hours ago`;
}

const TIME = new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit' });

/**
 * What a display's TV is doing, in plain words, for the display list. tv
 * names the TV; backAt is when the display's hours start again, for a TV
 * that GDash leaves alone outside them. Without a status the keeper has not
 * reported yet.
 */
export function describeCast(status: CastStatus | undefined, tv = 'the TV', backAt = ''): string {
  if (!status) return `Connecting to ${tv}…`;
  const since = TIME.format(new Date(status.since));
  switch (status.state) {
    case 'showing':
      return `On ${tv} since ${since}`;
    case 'standby':
      return `${capitalize(tv)} is asleep (standby) since ${since}`;
    case 'other_app':
      return `Another app is on ${tv}; GDash comes back when it stops`;
    case 'yielded':
      return `Someone is using ${tv}; GDash waits until it is switched off`;
    case 'resting':
      return backAt ? `Outside its hours; GDash comes back at ${backAt}` : 'Outside its hours';
    case 'unreachable':
      return status.reason === 'not_found'
        ? `Cannot find ${tv} on the network. Is it switched on and on the same network as GDash?`
        : `${capitalize(tv)} does not answer; GDash keeps trying`;
    default:
      return `Connecting to ${tv}…`;
  }
}

/**
 * Whether GDash keeps the dashboard in front of the TV's screensaver, in
 * plain words: when it last restarted the receiver, and when the page on
 * the TV last said it was hidden. Empty without a status.
 */
export function describeScreensaver(status: CastStatus | undefined): string[] {
  if (!status) return [];
  const lines: string[] = [];
  if (status.restartedAt) lines.push(`Restarted against the screensaver at ${TIME.format(new Date(status.restartedAt))}`);
  if (status.hiddenAt) lines.push(`The screen was last hidden at ${TIME.format(new Date(status.hiddenAt))}`);
  return lines;
}

/** The tone of a TV's status light: ok, warn, bad, or empty for neutral. */
export function castTone(status: CastStatus | undefined): 'ok' | 'warn' | 'bad' | '' {
  switch (status?.state) {
    case 'showing':
      return 'ok';
    case 'other_app':
    case 'yielded':
      return 'warn';
    case 'unreachable':
      return 'bad';
    default:
      return '';
  }
}

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}
