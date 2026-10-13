<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from './toast.svelte';
  import { api, ApiError, type TrafficInfo } from './api';
  import { describeStatus } from './status';
  import { anwbStatusText } from './traffic';

  // It loads and refreshes its own state; onchanged tells the page that
  // shows it, so its card follows.
  let { onchanged, onerror }: { onchanged?: () => Promise<void>; onerror: (err: unknown) => string } = $props();

  const REFRESH_MS = 5_000;

  let info = $state<TrafficInfo | null>(null);
  let acknowledged = $state(false);
  let ackError = $state('');
  let error = $state('');
  let busy = $state(false);

  async function load() {
    try {
      info = await api.traffic();
    } catch (err) {
      error = onerror(err);
    }
  }

  onMount(() => {
    void load();
    const timer = setInterval(() => void load(), REFRESH_MS);
    return () => clearInterval(timer);
  });

  async function turn(enabled: boolean, event?: SubmitEvent) {
    event?.preventDefault();
    busy = true;
    error = '';
    ackError = '';
    try {
      await api.setTraffic(enabled, enabled && acknowledged);
      toast.success(enabled ? 'ANWB traffic info is on.' : 'ANWB traffic info is off.');
      acknowledged = false;
      await load();
      await onchanged?.();
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        ackError = err.fieldMessage('acknowledged') ?? '';
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }
</script>

<section class="card">
  <h2>ANWB traffic info <span class="tag">unofficial</span></h2>
  <p class="muted">
    Jams, roadworks and mobile speed checks for the Traffic widget, from the traffic list that anwb.nl itself reads.
    The ANWB offers no public API for it: GDash reads the list without permission, at most once every 3 minutes for all
    screens together, and stops when the ANWB refuses. The data belongs to the ANWB, which forbids reusing it; use it
    only on your own screens at home. Fixed speed cameras come from OpenStreetMap and work without this.
  </p>
  {#if info?.enabled}
    <ul class="list">
      <li>
        <div class="grow">
          <strong>On</strong>
          <span class="muted small">{anwbStatusText(info.anwb)}</span>
        </div>
        <button class="link danger" disabled={busy} onclick={() => turn(false)}>Turn off</button>
      </li>
    </ul>
    {#if info.anwb.state === 'stopped'}
      <p class="error">
        GDash stopped asking because the ANWB refused it several times. Turn traffic info off and on to try again; if it
        keeps happening, the ANWB has closed the list to GDash.
      </p>
    {/if}
    {#if info.data}
      <p class="small" class:error={info.data.failing}>Traffic widgets: {describeStatus(info.data)}</p>
    {:else}
      <p class="small muted">Add a Traffic widget to a dashboard to choose the roads it watches.</p>
    {/if}
  {:else if info}
    <form onsubmit={(e) => turn(true, e)}>
      <label class="check">
        <input type="checkbox" bind:checked={acknowledged} />
        I use this for my own household only; the data belongs to the ANWB.
      </label>
      {#if ackError}<span class="field-error">{ackError}</span>{/if}
      <div class="inline">
        <button type="submit" disabled={busy || !acknowledged}>{busy ? 'Turning on…' : 'Turn on'}</button>
      </div>
    </form>
  {:else}
    <p class="muted">Loading…</p>
  {/if}
  {#if info?.cameras}
    <p class="small" class:error={info.cameras.failing}>
      Fixed cameras (© OpenStreetMap contributors, ODbL): {describeStatus(info.cameras)}
    </p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .tag {
    margin-left: 0.4em;
    padding: 1px 6px;
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    font-size: var(--fs-xs);
    font-weight: 550;
    color: var(--text-soft);
    vertical-align: middle;
  }
  .check {
    display: flex;
    gap: 8px;
    align-items: flex-start;
    font-weight: 400;
  }
  .check input {
    margin-top: 0.25em;
  }
</style>
