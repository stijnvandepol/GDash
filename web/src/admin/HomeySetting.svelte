<script lang="ts">
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, ApiError, type Homey, type SourceStatus } from './api';
  import { describeStatus } from './status';

  let {
    connection,
    status,
    onchanged,
    onerror,
  }: {
    connection: Homey | null;
    /** Present once a dashboard shows Homey values or who is home. */
    status: SourceStatus | undefined;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let url = $state('');
  let key = $state('');
  let fieldErrors = $state<{ url?: string; key?: string }>({});
  let error = $state('');
  let busy = $state(false);
  // Shows the form over an existing connection, to change the address or key.
  let editing = $state(false);

  async function connect(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    fieldErrors = {};
    try {
      await api.connectHomey(url.trim(), key.trim());
      toast.success('Homey is connected.');
      url = '';
      key = '';
      editing = false;
      await onchanged();
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = { url: err.fieldMessage('url'), key: err.fieldMessage('key') };
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  async function disconnect() {
    const message = 'Homey widgets then show that it is not set up. The API key is deleted.';
    if (!(await ask({ title: 'Disconnect Homey?', message, action: 'Disconnect' }))) return;
    error = '';
    try {
      await api.disconnectHomey();
      toast.success('Homey is disconnected.');
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>Homey</h2>
  <p class="muted">
    Show values from a Homey Pro, such as the temperature or whether a door is open, and who is home. GDash only
    reads and never changes anything. In the Homey web app, under Settings, API Keys, make a key with View devices and
    View zones, and View users for who is home; GDash stores it encrypted and never shows it again.
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
      <p class="small muted">Add a Homey or Who’s home widget to a dashboard to choose what it shows.</p>
    {/if}
  {:else}
    <form onsubmit={connect}>
      <label>
        Address
        <input
          bind:value={url}
          type="url"
          placeholder="http://192.168.1.20"
          autocomplete="off"
          spellcheck="false"
          required
        />
        {#if fieldErrors.url}<span class="field-error">{fieldErrors.url}</span>{/if}
      </label>
      <label>
        API key
        <!-- Not type="password", so browsers do not offer to save it. -->
        <input bind:value={key} class="masked" autocomplete="off" spellcheck="false" required />
        {#if fieldErrors.key}<span class="field-error">{fieldErrors.key}</span>{/if}
      </label>
      <div class="inline">
        <button type="submit" disabled={busy}>{busy ? 'Connecting…' : 'Connect'}</button>
        {#if editing}<button type="button" class="link" onclick={() => (editing = false)}>Cancel</button>{/if}
      </div>
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
