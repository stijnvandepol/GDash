// Entry point for browser and kiosk displays (/d/<display-id>).
import { mount } from 'svelte';
import App from './App.svelte';
import { isSnapshot } from './protocol';
import { applySnapshot, display, restoreSnapshot } from './state.svelte';

const displayId = location.pathname.split('/').filter(Boolean).pop() ?? 'default';

restoreSnapshot(displayId, isSnapshot);

// EventSource reconnects on its own, using the retry interval the server
// sends, so there is no reconnect loop here.
const events = new EventSource(`/api/displays/${encodeURIComponent(displayId)}/events`);
events.addEventListener('open', () => {
  display.connection = 'live';
});
events.addEventListener('error', () => {
  display.connection = 'offline';
});
events.addEventListener('snapshot', (event) => {
  let parsed: unknown;
  try {
    parsed = JSON.parse((event as MessageEvent<string>).data);
  } catch {
    console.warn('daycast: ignored a snapshot that is not valid JSON');
    return;
  }
  if (isSnapshot(parsed)) {
    applySnapshot(displayId, parsed);
  } else {
    console.warn('daycast: ignored a snapshot with an unexpected shape');
  }
});

mount(App, { target: document.getElementById('app')! });
