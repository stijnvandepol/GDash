// Gets a browser display allowed to stream its dashboard. See
// internal/pairing for the server side and docs/api.md for the endpoints.

const POLL_INTERVAL_MS = 3000;
const RETRY_INTERVAL_MS = 15000;

export interface PairingCode {
  code: string;
  expiresAt: string;
}

export type Access =
  | { kind: 'paired' }
  | { kind: 'preview' }
  | { kind: 'offline' }
  | { kind: 'unknown-display' };

export interface PairingHooks {
  /** Shows a code to enter in the web interface, or hides it (null). */
  showCode: (code: PairingCode | null) => void;
  /** Called when the server says this screen is not paired. */
  forgetData: () => void;
}

/**
 * Exchanges a launch token from the URL (a cast keeper opened this page) for
 * a display cookie, and removes the token from the address bar so it does
 * not linger in history. The token works once, so failure is not retried.
 */
export async function useLaunchToken(): Promise<void> {
  const params = new URLSearchParams(location.search);
  const launchToken = params.get('launch');
  if (!launchToken) return;
  params.delete('launch');
  const query = params.toString();
  history.replaceState(null, '', location.pathname + (query ? `?${query}` : ''));
  try {
    await fetch('/api/display-sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ launchToken }),
    });
  } catch {
    // Offline: the page falls back to pairing by code once it reconnects.
  }
}

/**
 * Resolves once this screen may stream displayId: paired, or a logged-in
 * user previewing it. While unpaired it requests a code and waits until an
 * admin enters it. Returns 'offline' when the server cannot be reached, so
 * the caller can keep showing the last known dashboard.
 */
export async function ensureAccess(displayId: string, hooks: PairingHooks): Promise<Access> {
  let state: Response;
  try {
    state = await fetch('/api/display-sessions/current');
  } catch {
    return { kind: 'offline' };
  }
  if (state.ok) {
    const body = (await state.json()) as { displayId?: string };
    if (body.displayId === displayId) return { kind: 'paired' };
  } else if (state.status !== 401) {
    return { kind: 'offline' };
  }
  if (await isLoggedIn()) return { kind: 'preview' };

  hooks.forgetData();
  return pairByCode(displayId, hooks);
}

async function isLoggedIn(): Promise<boolean> {
  try {
    return (await fetch('/api/me')).ok;
  } catch {
    return false;
  }
}

async function pairByCode(displayId: string, hooks: PairingHooks): Promise<Access> {
  for (;;) {
    let response: Response;
    try {
      response = await fetch('/api/pairing-requests', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ displayId }),
      });
    } catch {
      await sleep(RETRY_INTERVAL_MS);
      continue;
    }
    if (response.status === 404) return { kind: 'unknown-display' };
    if (!response.ok) {
      await sleep(RETRY_INTERVAL_MS);
      continue;
    }
    const request = (await response.json()) as PairingCode & { pollToken: string };
    hooks.showCode({ code: request.code, expiresAt: request.expiresAt });
    if (await waitForApproval(request.pollToken)) {
      hooks.showCode(null);
      return { kind: 'paired' };
    }
    // The code expired; ask for a new one.
  }
}

/** Polls until the code is approved (true) or has expired (false). */
async function waitForApproval(pollToken: string): Promise<boolean> {
  for (;;) {
    await sleep(POLL_INTERVAL_MS);
    let response: Response;
    try {
      response = await fetch('/api/display-sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ pollToken }),
      });
    } catch {
      continue;
    }
    if (response.status === 201) return true;
    if (response.status !== 202) return false;
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
