<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Camera } from './api';

  let {
    selected,
    onchange,
  }: {
    /** The chosen camera's number, or undefined when none is chosen. */
    selected: number | undefined;
    onchange: (id: number | undefined) => void;
  } = $props();

  let cameras = $state<Camera[]>([]);
  let loaded = $state(false);
  let loadError = $state('');

  // A camera that was removed after it was chosen stays in the list, so the
  // choice is not silently shown as another camera.
  const missing = $derived(loaded && selected !== undefined && !cameras.some((c) => c.id === selected));

  onMount(async () => {
    try {
      cameras = (await api.cameras()).cameras;
    } catch (err) {
      loadError = err instanceof Error ? err.message : String(err);
    }
    loaded = true;
  });
</script>

<div class="picker">
  <label class="inline-label">
    Camera
    <select
      value={selected === undefined ? '' : String(selected)}
      onchange={(e) => {
        const raw = (e.target as HTMLSelectElement).value;
        onchange(raw === '' ? undefined : Number(raw));
      }}
    >
      <option value="">Choose a camera</option>
      {#each cameras as c (c.id)}
        <option value={String(c.id)}>{c.name}</option>
      {/each}
      {#if missing}
        <option value={String(selected)}>Camera {selected} (not found)</option>
      {/if}
    </select>
  </label>
  {#if loadError}
    <p class="error">{loadError}</p>
  {:else if loaded && cameras.length === 0}
    <p class="field-error">There is no camera yet. Add one under Integrations, Camera snapshots, then choose it here.</p>
  {:else if missing}
    <p class="field-error">This camera was removed. Choose another.</p>
  {:else if selected === undefined}
    <p class="field-error">Choose the camera this widget shows.</p>
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
