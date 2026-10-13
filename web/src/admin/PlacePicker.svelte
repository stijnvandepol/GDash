<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, ApiError, type Place } from './api';
  import { typeAheadSearch } from './stopSearch';

  let {
    name,
    onpick,
    onclear,
  }: {
    /** The chosen place's name, or empty when the widget follows the weather location. */
    name: string;
    onpick: (place: Place) => void;
    onclear: () => void;
  } = $props();

  let query = $state('');
  let results = $state<Place[] | null>(null);
  let busy = $state(false);
  let error = $state('');

  // The same search as the weather location under Integrations.
  const search = typeAheadSearch((q: string) => api.places(q), {
    onIdle: () => {
      results = null;
      busy = false;
      error = '';
    },
    onBusy: () => (busy = true),
    onResult: (places) => {
      results = places;
      busy = false;
      error = '';
    },
    onError: (err) => {
      results = null;
      busy = false;
      error = err instanceof ApiError ? (err.fieldMessage('name') ?? err.message) : String(err);
    },
  });
  onDestroy(search.cancel);

  function pick(place: Place) {
    onpick(place);
    query = '';
    search.cancel();
    results = null;
    busy = false;
  }
</script>

<div class="picker">
  <span>Place</span>
  {#if name}
    <p class="chosen">
      {name}
      <button type="button" class="link" onclick={onclear}>Use the weather location</button>
    </p>
  {:else}
    <p class="muted small">The weather location from Integrations.</p>
  {/if}
  <input
    bind:value={query}
    oninput={() => search.input(query)}
    placeholder="Search a town or postcode"
    aria-label="Search places"
  />
  {#if error}
    <p class="error">{error}</p>
  {:else if busy}
    <p class="muted small">Searching…</p>
  {:else if results}
    <ul class="matches">
      {#each results as place, i (i)}
        <li>
          <button type="button" class="link" onclick={() => pick(place)}>{place.name}</button>
          <span class="muted small">{place.region}</span>
        </li>
      {:else}
        <li class="muted">No places found. Try another spelling.</li>
      {/each}
    </ul>
  {/if}
  <p class="muted small">Place search by Open-Meteo, with data from GeoNames. Prices are the ANWB's public data.</p>
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
