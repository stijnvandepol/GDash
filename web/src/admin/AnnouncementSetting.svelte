<script lang="ts">
  import { api, ApiError } from './api';

  let { onerror }: { onerror: (err: unknown) => string } = $props();

  // The server accepts these (internal/announcement).
  const MAX_MESSAGE = 280;
  const MIN_SECONDS = 5;
  const MAX_SECONDS = 3600;
  const DEFAULT_SECONDS = 60;

  let message = $state('');
  let seconds = $state(DEFAULT_SECONDS);
  let fieldErrors = $state<{ message?: string; seconds?: string; displays?: string }>({});
  let error = $state('');
  let sent = $state('');
  let busy = $state(false);

  async function send(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = sent = '';
    fieldErrors = {};
    try {
      const result = await api.announce(message.trim(), seconds);
      const count = result.displays.length;
      sent = `Sent to ${count} ${count === 1 ? 'display' : 'displays'}.`;
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = {
          message: err.fieldMessage('message'),
          seconds: err.fieldMessage('seconds'),
          displays: err.fieldMessage('displays'),
        };
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }
</script>

<section class="card">
  <h2>Send a test announcement</h2>
  <p class="muted">
    Shows a message over the dashboard of every display, and in the display's active hours only. Automations send the
    same with an API token, see the guide.
  </p>
  <form onsubmit={send}>
    <label>
      Message
      <input bind:value={message} maxlength={MAX_MESSAGE} placeholder="The washing machine is done" autocomplete="off" required />
      {#if fieldErrors.message}<span class="field-error">{fieldErrors.message}</span>{/if}
    </label>
    <label>
      Seconds on screen
      <input type="number" bind:value={seconds} min={MIN_SECONDS} max={MAX_SECONDS} step="1" required />
      {#if fieldErrors.seconds}<span class="field-error">{fieldErrors.seconds}</span>{/if}
    </label>
    {#if fieldErrors.displays}<span class="field-error">{fieldErrors.displays}</span>{/if}
    <button type="submit" disabled={busy}>Send</button>
  </form>
  {#if sent}<p class="muted">{sent}</p>{/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>
