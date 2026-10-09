<script lang="ts">
  import { api, ApiError, type HomeAssistant, type SourceStatus } from './api';
  import { describeStatus } from './status';

  let {
    connection,
    status,
    onchanged,
    onerror,
  }: {
    connection: HomeAssistant | null;
    /** Present once a dashboard shows Home Assistant entities. */
    status: SourceStatus | undefined;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let url = $state('');
  let token = $state('');
  let fieldErrors = $state<{ url?: string; token?: string }>({});
  let error = $state('');
  let busy = $state(false);
  // Shows the form over an existing connection, to change the address or token.
  let editing = $state(false);

  async function connect(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    fieldErrors = {};
    try {
      await api.connectHomeAssistant(url.trim(), token.trim());
      url = '';
      token = '';
      editing = false;
      await onchanged();
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = { url: err.fieldMessage('url'), token: err.fieldMessage('token') };
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  async function disconnect() {
    if (!confirm('Disconnect Home Assistant? Home Assistant widgets then show that it is not set up.')) return;
    error = '';
    try {
      await api.disconnectHomeAssistant();
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>Home Assistant</h2>
  <p class="muted">
    Show sensors and switches from Home Assistant, such as the temperature inside or whether a door is open. GDash
    only reads states and never changes anything. Make a long-lived access token in Home Assistant under your profile,
    Security; GDash stores it encrypted and never shows it again.
  </p>
  {#if connection && !editing}
    <ul class="list">
      <li>
        <div class="grow">
          <strong>Connected</strong>
          <span class="muted address">{connection.url}</span>
          {#if connection.unreadable}
            <span class="field-error">This connection cannot be read on this server. Connect again.</span>
          {/if}
        </div>
        <button class="link" onclick={() => (editing = true)}>Change</button>
        <button class="link danger" onclick={disconnect}>Disconnect</button>
      </li>
    </ul>
    {#if status}
      <p class="small" class:error={status.failing}>{describeStatus(status)}</p>
    {:else}
      <p class="small muted">Add a Home Assistant widget to a dashboard to choose what it shows.</p>
    {/if}
  {:else}
    <form onsubmit={connect}>
      <label>
        Address
        <input
          bind:value={url}
          type="url"
          placeholder="http://192.168.1.10:8123"
          autocomplete="off"
          spellcheck="false"
          required
        />
        {#if fieldErrors.url}<span class="field-error">{fieldErrors.url}</span>{/if}
      </label>
      <label>
        Long-lived access token
        <!-- Not type="password", so browsers do not offer to save it. -->
        <input bind:value={token} class="masked" autocomplete="off" spellcheck="false" required />
        {#if fieldErrors.token}<span class="field-error">{fieldErrors.token}</span>{/if}
      </label>
      <button type="submit" disabled={busy}>{busy ? 'Connecting…' : 'Connect'}</button>
      {#if editing}<button type="button" class="link" onclick={() => (editing = false)}>Cancel</button>{/if}
    </form>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .address {
    display: block;
    overflow-wrap: anywhere;
  }
  .masked {
    -webkit-text-security: disc;
  }
</style>
