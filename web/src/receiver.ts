// Entry point for the Cast receiver. The GDash server is the Cast sender
// and pushes snapshots over a custom namespace, so this page never connects
// to the server itself. That avoids mixed content: the receiver is served
// over HTTPS, the server usually runs on plain HTTP in the home network.
import '@fontsource-variable/geist';
import { mount } from 'svelte';
import App from './App.svelte';
import { CAST_NAMESPACE, isSnapshot } from './protocol';
import { applySnapshot, display, restoreSnapshot } from './state.svelte';

const STORAGE_ID = 'cast';

// Snapshots arrive from any sender on the network (see below), so a Web
// page widget here never opens the address one names.
display.framesAllowed = false;
restoreSnapshot(STORAGE_ID, isSnapshot);

const context = cast.framework.CastReceiverContext.getInstance();
const options = new cast.framework.CastReceiverOptions();
// GDash shows no media; without this the device closes the receiver
// when nothing plays.
options.disableIdleTimeout = true;
options.skipPlayersLoad = true;
options.customNamespaces = { [CAST_NAMESPACE]: cast.framework.system.MessageType.JSON };

// Cast does not authenticate senders: any device on the network can send
// on this namespace. The receiver therefore only takes snapshots, shown as
// plain text, and nothing that changes where the screen goes.
context.addCustomMessageListener(CAST_NAMESPACE, (event) => {
  const message = event.data;
  if (isSnapshot(message)) {
    display.connection = 'live';
    applySnapshot(STORAGE_ID, message);
  }
});

// Ask the sender for the current state as soon as it connects. The sender
// also pushes it unasked, but a message sent before this page was ready
// would otherwise be lost.
context.addEventListener(cast.framework.system.EventType.SENDER_CONNECTED, (event) => {
  context.sendCustomMessage(CAST_NAMESPACE, event.senderId, { type: 'ready' });
});
context.addEventListener(cast.framework.system.EventType.SENDER_DISCONNECTED, () => {
  if (context.getSenders().length === 0) display.connection = 'offline';
});

context.start(options);
mount(App, { target: document.getElementById('app')! });
