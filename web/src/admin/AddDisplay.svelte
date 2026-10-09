<script lang="ts">
  import { toast } from './toast.svelte';
  import { api, ApiError, slugify, type CastDevice } from './api';

  let {
    devices,
    searching,
    searchMessage,
    onsearch,
    oncreated,
    onerror,
  }: {
    /** Cast devices found on the network. */
    devices: CastDevice[];
    searching: boolean;
    searchMessage: string;
    onsearch: () => void;
    /** Called after the display was created. */
    oncreated: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let newName = $state('');
  let newId = $state('');
  let idEdited = $state(false);
  let newDevice = $state('');
  let createError = $state<ApiError | null>(null);
  let creating = $state(false);
  let showAddress = $state(false);

  const screens = $derived(devices.filter((d) => d.hasScreen));
  const speakers = $derived(devices.filter((d) => !d.hasScreen));

  // Choosing a TV before typing a name names the display after the TV.
  function nameFromDevice() {
    const device = devices.find((d) => d.id === newDevice);
    if (device && !newName.trim()) {
      newName = device.name;
      nameChanged();
    }
  }

  function nameChanged() {
    if (!idEdited) newId = slugify(newName);
  }

  async function create(event: SubmitEvent) {
    event.preventDefault();
    creating = true;
    createError = null;
    try {
      await api.createDisplay({ id: newId, name: newName.trim(), castDeviceId: newDevice });
      toast.success(`Added ${newName.trim()}.`);
      newName = '';
      newId = '';
      idEdited = false;
      newDevice = '';
      showAddress = false;
      await oncreated();
    } catch (err) {
      const message = onerror(err);
      createError = err instanceof ApiError ? err : new ApiError(0, 'unknown', message);
    } finally {
      creating = false;
    }
  }
</script>

<form class="card" onsubmit={create}>
  <h2>Add a display</h2>
  <label>
    Name
    <input bind:value={newName} oninput={nameChanged} placeholder="Kitchen" required />
    {#if createError?.fieldMessage('name')}<span class="field-error">{createError.fieldMessage('name')}</span>{/if}
  </label>
  <!-- The address follows from the name; most people never need to see
       the field, so it opens on request or when it has a problem. -->
  {#if showAddress || createError?.fieldMessage('id')}
  <label>
      Display page address
      <span class="with-prefix">
        <span class="prefix">/d/</span>
        <input bind:value={newId} oninput={() => (idEdited = true)} required spellcheck="false" />
      </span>
      {#if createError?.fieldMessage('id')}<span class="field-error">{createError.fieldMessage('id')}</span>{/if}
    </label>
  {:else}
    <p class="muted small address">
      Its page will be at <code>/d/{newId || '…'}</code>
      <button type="button" class="link" onclick={() => (showAddress = true)}>Change</button>
    </p>
  {/if}
  <label>
    TV
    <select bind:value={newDevice} onchange={nameFromDevice}>
      <option value="">No Cast: I open the page myself in a browser or kiosk</option>
      {#each screens as dev (dev.id)}
        <option value={dev.id}>{dev.name} ({dev.model})</option>
      {/each}
      {#if speakers.length > 0}
        <optgroup label="No screen, cannot show a dashboard">
          {#each speakers as dev (dev.id)}
            <option value={dev.id} disabled>{dev.name} ({dev.model})</option>
          {/each}
        </optgroup>
      {/if}
    </select>
  </label>
  <p class="muted small">
    {#if searching}Searching for Cast devices…{:else}
      {searchMessage || `Found ${screens.length} TVs and screens.`}
      <button type="button" class="link" onclick={onsearch}>Search again</button>
    {/if}
  </p>
  {#if createError && createError.fields.length === 0}<p class="error">{createError.message}</p>{/if}
  <button type="submit" disabled={creating}>{creating ? 'Working…' : 'Add display'}</button>
</form>

<style>
  .address {
    margin: 0;
  }
  .address code {
    font-family: var(--mono);
    color: var(--text-soft);
  }
</style>
