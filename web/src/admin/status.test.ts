import { describe, expect, it } from 'vitest';
import { castTone, describeCast, describeScreensaver, describeStatus } from './status';

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

  it('leads with what the TV is doing, by its name', () => {
    expect(describeCast({ state: 'showing', since }, 'Tv Keuken')).toBe('On Tv Keuken since 10:31');
    expect(describeCast({ state: 'standby', since }, 'Tv Keuken')).toBe('Tv Keuken is asleep (standby) since 10:31');
    expect(describeCast({ state: 'other_app', since }, 'Tv Keuken')).toBe(
      'Another app is on Tv Keuken; GDash comes back when it stops',
    );
    expect(describeCast({ state: 'yielded', since }, 'Tv Keuken')).toBe(
      'Someone is using Tv Keuken; GDash waits until it is switched off',
    );
    expect(describeCast({ state: 'connecting', since }, 'Tv Keuken')).toBe('Connecting to Tv Keuken…');
  });

  it('says when GDash comes back outside the hours', () => {
    expect(describeCast({ state: 'resting', since }, 'Tv Keuken', '07:00')).toBe('Outside its hours; GDash comes back at 07:00');
    expect(describeCast({ state: 'resting', since }, 'Tv Keuken')).toBe('Outside its hours');
  });

  it('says why the TV cannot be reached', () => {
    expect(describeCast({ state: 'unreachable', since, reason: 'not_found', problem: 'x' })).toBe(
      'Cannot find the TV on the network. Is it switched on and on the same network as GDash?',
    );
    expect(describeCast({ state: 'unreachable', since, reason: 'no_answer', problem: 'i/o timeout' })).toBe(
      'The TV does not answer; GDash keeps trying',
    );
  });

  it('is connecting before the keeper first reports', () => {
    expect(describeCast(undefined, 'Tv Keuken')).toBe('Connecting to Tv Keuken…');
  });
});

describe('castTone', () => {
  const since = '2026-10-09T08:31:00Z';

  it('lights the status by how well it goes', () => {
    expect(castTone({ state: 'showing', since })).toBe('ok');
    expect(castTone({ state: 'other_app', since })).toBe('warn');
    expect(castTone({ state: 'yielded', since })).toBe('warn');
    expect(castTone({ state: 'unreachable', since })).toBe('bad');
    expect(castTone({ state: 'standby', since })).toBe('');
    expect(castTone(undefined)).toBe('');
  });
});

describe('describeScreensaver', () => {
  // TZ is Europe/Amsterdam in the test run.
  const since = '2026-10-14T08:51:00Z';

  it('says when GDash last restarted the receiver, and when the screen was hidden', () => {
    expect(describeScreensaver({ state: 'showing', since, restartedAt: '2026-10-14T09:52:00Z', restartReason: 'periodic' })).toEqual([
      'Restarted against the screensaver at 11:52',
    ]);
    expect(
      describeScreensaver({ state: 'yielded', since, restartedAt: '2026-10-14T09:52:00Z', restartReason: 'periodic', hiddenAt: '2026-10-14T09:58:55Z' }),
    ).toEqual(['Restarted against the screensaver at 11:52', 'The screen was last hidden at 11:58']);
  });

  it('says nothing before the first restart', () => {
    expect(describeScreensaver({ state: 'showing', since })).toEqual([]);
    expect(describeScreensaver(undefined)).toEqual([]);
  });
});
