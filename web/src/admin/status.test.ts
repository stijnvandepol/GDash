import { describe, expect, it } from 'vitest';
import { describeCast, describeStatus } from './status';

const now = Date.parse('2026-10-09T10:00:00Z');

describe('describeStatus', () => {
  it('reports a source that was not fetched', () => {
    expect(describeStatus(undefined, now)).toBe('Not fetched yet.');
    expect(describeStatus({ failing: false }, now)).toBe('Fetching…');
    expect(describeStatus({ failing: true }, now)).toBe('Could not be fetched yet. Check the address.');
  });

  it('says how long ago the data arrived', () => {
    expect(describeStatus({ updatedAt: '2026-10-09T10:00:20Z', failing: false }, now)).toBe('Updated just now.');
    expect(describeStatus({ updatedAt: '2026-10-09T09:59:00Z', failing: false }, now)).toBe('Updated 1 minute ago.');
    expect(describeStatus({ updatedAt: '2026-10-09T09:15:00Z', failing: false }, now)).toBe('Updated 45 minutes ago.');
    expect(describeStatus({ updatedAt: '2026-10-09T05:00:00Z', failing: false }, now)).toBe('Updated 5 hours ago.');
  });

  it('warns that a failing source shows older data', () => {
    expect(describeStatus({ updatedAt: '2026-10-09T09:30:00Z', failing: true }, now)).toBe(
      'The last update failed; showing data from 30 minutes ago.',
    );
  });
});

describe('describeCast', () => {
  // TZ is Europe/Amsterdam in the test run, so 08:31 UTC shows as 10:31.
  const since = '2026-10-09T08:31:00Z';

  it('describes each state', () => {
    expect(describeCast({ state: 'showing', since })).toBe('On screen since 10:31');
    expect(describeCast({ state: 'standby', since })).toBe('TV in standby since 10:31');
    expect(describeCast({ state: 'other_app', since })).toBe('Another app is on the TV; Nookboard waits until it stops');
    expect(describeCast({ state: 'resting', since })).toBe('Outside active hours');
    expect(describeCast({ state: 'unreachable', since, problem: 'i/o timeout' })).toBe('Cannot reach the TV; retrying');
    expect(describeCast({ state: 'connecting', since })).toBe('Connecting to the TV…');
  });

  it('says nothing for a display without a Cast device', () => {
    expect(describeCast(undefined)).toBe('');
  });
});
