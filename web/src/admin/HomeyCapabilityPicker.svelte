<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from './api';
  import { matchEntities } from './entities';
  import { capabilityChoices, type CapabilityChoice } from './homey';

  let {
    selected,
    max,
    onchange,
  }: {
    /** Capability keys in the order the widget shows them. */
    selected: string[];
    max: number;
    onchange: (keys: string[]) => void;
  } = $props();

  let choices = $state<CapabilityChoice[]>([]);
  let loadError = $state('');
  let query = $state('');

  const byKey = $derived(new Map(choices.map((c) => [c.id, c])));
  const matches = $derived(matchEntities(choices, query, selected));

  onMount(async () => {
    try {
      choices = capabilityChoices(await api.homeyCapabilities());
    } catch (err) {
      loadError = err instanceof Error ? err.message : String(err);
    }
  });

  function add(key: string) {
    if (selected.length < max && !selected.includes(key)) onchange([...selected, key]);
    query = '';
  }
</script>

<div class="picker">
  <span>Device values</span>
  {#if selected.length > 0}
    <ul class="chosen">
      {#each selected as key (key)}
        {@const choice = byKey.get(key)}
        <li>
          <span class="grow">{choice?.name ?? key} {#if choice?.zone}<span class="muted small">{choice.zone}</span>{/if}</span>
          <button type="button" class="link danger" onclick={() => onchange(selected.filter((s) => s !== key))}>
            Remove
          </button>
        </li>
      {/each}
    </ul>
  {:else}
    <p class="field-error">Choose at least one value.</p>
  {/if}
  {#if loadError}
    <p class="error">{loadError}</p>
  {:else if selected.length < max}
    <input bind:value={query} placeholder="Search, for example kitchen temperature" aria-label="Search Homey devices" />
    {#if query.trim()}
      <ul class="matches">
        {#each matches as c (c.id)}
          <li>
            <button type="button" class="link" onclick={() => add(c.id)}>{c.name}</button>
            <span class="muted small">{c.detail}</span>
          </li>
        {:else}
          <li class="muted">No device value matches.</li>
        {/each}
      </ul>
    {/if}
  {:else}
    <p class="muted small">This widget shows at most {max} values.</p>
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
