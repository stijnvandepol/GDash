// Entry point for browser and kiosk displays (/d/<display-id>).
import '@fontsource-variable/geist';
import { mount } from 'svelte';
import App from './App.svelte';
import { ensureAccess, useLaunchToken } from './pairing';
import { t } from './i18n.svelte';
import { isSnapshot } from './protocol';
import { applySnapshot, display, forgetSnapshot, restoreSnapshot } from './state.svelte';
import { shouldReloadForFrames } from './widgets/webpage';

const OFFLINE_RETRY_MS = 15000;
const FRAME_RELOAD_KEY = 'gdash.frameReloadAt';

const displayId = location.pathname.split('/').filter(Boolean).pop() ?? 'default';

restoreSnapshot(displayId, isSnapshot);
mount(App, { target: document.getElementById('app')! });
document.addEventListener('securitypolicyviolation', reloadForBlockedFrame);
void start();

/**
 * The server lists the origins of this display's web pages in the page's
 * policy when it loads. A page added to the dashboard later, or loaded
 * before this screen was paired, is blocked by the browser; loading the
 * page again lets it through, at most once per FRAME_RELOAD_GAP_MS.
 */
function reloadForBlockedFrame(event: SecurityPolicyViolationEvent): void {
  if (event.effectiveDirective !== 'frame-src') return;
  let last: number | null = null;
  try {
    const stored = Number(sessionStorage.getItem(FRAME_RELOAD_KEY));
    last = stored > 0 ? stored : null;
    if (!shouldReloadForFrames(last, Date.now())) return;
    sessionStorage.setItem(FRAME_RELOAD_KEY, String(Date.now()));
  } catch {
    // Without storage there is no guard against reloading in a loop.
    return;
  }
  location.reload();
}

/** Gets access to the display, then streams it. Runs again when access is lost. */
async function start(): Promise<void> {
  await useLaunchToken();
  for (;;) {
    const access = await ensureAccess(displayId, {
      showCode: (code) => (display.pairingCode = code),
      forgetData: () => forgetSnapshot(displayId),
    });
    if (access.kind === 'unknown-display') {
      display.problem = t('noDisplay', { id: displayId });
      return;
    }
    if (access.kind !== 'offline') break;
    display.connection = 'offline';
    await new Promise((resolve) => setTimeout(resolve, OFFLINE_RETRY_MS));
  }
  connect();
}

function connect(): void {
  // EventSource reconnects on its own after network errors, using the retry
  // interval the server sends. An error status (401 after unpairing, or a
  // proxy error) closes it for good; then access is checked again.
  const events = new EventSource(`/api/displays/${encodeURIComponent(displayId)}/events`);
  events.addEventListener('open', () => {
    display.connection = 'live';
  });
  events.addEventListener('error', () => {
    display.connection = 'offline';
    if (events.readyState === EventSource.CLOSED) {
      events.close();
      void start();
    }
  });
  events.addEventListener('snapshot', (event) => {
    let parsed: unknown;
    try {
      parsed = JSON.parse((event as MessageEvent<string>).data);
    } catch {
      console.warn('gdash: ignored a snapshot that is not valid JSON');
      return;
    }
    if (isSnapshot(parsed)) {
      applySnapshot(displayId, parsed);
    } else {
      console.warn('gdash: ignored a snapshot with an unexpected shape');
    }
  });
}
