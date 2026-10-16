// Entry point for browser and kiosk displays (/d/<display-id>).
import '@fontsource-variable/geist';
import { mount } from 'svelte';
import App from './App.svelte';
import { ensureAccess, useLaunchToken } from './pairing';
import { t } from './i18n.svelte';
import { isSnapshot } from './protocol';
import { applySnapshot, display, forgetSnapshot, restoreSnapshot } from './state.svelte';
import { shouldReloadForFrames } from './widgets/webpage';
import { compat, detectFeatures, startCompat } from './compat';
import { buildReport, createErrorLog, createVisibilityLog, rejectionMessage, scriptSource, sendReport, type ReportReason } from './screenReport';
import { stageFor } from './stage';

const OFFLINE_RETRY_MS = 15000;
const FRAME_RELOAD_KEY = 'gdash.frameReloadAt';
// The screen reports when it has drawn the dashboard and after every
// reconnect, at most once per REPORT_GAP_MS; the server limits it too.
const REPORT_GAP_MS = 60_000;
// A page that is hidden or shown again says so at once, so the cast keeper
// leaves an app opened over the dashboard, such as Netflix, alone. A change
// within VISIBILITY_GAP_MS of the last such report waits for the gap, and
// then the state of that moment is sent.
const VISIBILITY_GAP_MS = 30_000;
const RENDER_WAIT_MS = 30_000;
const FONT_WAIT_MS = 10_000;
const POLL_MS = 500;

const displayId = location.pathname.split('/').filter(Boolean).pop() ?? 'default';

const errors = createErrorLog();
window.addEventListener('error', (event) => errors.add(event.message, scriptSource(event.filename, event.lineno, event.colno)));
window.addEventListener('unhandledrejection', (event) => errors.add(rejectionMessage(event.reason), ''));
const visibility = createVisibilityLog();
document.addEventListener('visibilitychange', () => {
  const hidden = document.visibilityState === 'hidden';
  visibility.add(hidden ? 'hidden' : 'visible', new Date());
  reportVisibility();
});
window.addEventListener('blur', () => visibility.add('blur', new Date()));
window.addEventListener('focus', () => visibility.add('focus', new Date()));
let lastReportAt = -Infinity;
// The page loads on screen; the keeper assumes so after every launch.
let reportedVisibility: 'hidden' | 'visible' = 'visible';
let lastVisibilityAt = -Infinity;
let visibilityTimer: ReturnType<typeof setTimeout> | undefined;

startCompat();
restoreSnapshot(displayId, isSnapshot);
mount(App, { target: document.getElementById('app')! });
document.addEventListener('securitypolicyviolation', reloadForBlockedFrame);
void start();
// A page that loads behind an app says so at once.
reportVisibility();

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
    void reportScreen();
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

const delay = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/** Waits until the dashboard is on screen, or RENDER_WAIT_MS; a sleeping display draws none. */
async function rendered(): Promise<void> {
  for (let waited = 0; display.firstRenderMs === null && waited < RENDER_WAIT_MS; waited += POLL_MS) await delay(POLL_MS);
}

/** Tells the server what this screen's browser can do; see screenReport.ts. */
async function reportScreen(): Promise<void> {
  if (Date.now() - lastReportAt < REPORT_GAP_MS) return;
  lastReportAt = Date.now();
  await rendered();
  await Promise.race([document.fonts.ready, delay(FONT_WAIT_MS)]);
  await sendReport(displayId, measure('connected'));
}

/** Tells the server that the page was hidden, such as by an app, or is on screen again. */
function reportVisibility(): void {
  const state = document.visibilityState === 'hidden' ? 'hidden' : 'visible';
  if (visibilityTimer !== undefined || state === reportedVisibility) return;
  const wait = lastVisibilityAt + VISIBILITY_GAP_MS - Date.now();
  if (wait > 0) {
    visibilityTimer = setTimeout(() => {
      visibilityTimer = undefined;
      reportVisibility();
    }, wait);
    return;
  }
  reportedVisibility = state;
  lastVisibilityAt = Date.now();
  void sendReport(displayId, measure(state));
}

function measure(reason: ReportReason) {
  return buildReport({
    reason,
    userAgent: navigator.userAgent,
    width: innerWidth,
    height: innerHeight,
    devicePixelRatio,
    stageScale: stageFor(innerWidth, innerHeight, display.snapshot?.textScale).scale,
    features: detectFeatures(),
    compat: compat.active,
    fontsLoaded: document.fonts.status === 'loaded',
    firstRenderMs: display.firstRenderMs,
    errors: errors.list(),
    visibility: visibility.list(),
  });
}
