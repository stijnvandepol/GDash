<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Calendar } from './api';

  let {
    selected,
    onchange,
  }: {
    /** The chosen calendar's number, or undefined when none is chosen. */
    selected: number | undefined;
    onchange: (id: number | undefined) => void;
  } = $props();

  let calendars = $state<Calendar[]>([]);
  let loaded = $state(false);
  let loadError = $state('');

  // A calendar that was deleted after it was chosen stays in the list, so
  // the choice is not silently shown as something else.
  const missing = $derived(loaded && selected !== undefined && !calendars.some((c) => c.id === selected));

  onMount(async () => {
    try {
      calendars = (await api.settings()).settings.calendars;
    } catch (err) {
      loadError = err instanceof Error ? err.message : String(err);
    }
    loaded = true;
  });
</script>

<div class="picker">
  <label class="inline-label">
    Calendar
    <select
      value={selected === undefined ? '' : String(selected)}
      onchange={(e) => {
        const raw = (e.target as HTMLSelectElement).value;
        onchange(raw === '' ? undefined : Number(raw));
      }}
    >
      <option value="">Choose a calendar</option>
      {#each calendars as c (c.id)}
        <option value={String(c.id)}>{c.name}</option>
      {/each}
      {#if missing}
        <option value={String(selected)}>Calendar {selected} (not found)</option>
      {/if}
    </select>
  </label>
  {#if loadError}
    <p class="error">{loadError}</p>
  {:else if loaded && calendars.length === 0}
    <p class="field-error">There is no calendar yet. Add the iCal link of your waste collection under Integrations, Calendars, then choose it here.</p>
  {:else if missing}
    <p class="field-error">This calendar was removed. Choose another.</p>
  {:else if selected === undefined}
    <p class="field-error">Choose the calendar that holds the pickup dates.</p>
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
