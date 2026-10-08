import { ApiError } from './api';

/**
 * The message to show for a failed request. A 401 means the session ended:
 * then onSessionEnded sends the user to the login screen and the message is
 * empty.
 */
export function errorMessage(err: unknown, onSessionEnded: () => void): string {
  if (err instanceof ApiError && err.status === 401) {
    onSessionEnded();
    return '';
  }
  return err instanceof Error ? err.message : 'Something went wrong.';
}
