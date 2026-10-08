/** One entry of the audit log, as GET /api/audit-events returns it. */
export interface AuditEvent {
  at: string;
  /** Empty for an anonymous request, such as a failed login. */
  actor: string;
  action: string;
  detail: string;
  ip: string;
}

// The actions internal/api/audit.go records.
const ACTIONS: Record<string, string> = {
  account_created: 'Created the account',
  login: 'Logged in',
  login_failed: 'Failed login',
  logout: 'Logged out',
  password_changed: 'Changed the password',
  password_change_failed: 'Failed to change the password',
  screen_paired: 'Paired a screen with display',
  screens_unpaired: 'Unpaired the screens of display',
  display_created: 'Added display',
  display_updated: 'Changed display',
  display_deleted: 'Removed display',
  dashboard_created: 'Added dashboard',
  dashboard_updated: 'Changed dashboard',
  dashboard_deleted: 'Removed dashboard',
  location_changed: 'Set the location to',
  location_cleared: 'Removed the location',
  language_changed: 'Set the language on the TVs to',
  news_feed_added: 'Added news feed',
  news_feed_removed: 'Removed news feed',
  calendar_added: 'Added calendar',
  calendar_removed: 'Removed calendar',
};

/** "Changed dashboard morning", or the raw action for one this version does not know. */
export function describeAction(event: AuditEvent): string {
  const text = ACTIONS[event.action] ?? event.action;
  if (!event.detail) return text;
  return event.action === 'login_failed' || event.action === 'password_change_failed'
    ? `${text}: ${event.detail}`
    : `${text} ${event.detail}`;
}

/** Whether an event deserves attention, such as a failed login. */
export function isWarning(event: AuditEvent): boolean {
  return event.action.endsWith('_failed');
}
