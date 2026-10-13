<script lang="ts">
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, ApiError, type Homey, type SourceStatus } from './api';
  import { describeStatus } from './status';
  import { describeSnapshot, isSnapshotLate } from './homey';
  import HomeyPushInstructions from './HomeyPushInstructions.svelte';

  let {
    connection,
    status,
    onchanged,
    onmanagetokens,
    onerror,
  }: {
    connection: Homey | null;
    /**
     * For a connection: present once a dashboard shows Homey values or who
     * is home. For a Homey that sends its data: when the last one arrived.
     */
    status: SourceStatus | undefined;
    onchanged: () => Promise<void>;
    /** Opens the API tokens, which the HomeyScript needs. */
    onmanagetokens: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  let url = $state('');
  let key = $state('');
  let fieldErrors = $state<{ url?: string; key?: string }>({});
  let error = $state('');
  let busy = $state(false);
  // Shows the form over an existing setup, to change the address, key or mode.
  let editing = $state(false);
  // svelte-ignore state_referenced_locally
  let mode = $state<'local' | 'push'>(connection?.mode ?? 'local');
  let showSteps = $state(false);

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

  async function usePush() {
    busy = true;
    error = '';
    try {
      await api.useHomeyPush();
      toast.success('GDash now waits for your Homey to send its data.');
      editing = false;
      showSteps = true;
      await onchanged();
    } catch (err) {
      error = onerror(err);
    } finally {
      busy = false;
    }
  }

  async function disconnect() {
    const message =
      connection?.mode === 'push'
        ? 'Homey widgets then show that it is not set up. Remove the Flow on your Homey and revoke its API token too.'
        : 'Homey widgets then show that it is not set up. The API key is deleted.';
    if (!(await ask({ title: 'Disconnect Homey?', message, action: 'Disconnect' }))) return;
    error = '';
    try {
      await api.disconnectHomey();
      toast.success('Homey is disconnected.');
      showSteps = false;
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
    reads and never changes anything. A Homey Pro (2023), Homey Pro mini or Homey Self-Hosted Server gives GDash an API
    key; an older Homey Pro sends its data itself with a HomeyScript.
  </p>
  {#if connection && !editing}
    <ul class="list">
      <li>
        <div class="grow">
          {#if connection.mode === 'push'}
            <strong>Your Homey sends its data</strong>
            <span class="muted small">HomeyScript, every minute</span>
          {:else}
            <strong>Connected</strong>
            <span class="muted address">{connection.url}</span>
          {/if}
          {#if connection.unreadable}
            <span class="field-error">This connection cannot be read on this server. Connect again.</span>
          {/if}
        </div>
        {#if connection.mode === 'push'}
          <button class="link" onclick={() => (showSteps = !showSteps)} aria-expanded={showSteps}>How to send</button>
        {/if}
        <button class="link" onclick={() => ((editing = true), (mode = connection.mode))}>Change</button>
        <button class="link danger" onclick={disconnect}>Disconnect</button>
      </li>
    </ul>
    {#if connection.mode === 'push'}
      <p class="small" class:late={isSnapshotLate(status?.updatedAt)}>{describeSnapshot(status?.updatedAt)}</p>
      {#if showSteps}
        <HomeyPushInstructions onclose={() => (showSteps = false)} {onmanagetokens} {onerror} />
      {/if}
    {:else if status}
      <p class="small" class:error={status.failing}>{describeStatus(status)}</p>
    {:else}
      <p class="small muted">Add a Homey or Who’s home widget to a dashboard to choose what it shows.</p>
    {/if}
  {:else}
    <fieldset class="mode">
      <legend>Which Homey do you have?</legend>
      <label class="check"><input type="radio" name="homey-mode" value="local" bind:group={mode} /> Homey Pro (2023) or Homey Pro mini: address and API key</label>
      <label class="check"><input type="radio" name="homey-mode" value="push" bind:group={mode} /> Homey Pro (2016–2019): your Homey sends its data (HomeyScript)</label>
    </fieldset>
    {#if mode === 'push'}
      <p class="small muted">
        This Homey has no API keys, so GDash cannot ask it. A HomeyScript that a Flow runs every minute sends every
        device and who is home to GDash instead; GDash shows the script next.
      </p>
      <div class="inline">
        <button type="button" disabled={busy} onclick={usePush}>{busy ? 'Saving…' : 'Use HomeyScript'}</button>
        {#if editing}<button type="button" class="link" onclick={() => (editing = false)}>Cancel</button>{/if}
      </div>
    {:else}
      <p class="small muted">
        In the Homey web app, under Settings, API Keys, make a key with View devices and View zones, and View users for
        who is home; GDash stores it encrypted and never shows it again.
      </p>
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
  .mode {
    display: grid;
    gap: var(--s-2);
    margin: 0;
    padding: 0;
    border: 0;
  }
  .mode legend {
    padding: 0;
    margin-bottom: var(--s-2);
    font-size: var(--fs-sm);
    font-weight: 550;
    color: var(--text-soft);
  }
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 400;
  }
  .late {
    color: var(--warning);
  }
</style>
