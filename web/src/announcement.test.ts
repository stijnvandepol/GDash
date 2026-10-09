import { describe, expect, it } from 'vitest';
import { clockOffset, msUntilExpiry, visibleAnnouncement } from './announcement';

const EXPIRES = '2026-10-09T19:01:00Z';
const expiresMs = Date.parse(EXPIRES);
const announcement = { title: 'Laundry', message: 'The washing machine is done', expiresAt: EXPIRES };

describe('clockOffset', () => {
  const server = '2026-10-09T19:00:00Z';
  const serverMs = Date.parse(server);

  it('trusts a clock that is close, also when the snapshot is up to a minute old', () => {
    expect(clockOffset(server, serverMs)).toBe(0);
    expect(clockOffset(server, serverMs + 60_000)).toBe(0);
    expect(clockOffset(server, serverMs - 30_000)).toBe(0);
  });

  it('corrects a clock that is far off', () => {
    expect(clockOffset(server, serverMs - 3_600_000)).toBe(3_600_000);
    expect(clockOffset(server, serverMs + 600_000)).toBe(-600_000);
  });

  it('ignores a server time it cannot read', () => {
    expect(clockOffset('', serverMs)).toBe(0);
    expect(clockOffset('yesterday', serverMs)).toBe(0);
  });
});

describe('visibleAnnouncement', () => {
  it('shows the message until its expiry', () => {
    expect(visibleAnnouncement(announcement, 0, expiresMs - 1)).toEqual(announcement);
    expect(visibleAnnouncement(announcement, 0, expiresMs)).toBeUndefined();
    expect(visibleAnnouncement(announcement, 0, expiresMs + 60_000)).toBeUndefined();
  });

  it('follows the server clock when the screen is off', () => {
    // The screen runs an hour behind: it shows the message for as long as the server does.
    const behind = 3_600_000;
    expect(visibleAnnouncement(announcement, behind, expiresMs - behind - 1_000)).toBeDefined();
    expect(visibleAnnouncement(announcement, behind, expiresMs - behind)).toBeUndefined();
  });

  it('shows no title when there is none', () => {
    expect(visibleAnnouncement({ message: 'Dinner', expiresAt: EXPIRES }, 0, 0)?.title).toBeUndefined();
    expect(visibleAnnouncement({ title: '', message: 'Dinner', expiresAt: EXPIRES }, 0, 0)?.title).toBeUndefined();
  });

  it('ignores anything that is not an announcement', () => {
    for (const bad of [
      undefined,
      null,
      'hello',
      42,
      {},
      { message: '', expiresAt: EXPIRES },
      { message: 7, expiresAt: EXPIRES },
      { message: 'x' },
      { message: 'x', expiresAt: 'soon' },
      { message: 'x', expiresAt: 1_800_000_000_000 },
    ]) {
      expect(visibleAnnouncement(bad, 0, 0)).toBeUndefined();
      expect(msUntilExpiry(bad, 0, 0)).toBeNull();
    }
  });

  it('keeps the text as it is, for rendering as plain text', () => {
    const markup = '<img src=x onerror=alert(1)> & "quotes"';
    expect(visibleAnnouncement({ message: markup, title: markup, expiresAt: EXPIRES }, 0, 0)).toEqual({
      message: markup,
      title: markup,
      expiresAt: EXPIRES,
    });
  });
});

describe('msUntilExpiry', () => {
  it('counts down to the expiry on the server clock', () => {
    expect(msUntilExpiry(announcement, 0, expiresMs - 5_000)).toBe(5_000);
    expect(msUntilExpiry(announcement, 2_000, expiresMs - 5_000)).toBe(3_000);
    expect(msUntilExpiry(announcement, 0, expiresMs + 1_000)).toBe(-1_000);
  });

  it('is null without an announcement', () => {
    expect(msUntilExpiry(undefined, 0, 0)).toBeNull();
  });
});
