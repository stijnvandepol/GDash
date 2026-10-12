<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, ApiError, type DepartureStop } from './api';
  import { typeAheadSearch } from './stopSearch';

  let {
    code,
    name,
    onpick,
  }: {
    /** The chosen stop area code, or empty. */
    code: string;
    /** The chosen stop's name, shown on the screen. */
    name: string;
    onpick: (stop: DepartureStop) => void;
  } = $props();

  let query = $state('');
  let results = $state<DepartureStop[] | null>(null);
  let busy = $state(false);
  let error = $state('');

  const search = typeAheadSearch((q: string) => api.departureStops(q), {
    onIdle: () => {
      results = null;
      busy = false;
      error = '';
    },
    onBusy: () => (busy = true),
    onResult: (stops) => {
      results = stops;
      busy = false;
      error = '';
    },
    onError: (err) => {
      results = null;
      busy = false;
      error = err instanceof ApiError ? (err.fieldMessage('q') ?? err.message) : String(err);
    },
  });
  onDestroy(search.cancel);

  function pick(stop: DepartureStop) {
    onpick(stop);
    query = '';
    search.cancel();
    results = null;
    busy = false;
  }
</script>

<div class="picker">
  <span>Stop</span>
  {#if code}
    <p class="chosen">{name || code} <span class="muted small">{code}</span></p>
  {:else}
    <p class="field-error">Choose a stop.</p>
  {/if}
  <input
    bind:value={query}
    oninput={() => search.input(query)}
    placeholder="Search a stop or town, for example Utrecht CS"
    aria-label="Search stops"
  />
  <p class="muted small">Buses, trams, metros and ferries from OVapi. Trains are not included.</p>
  {#if error}
    <p class="error">{error}</p>
  {:else if busy}
    <p class="muted small">Searching…</p>
  {:else if results}
    <ul class="matches">
      {#each results as stop (stop.code)}
        <li>
          <button type="button" class="link" onclick={() => pick(stop)}>{stop.name}</button>
          <span class="muted small">{stop.code}</span>
        </li>
      {:else}
        <li class="muted">No stop matches.</li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .picker {
    display: grid;
    gap: 6px;
    min-width: 0;
  }
  .chosen {
    margin: 0;
    overflow-wrap: anywhere;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 4px;
  }
  li {
    display: flex;
    gap: 8px;
    align-items: baseline;
    flex-wrap: wrap;
    overflow-wrap: anywhere;
  }
  .matches {
    max-height: 14em;
    overflow-y: auto;
  }
</style>
