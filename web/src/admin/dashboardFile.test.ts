import { describe, expect, it } from 'vitest';
import { freeId, fromFile, toFile } from './dashboardFile';
import type { Dashboard } from './api';

const evening: Dashboard = {
  id: 'evening',
  name: 'Evening',
  theme: 'night',
  widgets: [{ id: 'clock', type: 'clock', x: 0, y: 0, w: 6, h: 4 }],
};

describe('dashboard files', () => {
  it('round-trips name, theme and widgets', () => {
    const back = fromFile(toFile(evening));
    expect(back).toEqual({ name: 'Evening', theme: 'night', widgets: [{ id: 'clock', type: 'clock', x: 0, y: 0, w: 6, h: 4 }] });
  });

  it('reads files exported by Daycast, its former name', () => {
    const old = toFile(evening).replace('nookboard-dashboard', 'daycast-dashboard');
    expect(fromFile(old).name).toBe('Evening');
  });

  it('rejects other files with a clear message', () => {
    expect(() => fromFile('not json')).toThrow('not a Nookboard dashboard file');
    expect(() => fromFile('{"name":"x","widgets":[]}')).toThrow('not a Nookboard dashboard file');
    expect(() => fromFile('{"format":"nookboard-dashboard","version":2,"name":"x","widgets":[]}')).toThrow('newer Nookboard');
    expect(() => fromFile('{"format":"nookboard-dashboard","version":1,"name":"x"}')).toThrow('damaged');
    expect(() => fromFile('"' + 'x'.repeat(70_000) + '"')).toThrow('too large');
  });
});

describe('freeId', () => {
  it('numbers a name that is taken', () => {
    expect(freeId('Evening', [])).toBe('evening');
    expect(freeId('Evening', ['evening', 'evening-2'])).toBe('evening-3');
    expect(freeId('!!!', [])).toBe('dashboard');
  });

  it('stays within the 32 characters an id may have', () => {
    const long = 'a'.repeat(40);
    expect(freeId(long, ['a'.repeat(32)])).toBe('a'.repeat(30) + '-2');
  });
});
