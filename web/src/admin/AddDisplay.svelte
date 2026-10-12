<script lang="ts">
  import { toast } from './toast.svelte';
  import { api, ApiError, slugify, type CastDevice, type Dashboard } from './api';
  import { DEFAULT_DASHBOARD_ID, suggestedDashboard } from './displays';
  import DashboardThumb from './DashboardThumb.svelte';

  let {
    devices,
    dashboards,
    grid,
    searching,
    searchMessage,
    onsearch,
    oncreated,
    onerror,
  }: {
    /** Cast devices found on the network. */
    devices: CastDevice[];
    dashboards: Dashboard[];
    /** The TV grid, to draw the chosen dashboard; null until it is known. */
    grid: { columns: number; rows: number } | null;
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
  // The dashboard someone picked; until then the newest one is offered.
  let pickedDashboard = $state<string | null>(null);
  const dashboardId = $derived(pickedDashboard ?? suggestedDashboard(dashboards));
  const chosen = $derived(dashboards.find((b) => b.id === dashboardId));

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
      const name = newName.trim();
      await api.createDisplay({
        id: newId,
        name,
        castDeviceId: newDevice,
        dashboardId: dashboardId === DEFAULT_DASHBOARD_ID ? '' : dashboardId,
      });
      toast.success(newDevice ? `Added ${name}. GDash puts it on the TV in a moment.` : `Added ${name}. Now connect a screen to it.`);
      newName = '';
      newId = '';
      idEdited = false;
      newDevice = '';
      showAddress = false;
      pickedDashboard = null;
      await oncreated();
    } catch (err) {
      const message = onerror(err);
      createError = err instanceof ApiError ? err : new ApiError(0, 'unknown', message);
    } finally {
      creating = false;
    }
  }
</script>

<form class="card" onsubmit={create} aria-labelledby="add-display-title">
  <h2 id="add-display-title">Add a display</h2>
  <label>
    Name
    <input id="display-name" bind:value={newName} oninput={nameChanged} placeholder="Kitchen" required />
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
    Shows
    <span class="pick">
      <select value={dashboardId} onchange={(e) => (pickedDashboard = (e.target as HTMLSelectElement).value)}>
        {#each dashboards as b (b.id)}
          <option value={b.id}>{b.name}</option>
        {/each}
      </select>
      {#if grid && chosen}
        <span class="thumb" aria-hidden="true">
          <DashboardThumb widgets={chosen.widgets} theme={chosen.theme ?? 'minimal-dark'} font={chosen.font} columns={grid.columns} rows={grid.rows} labels={false} />
        </span>
      {/if}
    </span>
    {#if createError?.fieldMessage('dashboardId')}<span class="field-error">{createError.fieldMessage('dashboardId')}</span>{/if}
  </label>
  <label>
    Cast TV
    <select bind:value={newDevice} onchange={nameFromDevice}>
      <option value="">None: I open its address on the screen myself (tablet, Fire TV, computer)</option>
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
      {searchMessage || `Found ${screens.length === 1 ? '1 Cast device' : `${screens.length} Cast devices`} with a screen.`}
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
  .pick {
    display: flex;
    align-items: center;
    gap: var(--s-3);
  }
  .pick select {
    flex: 1;
    min-width: 0;
  }
  .thumb {
    flex: none;
    width: 96px;
  }
</style>
