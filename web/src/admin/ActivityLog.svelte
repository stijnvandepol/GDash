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
    <!-- A log is read down the time column and across a row, so it is a
         table: one line per event, the times aligned. -->
    <table class="log">
      <caption class="sr-only">Recent activity, newest first</caption>
      <thead>
        <tr><th scope="col">When</th><th scope="col">What</th><th scope="col">Who and from where</th></tr>
      </thead>
      <tbody>
        {#each events as e, i (i)}
          <tr class:warning={isWarning(e)}>
            <td class="when mono num">{when.format(new Date(e.at))}</td>
            <td class="what">{describeAction(e)}</td>
            <td class="who muted">{e.actor || 'unknown'} <span class="mono">{e.ip}</span></td>
          </tr>
        {/each}
      </tbody>
    </table>
    <button class="link" onclick={load}>Refresh</button>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .log {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-sm);
    /* Names and addresses can be long words; they wrap instead of
       widening the page. */
    overflow-wrap: anywhere;
  }
  th {
    padding: 0 var(--s-3) var(--s-2) 0;
    border-bottom: 1px solid var(--border-strong);
    color: var(--muted);
    font-size: var(--fs-xs);
    font-weight: 550;
    text-align: left;
  }
  td {
    padding: 7px var(--s-3) 7px 0;
    border-bottom: 1px solid var(--border);
    vertical-align: baseline;
  }
  tbody tr:hover td {
    background: var(--neutral-soft);
  }
  .when {
    width: 9.5rem;
    color: var(--muted);
    white-space: nowrap;
  }
  .who {
    text-align: right;
    padding-right: 0;
  }
  th:last-child {
    text-align: right;
    padding-right: 0;
  }
  .warning .what {
    color: var(--danger);
    font-weight: 550;
  }
  /* A phone: each event as a small block, the time above it. */
  @container page (max-width: 560px) {
    thead {
      display: none;
    }
    tr,
    td {
      display: block;
    }
    tr {
      padding: var(--s-2) 0;
      border-bottom: 1px solid var(--border);
    }
    td {
      padding: 0;
      border: 0;
      text-align: left;
    }
    .who {
      text-align: left;
    }
  }
</style>
