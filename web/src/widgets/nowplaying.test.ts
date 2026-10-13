import { describe, expect, it } from 'vitest';
import { followedDevices, formatTime, playingItems, positionAt, progressAt, shownItems, type Playing } from './nowplaying';

const AT = '2026-10-13T08:00:00Z';
const atMs = Date.parse(AT);

function item(over: Partial<Playing> = {}): Playing {
  return {
    id: 'a',
    name: 'Kitchen',
    group: false,
    app: 'Spotify',
    title: 'One More Time',
    artist: 'Daft Punk',
    album: 'Discovery',
    image: '',
    state: 'playing',
    position: 60,
    duration: 320,
    at: atMs,
    ...over,
  };
}

describe('formatTime', () => {
  it('shows minutes and seconds, and hours for long media', () => {
    expect(formatTime(0)).toBe('0:00');
    expect(formatTime(7.9)).toBe('0:07');
    expect(formatTime(205)).toBe('3:25');
    expect(formatTime(3723)).toBe('1:02:03');
    expect(formatTime(-5)).toBe('0:00');
  });
});

describe('positionAt and progressAt', () => {
  it('moves a playing track on from where it was read', () => {
    expect(positionAt(item(), atMs + 15_000)).toBe(75);
    expect(progressAt(item({ position: 160 }), atMs)).toBe(0.5);
  });

  it('keeps a paused track where it is', () => {
    expect(positionAt(item({ state: 'paused' }), atMs + 15_000)).toBe(60);
  });

  it('stops at the end of a track', () => {
    expect(positionAt(item(), atMs + 3_600_000)).toBe(320);
    expect(progressAt(item(), atMs + 3_600_000)).toBe(1);
  });

  it('never goes back when the screen clock is behind', () => {
    expect(positionAt(item(), atMs - 5_000)).toBe(60);
  });

  it('has no progress for a live stream', () => {
    expect(progressAt(item({ duration: 0 }), atMs)).toBeUndefined();
    expect(positionAt(item({ duration: 0 }), atMs + 10_000)).toBe(70);
  });
});

describe('playingItems', () => {
  it('reads what the server sends and leaves out what it cannot show', () => {
    const items = playingItems({
      speaker: { name: 'Kitchen', app: 'Spotify', title: 'Song', state: 'playing', position: 3, duration: 200, at: AT, image: '/api/now-playing/artwork/0123456789abcdef0123456789abcdef' },
      odd: { name: 'Hall', title: 'Song', state: 'idle', at: AT },
      untitled: { name: 'Hall', state: 'playing', at: AT },
      badImage: { name: 'Bed', title: 'Song', state: 'paused', at: AT, image: 'https://evil.example/x.jpg' },
      broken: 'nope',
    });
    expect(items?.map((i) => i.id)).toEqual(['speaker', 'badImage']);
    expect(items?.[0]?.image).toBe('/api/now-playing/artwork/0123456789abcdef0123456789abcdef');
    expect(items?.[1]?.image).toBe('');
    expect(playingItems(undefined)).toBeUndefined();
    expect(playingItems([])).toBeUndefined();
  });
});

describe('shownItems', () => {
  it('puts the most recently started playing device first and paused ones last', () => {
    const items = [
      item({ id: 'old', at: atMs - 60_000 }),
      item({ id: 'paused', state: 'paused', at: atMs + 60_000 }),
      item({ id: 'new', at: atMs }),
    ];
    expect(shownItems(items, []).map((i) => i.id)).toEqual(['new', 'old', 'paused']);
  });

  it('shows only the devices a widget follows', () => {
    expect(shownItems([item({ id: 'a' }), item({ id: 'b' })], ['b']).map((i) => i.id)).toEqual(['b']);
  });

  it('shows a song on a speaker group once', () => {
    const items = [item({ id: 'group', name: 'Whole house', group: true }), item({ id: 'kitchen' }), item({ id: 'bed', name: 'Bedroom', title: 'Other song', at: atMs - 1000 })];
    expect(shownItems(items, []).map((i) => i.id)).toEqual(['group', 'bed']);
  });
});

describe('followedDevices', () => {
  it('reads the devices setting, empty for any device', () => {
    expect(followedDevices({ devices: ['a', 3, 'b'] })).toEqual(['a', 'b']);
    expect(followedDevices({})).toEqual([]);
    expect(followedDevices(undefined)).toEqual([]);
  });
});
