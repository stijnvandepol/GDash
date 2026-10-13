<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, ApiError, type KnmiInfo } from './api';
  import { describeStatus } from './status';

  // It loads and refreshes its own state; onchanged tells the page that
  // shows it, so its card follows.
  let { onchanged, onerror }: { onchanged?: () => Promise<void>; onerror: (err: unknown) => string } = $props();

  const REFRESH_MS = 5_000;

  let info = $state<KnmiInfo | null>(null);
  let key = $state('');
  let keyError = $state('');
  let error = $state('');
  let busy = $state(false);
  let editing = $state(false);

  async function load() {
    try {
      info = await api.knmi();
    } catch (err) {
      error = onerror(err);
    }
  }

  onMount(() => {
    void load();
    const timer = setInterval(() => void load(), REFRESH_MS);
    return () => clearInterval(timer);
  });

  async function connect(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    keyError = '';
    try {
      await api.connectKnmi(key.trim());
      toast.success('The KNMI key works and is stored.');
      key = '';
      editing = false;
      await load();
      await onchanged?.();
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        keyError = err.fieldMessage('key') ?? '';
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  async function disconnect() {
    const message = 'Weather widgets then show the weather model again. The key is deleted.';
    if (!(await ask({ title: 'Remove the KNMI key?', message, action: 'Remove' }))) return;
    try {
      await api.disconnectKnmi();
      toast.success('The KNMI key is removed.');
      await load();
      await onchanged?.();
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>KNMI</h2>
  <p class="muted">
    Show what the KNMI weather station nearest to the weather location measured, every 10 minutes, instead of the
    weather model's current temperature, humidity and wind. Request a free EDR API key on the KNMI Developer Portal
    (developer.dataplatform.knmi.nl); GDash stores it encrypted and never shows it again. Then choose "Measured by KNMI"
    in a Weather widget. Data: KNMI, CC BY 4.0.
  </p>
  {#if info?.connected && !editing}
    <ul class="list">
      <li>
        <div class="grow">
          <strong>Key set</strong>
          {#if info.unreadable}
            <span class="field-error">This key cannot be read on this server. Enter it again.</span>
          {/if}
        </div>
        <button class="link" onclick={() => (editing = true)}>Change</button>
        <button class="link danger" onclick={disconnect}>Remove</button>
      </li>
    </ul>
    {#if info.data}
      <p class="small" class:error={info.data.failing}>Measured weather: {describeStatus(info.data)}</p>
    {:else}
      <p class="small muted">Set a Weather widget to "Measured by KNMI" to use it.</p>
    {/if}
  {:else if info}
    <form onsubmit={connect}>
      <label>
        EDR API key
        <!-- Not type="password", so browsers do not offer to save it. -->
        <input bind:value={key} class="masked" autocomplete="off" spellcheck="false" required />
        {#if keyError}<span class="field-error">{keyError}</span>{/if}
      </label>
      <div class="inline">
        <button type="submit" disabled={busy}>{busy ? 'Trying the key…' : 'Save'}</button>
        {#if editing}<button type="button" class="link" onclick={() => (editing = false)}>Cancel</button>{/if}
      </div>
    </form>
  {:else}
    <p class="muted">Loading…</p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .masked {
    -webkit-text-security: disc;
  }
</style>
