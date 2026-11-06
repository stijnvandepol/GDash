import { describe, expect, it } from 'vitest';
import { isSnapshot } from './protocol';

const board = { columns: 24, rows: 12, widgets: [{ id: 'a', type: 'clock', x: 0, y: 0, w: 6, h: 4 }] };

describe('isSnapshot', () => {
  it('accepts a snapshot with pages that are dashboards', () => {
    expect(isSnapshot({ type: 'snapshot', version: 1, dashboard: board, data: {} })).toBe(true);
    expect(isSnapshot({ type: 'snapshot', version: 1, dashboard: board, pages: [board], pageSeconds: 30, data: {} })).toBe(true);
  });

  it('rejects pages that are not dashboards, as any sender on the LAN may try', () => {
    expect(isSnapshot({ type: 'snapshot', version: 1, dashboard: board, pages: [1], data: {} })).toBe(false);
    expect(isSnapshot({ type: 'snapshot', version: 1, dashboard: board, pages: [{ columns: 24 }], data: {} })).toBe(false);
    expect(isSnapshot({ type: 'snapshot', version: 1, dashboard: { ...board, widgets: [{}] }, data: {} })).toBe(false);
  });
});
