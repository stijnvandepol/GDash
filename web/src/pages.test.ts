import { describe, expect, it } from 'vitest';
import { msToNextPage, pageAt } from './pages';
import type { Dashboard, Snapshot } from './protocol';

const page = (id: string): Dashboard => ({ id, name: id, theme: 'minimal-dark', columns: 24, rows: 14, widgets: [] });
const own = page('own');
const snapshot = (extra: Partial<Snapshot>): Snapshot =>
  ({ type: 'snapshot', version: 1, serverTime: '', dashboard: own, data: {}, ...extra }) as Snapshot;

describe('pageAt', () => {
  it('shows the dashboard when nothing rotates', () => {
    expect(pageAt(snapshot({}), 0)?.id).toBe('own');
    expect(pageAt(null, 0)).toBeUndefined();
  });

  it('turns the pages on the wall clock', () => {
    const s = snapshot({ pages: [own, page('news'), page('calendar')], pageSeconds: 30 });
    expect(pageAt(s, 0)?.id).toBe('own');
    expect(pageAt(s, 29_999)?.id).toBe('own');
    expect(pageAt(s, 30_000)?.id).toBe('news');
    expect(pageAt(s, 60_000)?.id).toBe('calendar');
    expect(pageAt(s, 90_000)?.id).toBe('own');
  });
});

describe('msToNextPage', () => {
  it('waits until the next boundary', () => {
    expect(msToNextPage(0, 30)).toBe(30_000);
    expect(msToNextPage(29_000, 30)).toBe(1_000);
    expect(msToNextPage(30_000, 30)).toBe(30_000);
  });
});
