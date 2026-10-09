<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from './api';
  import { describeAction, isWarning, type AuditEvent } from './audit';

  let { onerror }: { onerror: (err: unknown) => string } = $props();

  // A screenful; the server keeps 90 days.
  const SHOWN = 50;

  let events = $state<AuditEvent[] | null>(null);
  let error = $state('');
  const when = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

  onMount(() => void load());

  async function load() {
    try {
      events = await api.auditEvents(SHOWN);
      error = '';
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>Recent activity</h2>
  <p class="muted">Logins, pairings and changes, kept for 90 days.</p>
  {#if events && events.length === 0}
    <p class="muted">Nothing yet.</p>
  {:else if events}
    <ul class="list">
      {#each events as e, i (i)}
        <li>
          <div class="grow">
            <span class:error={isWarning(e)}>{describeAction(e)}</span>
            <span class="muted small">{when.format(new Date(e.at))} · {e.actor || 'unknown'} · {e.ip}</span>
          </div>
        </li>
      {/each}
    </ul>
    <button class="link" onclick={load}>Refresh</button>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  /* Names and addresses can be long words; they wrap instead of widening
     the page. */
  .list {
    overflow-wrap: anywhere;
  }
</style>
