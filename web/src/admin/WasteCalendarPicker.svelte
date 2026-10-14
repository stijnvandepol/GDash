<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Calendar } from './api';

  let {
    selected,
    onchange,
  }: {
    /** The chosen waste calendar's number, or undefined: the only one. */
    selected: number | undefined;
    onchange: (id: number | undefined) => void;
  } = $props();

  let calendars = $state<Calendar[]>([]);
  let loaded = $state(false);
  let loadError = $state('');

  // A calendar that was deleted after it was chosen stays in the list, so
  // the choice is not silently shown as something else.
  const missing = $derived(loaded && selected !== undefined && !calendars.some((c) => c.id === selected));
  // With one waste calendar there is nothing to choose: a widget without a
  // choice shows it (internal/display WasteDefaultKey).
  const only = $derived(calendars.length === 1 && !missing ? calendars[0] : undefined);

  onMount(async () => {
    try {
      calendars = (await api.settings()).settings.wasteCalendars;
    } catch (err) {
      loadError = err instanceof Error ? err.message : String(err);
    }
    loaded = true;
  });
</script>

<div class="picker">
  {#if loadError}
    <p class="error">{loadError}</p>
  {:else if !loaded}
    <p class="small muted" role="status">Loading…</p>
  {:else if calendars.length === 0 && !missing}
    <p class="field-error">
      There is no waste calendar yet. Paste the link of your waste calendar under Integrations, Waste collection; this
      widget then shows it.
    </p>
  {:else if only}
    <p class="small">Shows <strong>{only.name}</strong>, your waste calendar.</p>
  {:else}
    <label class="inline-label">
      Waste calendar
      <select
        value={selected === undefined ? '' : String(selected)}
        onchange={(e) => {
          const raw = (e.target as HTMLSelectElement).value;
          onchange(raw === '' ? undefined : Number(raw));
        }}
      >
        <option value="">Choose an address</option>
        {#each calendars as c (c.id)}
          <option value={String(c.id)}>{c.name}</option>
        {/each}
        {#if missing}
          <option value={String(selected)}>Calendar {selected} (not found)</option>
        {/if}
      </select>
    </label>
    {#if missing}
      <p class="field-error">This waste calendar was removed. Choose another.</p>
    {:else if selected === undefined}
      <p class="field-error">You have more than one waste calendar. Choose the one this widget shows.</p>
    {/if}
  {/if}
</div>

<style>
  .picker {
    display: grid;
    gap: 6px;
    min-width: 0;
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
