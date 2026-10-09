import { describe, expect, it } from 'vitest';
import type { ApiToken } from './api';
import { describeLastUse } from './apiTokens';

const NOW = Date.parse('2026-10-09T12:00:00Z');
const token = (lastUsedAt: string | null): ApiToken => ({ id: 1, name: 'Home Assistant', createdAt: '2026-10-01T08:00:00Z', lastUsedAt });

describe('describeLastUse', () => {
  it('says when a token was never used', () => {
    expect(describeLastUse(token(null), NOW)).toBe('Never used');
    expect(describeLastUse(token('not a date'), NOW)).toBe('Never used');
  });

  it('says how long ago a token was used', () => {
    expect(describeLastUse(token('2026-10-09T12:00:00Z'), NOW)).toBe('Last used just now');
    expect(describeLastUse(token('2026-10-09T11:56:00Z'), NOW)).toBe('Last used 4 minutes ago');
    expect(describeLastUse(token('2026-10-09T08:00:00Z'), NOW)).toBe('Last used 4 hours ago');
  });

  it('does not count a use in the future against the clock', () => {
    expect(describeLastUse(token('2026-10-09T12:05:00Z'), NOW)).toBe('Last used just now');
  });
});
