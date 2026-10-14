import { describe, expect, it } from 'vitest';
import { summarisePickups } from './waste';

describe('summarisePickups', () => {
  it('lists the recognised kinds in the order of the widget, with their first pickup and count', () => {
    const { kinds, others } = summarisePickups([
      { kind: 'pmd', title: 'PMD', date: '2026-10-16' },
      { kind: 'gft', title: 'GFT', date: '2026-10-27' },
      { kind: 'gft', title: 'GFT', date: '2026-10-13' },
      { kind: 'tree', title: 'Kerstbomen', date: '2027-01-08' },
    ]);
    expect(kinds.map((k) => [k.kind, k.first, k.count])).toEqual([
      ['gft', '2026-10-13', 2],
      ['pmd', '2026-10-16', 1],
      ['tree', '2027-01-08', 1],
    ]);
    expect(kinds[0]!.name).toBe('GFT (organic waste)');
    expect(others).toEqual([]);
  });

  it('names each unrecognised title once', () => {
    const { kinds, others } = summarisePickups([
      { kind: 'other', title: 'Grofvuil op afspraak', date: '2026-10-22' },
      { kind: 'other', title: 'Grofvuil op afspraak', date: '2026-11-22' },
      { kind: 'other', title: 'Chemokar', date: '2026-11-02' },
    ]);
    expect(kinds).toEqual([]);
    expect(others).toEqual(['Grofvuil op afspraak', 'Chemokar']);
  });

  it('finds nothing in nothing', () => {
    expect(summarisePickups([])).toEqual({ kinds: [], others: [] });
  });
});
