import type { ApiToken } from './api';
import { describeAge } from './status';

const MINUTE = 60_000;

/** "Last used 4 minutes ago", or that the token was never used. */
export function describeLastUse(token: ApiToken, now = Date.now()): string {
  const used = token.lastUsedAt ? Date.parse(token.lastUsedAt) : NaN;
  if (Number.isNaN(used)) return 'Never used';
  return `Last used ${describeAge(Math.max(0, Math.round((now - used) / MINUTE)))}`;
}
