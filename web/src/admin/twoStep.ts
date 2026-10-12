// Helpers for two-step login: the login answer, the setup's QR code and
// secret, and the recovery codes file.
import type { PendingLogin, User } from './types';

/** Whether a login answer waits for a two-step code instead of being a session. */
export function isPendingLogin(answer: User | PendingLogin): answer is PendingLogin {
  return 'pending' in answer;
}

/**
 * The QR code as an image address. An <img> shows the server's SVG without
 * running anything in it, and the page's CSP allows data: images.
 */
export function qrImageSource(svg: string): string {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

/** The secret in groups of four, as apps show it, so it is easier to type. */
export function groupSecret(secret: string): string {
  return secret.match(/.{1,4}/g)?.join(' ') ?? '';
}

/** The text file with the recovery codes, to print or keep in a password manager. */
export function recoveryCodesText(username: string, codes: string[], made: Date): string {
  return [
    `GDash recovery codes for ${username}`,
    `Made on ${made.toISOString().slice(0, 10)}.`,
    '',
    'Each code logs in once instead of a code from your authenticator app.',
    'Keep them somewhere safe, away from your phone.',
    '',
    ...codes,
    '',
  ].join('\n');
}
