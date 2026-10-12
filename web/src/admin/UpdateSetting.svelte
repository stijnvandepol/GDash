<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type SystemInfo } from './api';
  import UpdateNotice from './UpdateNotice.svelte';

  let { onerror }: { onerror: (err: unknown) => string } = $props();

  let system = $state<SystemInfo | null>(null);
  let enabled = $state(true);
  let error = $state('');
  let busy = $state(false);

  onMount(() => void load());

  async function load() {
    try {
      const [info, current] = await Promise.all([api.system(), api.settings()]);
      system = info;
      enabled = current.settings.updateCheck;
      error = '';
    } catch (err) {
      error = onerror(err);
    }
  }

  async function toggle(event: Event) {
    const on = (event.target as HTMLInputElement).checked;
    busy = true;
    error = '';
    try {
      await api.setUpdateCheck(on);
      enabled = on;
    } catch (err) {
      error = onerror(err);
    } finally {
      busy = false;
    }
  }
</script>

<section class="card" aria-labelledby="updates-title">
  <h2 id="updates-title">Updates</h2>
  <p class="muted">
    {#if system}This server runs GDash {system.version}.{/if}
    Once a day GDash can ask GitHub which version is the newest, and say so at the top of the web interface. The
    request sends nothing about you or your screens, only the version number. Apart from your data sources, it
    is the only request GDash makes.
  </p>
  {#if system?.updateCheckLocked}
    <p class="muted small">Turned off on the server with <code>GDASH_UPDATE_CHECK=off</code>.</p>
  {:else}
    <label class="check">
      <input type="checkbox" checked={enabled} disabled={busy || !system} onchange={toggle} />
      Check for new versions once a day
    </label>
  {/if}
  {#if system?.update && enabled && !system.updateCheckLocked}<UpdateNotice release={system.update} />{/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .check {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-md);
    font-weight: 450;
  }
  .check input {
    width: auto;
  }
  code {
    font-family: var(--mono);
  }
</style>
