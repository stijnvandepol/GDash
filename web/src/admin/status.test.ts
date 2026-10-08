import { describe, expect, it } from 'vitest';
import { describeStatus } from './status';

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
