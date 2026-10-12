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

  it('describes failed setups, rejected tokens and resets', () => {
    expect(describeAction({ ...base, actor: '', action: 'setup_failed', detail: 'wrong setup token' })).toBe(
      'Setup attempt failed: wrong setup token',
    );
    expect(describeAction({ ...base, actor: '', action: 'api_token_rejected' })).toBe('An automation used an invalid API token');
    expect(describeAction({ ...base, actor: 'command line', action: 'password_reset', detail: 'stijn' })).toBe(
      'Reset the password from the command line for stijn',
    );
  });

  it('describes two-step login', () => {
    expect(describeAction({ ...base, detail: 'with a two-step code' })).toBe('Logged in with a two-step code');
    expect(describeAction({ ...base, action: 'login_code_failed', detail: 'right password, wrong code' })).toBe(
      'Failed two-step login: right password, wrong code',
    );
    expect(describeAction({ ...base, action: 'recovery_code_used', detail: '9 left' })).toBe('Logged in with a recovery code, 9 left');
    expect(describeAction({ ...base, action: 'two_step_enabled', detail: 'other sessions logged out' })).toBe(
      'Turned on two-step login, other sessions logged out',
    );
    expect(describeAction({ ...base, action: 'two_step_disabled' })).toBe('Turned off two-step login');
    expect(describeAction({ ...base, action: 'two_step_change_failed', detail: 'wrong code' })).toBe(
      'Failed to change two-step login: wrong code',
    );
    expect(describeAction({ ...base, actor: 'command line', action: 'two_step_reset', detail: 'stijn' })).toBe(
      'Turned off two-step login from the command line for stijn',
    );
  });

  it('shows an action from a newer server as it is', () => {
    expect(describeAction({ ...base, action: 'backup_restored' })).toBe('backup_restored');
  });
});

describe('isWarning', () => {
  it('flags failed attempts and rejected tokens only', () => {
    expect(isWarning({ ...base, action: 'login_failed' })).toBe(true);
    expect(isWarning({ ...base, action: 'password_change_failed' })).toBe(true);
    expect(isWarning({ ...base, action: 'setup_failed' })).toBe(true);
    expect(isWarning({ ...base, action: 'api_token_rejected' })).toBe(true);
    expect(isWarning(base)).toBe(false);
  });

  it('flags wrong two-step codes and every use of a recovery code', () => {
    expect(isWarning({ ...base, action: 'login_code_failed' })).toBe(true);
    expect(isWarning({ ...base, action: 'two_step_change_failed' })).toBe(true);
    expect(isWarning({ ...base, action: 'recovery_code_used' })).toBe(true);
    expect(isWarning({ ...base, action: 'two_step_enabled' })).toBe(false);
  });
});
