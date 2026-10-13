<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type WebPage } from './api';

  let {
    selected,
    onchange,
  }: {
    /** The chosen page's number, or undefined when none is chosen. */
    selected: number | undefined;
    onchange: (id: number | undefined) => void;
  } = $props();

  let pages = $state<WebPage[]>([]);
  let loaded = $state(false);
  let loadError = $state('');

  // A page that was removed after it was chosen stays in the list, so the
  // choice is not silently shown as another.
  const missing = $derived(loaded && selected !== undefined && !pages.some((p) => p.id === selected));
  const chosen = $derived(pages.find((p) => p.id === selected));

  onMount(async () => {
    try {
      pages = await api.webPages();
    } catch (err) {
      loadError = err instanceof Error ? err.message : String(err);
    }
    loaded = true;
  });
</script>

<div class="picker">
  <label class="inline-label">
    Web page
    <select
      value={selected === undefined ? '' : String(selected)}
      onchange={(e) => {
        const raw = (e.target as HTMLSelectElement).value;
        onchange(raw === '' ? undefined : Number(raw));
      }}
    >
      <option value="">Choose a web page</option>
      {#each pages as p (p.id)}
        <option value={String(p.id)}>{p.name}</option>
      {/each}
      {#if missing}
        <option value={String(selected)}>Web page {selected} (not found)</option>
      {/if}
    </select>
  </label>
  {#if loadError}
    <p class="error">{loadError}</p>
  {:else if loaded && pages.length === 0}
    <p class="field-error">There is no web page yet. Add one under Integrations, Web pages, then choose it here.</p>
  {:else if missing}
    <p class="field-error">This web page was removed. Choose another.</p>
  {:else if selected === undefined}
    <p class="field-error">Choose the page to show.</p>
  {:else if chosen}
    <p class="small muted">Shown at {chosen.zoom}% unless you set a zoom here. The TV shows it without interaction.</p>
  {/if}
</div>

<style>
  .picker {
    display: grid;
    gap: 6px;
    min-width: 0;
  }
  .picker p {
    margin: 0;
  }
  .inline-label {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    font-weight: 400;
    min-width: 0;
  }
</style>
