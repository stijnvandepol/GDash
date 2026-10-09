// Entry point for the dashboard preview (/preview/<dashboard-id>): the same
// screen as a TV, fed by polling the preview endpoint instead of a display
// stream. The editor frames this page and sends it the unsaved layout.
import '@fontsource-variable/atkinson-hyperlegible-next';
import { mount } from 'svelte';
import App from './App.svelte';
import { t } from './i18n.svelte';
import { isPreviewMessage, previewOutcome, previewRequest, type PreviewDraft, type PreviewOutcome } from './previewApi';
import { display, showSnapshot } from './state.svelte';

const POLL_MS = 5000;
const DEFAULT_TEXT_SCALE = 100;

const dashboardId = location.pathname.split('/').filter(Boolean).pop() ?? 'default';

// A page opened on its own can still choose the text size: /preview/x?textScale=150.
// The server refuses a size a display cannot have.
let textScale = Number(new URLSearchParams(location.search).get('textScale')) || DEFAULT_TEXT_SCALE;
let draft: PreviewDraft | null = null;
let timer: ReturnType<typeof setTimeout> | undefined;
// Only the newest request may change the screen, however the answers arrive.
let latest = 0;

mount(App, { target: document.getElementById('app')! });
window.addEventListener('message', onMessage);
// In the editor's frame the first message says what to show, so the stored
// layout never flashes before an unsaved one.
if (window.parent === window) void poll();

function onMessage(event: MessageEvent): void {
  // Only the editor that framed this page, from the same origin.
  if (event.origin !== location.origin || event.source !== window.parent) return;
  if (!isPreviewMessage(event.data)) return;
  textScale = event.data.textScale;
  draft = event.data.draft;
  void poll();
}

async function poll(): Promise<void> {
  clearTimeout(timer);
  await refresh();
  timer = setTimeout(() => void poll(), POLL_MS);
}

async function refresh(): Promise<void> {
  const run = ++latest;
  const { url, init } = previewRequest(dashboardId, textScale, draft);
  let outcome: PreviewOutcome;
  try {
    const response = await fetch(url, init);
    outcome = previewOutcome(response.status, await response.json().catch(() => null));
  } catch {
    outcome = { kind: 'error' };
  }
  if (run === latest) show(outcome);
}

function show(outcome: PreviewOutcome): void {
  display.connection = outcome.kind === 'error' ? 'offline' : 'live';
  switch (outcome.kind) {
    case 'snapshot':
      display.problem = null;
      showSnapshot(outcome.snapshot);
      break;
    case 'login':
      display.problem = t('previewLogin');
      break;
    case 'missing':
      display.problem = t('previewMissing', { id: dashboardId });
      break;
    case 'invalid':
      display.problem = t('previewInvalid', { reason: outcome.reason });
      break;
    case 'error':
      // A passing network problem keeps what is on screen.
      if (!display.snapshot) display.problem = t('previewFailed');
      break;
  }
}
