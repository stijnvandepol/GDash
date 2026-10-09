import { describe, expect, it } from 'vitest';
import { describeAction, isWarning, type AuditEvent } from './audit';

const base: AuditEvent = { at: '2026-10-09T08:00:00Z', actor: 'stijn', action: 'login', detail: '', ip: '192.168.1.20' };

describe('describeAction', () => {
  it('names the action and what it was about', () => {
    expect(describeAction(base)).toBe('Logged in');
    expect(describeAction({ ...base, action: 'dashboard_updated', detail: 'morning' })).toBe('Changed dashboard morning');
    expect(describeAction({ ...base, action: 'login_failed', actor: '', detail: 'wrong username or password' })).toBe(
      'Failed login: wrong username or password',
    );
  });

  it('names the API token that was made or revoked', () => {
    expect(describeAction({ ...base, action: 'api_token_created', detail: 'Home Assistant' })).toBe('Made API token Home Assistant');
    expect(describeAction({ ...base, action: 'api_token_revoked', detail: 'Home Assistant' })).toBe('Revoked API token Home Assistant');
  });

  it('leaves out file names of photos', () => {
    expect(describeAction({ ...base, action: 'photo_added', detail: '17ffba802f212d49844c20cc3dca1edb.png' })).toBe('Added a photo');
  });

  it('shows an action from a newer server as it is', () => {
    expect(describeAction({ ...base, action: 'backup_restored' })).toBe('backup_restored');
  });
});

describe('isWarning', () => {
  it('flags failed attempts only', () => {
    expect(isWarning({ ...base, action: 'login_failed' })).toBe(true);
    expect(isWarning({ ...base, action: 'password_change_failed' })).toBe(true);
    expect(isWarning(base)).toBe(false);
  });
});
