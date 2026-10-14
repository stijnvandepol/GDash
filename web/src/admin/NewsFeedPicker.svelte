<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type NewsFeed } from './api';

  let {
    selected,
    max,
    onchange,
  }: {
    /** The IDs of the news feeds the widget shows; empty for every feed. */
    selected: number[];
    max: number;
    onchange: (ids: number[]) => void;
  } = $props();

  let feeds = $state<NewsFeed[]>([]);
  let loaded = $state(false);
  let loadError = $state('');
  // "Only these sources" stays chosen while none is ticked yet.
  let pickFeeds = $state(false);
  const onlyThese = $derived(pickFeeds || selected.length > 0);
  // A feed removed under Integrations after it was ticked stays in the list,
  // so it can be unticked instead of silently counting.
  const removed = $derived(loaded ? selected.filter((id) => !feeds.some((f) => f.id === id)) : []);

  onMount(async () => {
    try {
      feeds = (await api.settings()).settings.newsFeeds;
    } catch (err) {
      loadError = err instanceof Error ? err.message : String(err);
    }
    loaded = true;
  });

  function toggle(id: number, on: boolean) {
    onchange(on ? [...selected, id].slice(0, max) : selected.filter((s) => s !== id));
  }

  function chooseEvery() {
    pickFeeds = false;
    onchange([]);
  }
</script>

<fieldset class="picker">
  <legend>Sources</legend>
  <label class="option">
    <input type="radio" name="news-feeds" checked={!onlyThese} onchange={chooseEvery} />
    All chosen sources
  </label>
  <label class="option">
    <input type="radio" name="news-feeds" checked={onlyThese} onchange={() => (pickFeeds = true)} />
    Only these sources
  </label>
  {#if onlyThese}
    {#if loadError}
      <p class="error">{loadError}</p>
    {:else if !loaded}
      <p class="muted small" role="status">Loading…</p>
    {:else}
      <ul>
        {#each feeds as f (f.id)}
          {@const checked = selected.includes(f.id)}
          <li>
            <label>
              <input
                type="checkbox"
                {checked}
                disabled={!checked && selected.length >= max}
                onchange={(e) => toggle(f.id, (e.target as HTMLInputElement).checked)}
              />
              {f.name}
            </label>
          </li>
        {:else}
          <li class="muted">No news sources yet. Choose them under Integrations, News feeds.</li>
        {/each}
        {#each removed as id (id)}
          <li>
            <label>
              <input type="checkbox" checked onchange={() => toggle(id, false)} />
              <span class="field-error">Source {id} (removed)</span>
            </label>
          </li>
        {/each}
      </ul>
      {#if selected.length === 0}
        <p class="field-error">Tick at least one source, or choose all chosen sources.</p>
      {/if}
    {/if}
  {/if}
</fieldset>

<style>
  .picker {
    display: grid;
    gap: 6px;
    min-width: 0;
    margin: 0;
    padding: 0;
    border: 0;
  }
  legend {
    padding: 0;
    margin-bottom: 4px;
  }
  .option {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 400;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 4px;
    max-height: 14em;
    overflow-y: auto;
  }
  li {
    overflow-wrap: anywhere;
  }
  li label {
    display: flex;
    gap: 8px;
    align-items: baseline;
    font-weight: 400;
  }
</style>
