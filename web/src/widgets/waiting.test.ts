import { describe, expect, it } from 'vitest';
import { waitingText } from './waiting';

const entry = { fetchedAt: '0001-01-01T00:00:00Z', staleAfter: '0001-01-01T00:00:00Z', failing: false };

describe('waitingText', () => {
  it('says what to set up when the server sends no entry', () => {
    expect(waitingText(undefined, 'weather', 'No location set.')).toBe('No location set.');
  });

  it('says loading while the first fetch runs', () => {
    expect(waitingText(entry, 'weather', 'No location set.')).toBe('Loading weather…');
  });

  it('says the fetch failed', () => {
    expect(waitingText({ ...entry, failing: true }, 'weather', 'No location set.')).toBe('Could not load the weather');
  });
});
