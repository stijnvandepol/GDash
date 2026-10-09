<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type HomeAssistantEntity } from './api';
  import { matchEntities } from './entities';

  let {
    selected,
    max,
    onchange,
  }: {
    /** Entity IDs in the order the widget shows them. */
    selected: string[];
    max: number;
    onchange: (ids: string[]) => void;
  } = $props();

  let entities = $state<HomeAssistantEntity[]>([]);
  let loadError = $state('');
  let query = $state('');

  const names = $derived(new Map(entities.map((e) => [e.id, e.name])));
  const matches = $derived(matchEntities(entities, query, selected));

  onMount(async () => {
    try {
      entities = await api.homeAssistantEntities();
    } catch (err) {
      loadError = err instanceof Error ? err.message : String(err);
    }
  });

  function add(id: string) {
    if (selected.length < max && !selected.includes(id)) onchange([...selected, id]);
    query = '';
  }
</script>

<div class="picker">
  <span>Entities</span>
  {#if selected.length > 0}
    <ul class="chosen">
      {#each selected as id (id)}
        <li>
          <span class="grow">{names.get(id) ?? id} <span class="muted small">{id}</span></span>
          <button type="button" class="link danger" onclick={() => onchange(selected.filter((s) => s !== id))}>
            Remove
          </button>
        </li>
      {/each}
    </ul>
  {:else}
    <p class="field-error">Choose at least one entity.</p>
  {/if}
  {#if loadError}
    <p class="error">{loadError}</p>
  {:else if selected.length < max}
    <input bind:value={query} placeholder="Search, for example temperature" aria-label="Search entities" />
    {#if query.trim()}
      <ul class="matches">
        {#each matches as e (e.id)}
          <li>
            <button type="button" class="link" onclick={() => add(e.id)}>{e.name}</button>
            <span class="muted small">{e.id} · {e.state}{e.unit ? ` ${e.unit}` : ''}</span>
          </li>
        {:else}
          <li class="muted">No entity matches.</li>
        {/each}
      </ul>
    {/if}
  {:else}
    <p class="muted small">This widget shows at most {max} entities.</p>
  {/if}
</div>

<style>
  .picker {
    display: grid;
    gap: 6px;
    min-width: 0;
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
  .grow {
    flex: 1 1 auto;
    min-width: 0;
  }
  .matches {
    max-height: 14em;
    overflow-y: auto;
  }
</style>
