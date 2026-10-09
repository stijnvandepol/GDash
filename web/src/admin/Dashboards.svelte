<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError, slugify, type Dashboard, type WidgetCatalogue } from './api';
  import { errorMessage } from './errors';
  import { freeId, fromFile, toFile } from './dashboardFile';
  import DashboardEditor from './DashboardEditor.svelte';

  let { onSessionEnded }: { onSessionEnded: () => void } = $props();

  let dashboards = $state<Dashboard[]>([]);
  let catalogue = $state<WidgetCatalogue | null>(null);
  let editing = $state<Dashboard | null>(null);
  let loadError = $state('');
  let newName = $state('');
  let createError = $state('');

  onMount(() => void load());

  const handle = (err: unknown) => errorMessage(err, onSessionEnded);

  async function load() {
    try {
      [dashboards, catalogue] = await Promise.all([api.dashboards(), api.widgetTypes()]);
      loadError = '';
    } catch (err) {
      loadError = handle(err);
    }
  }

  async function create(event: SubmitEvent) {
    event.preventDefault();
    createError = '';
    const name = newName.trim();
    try {
      const created = await api.createDashboard({ id: slugify(name), name, widgets: [] });
      newName = '';
      await load();
      editing = created;
    } catch (err) {
      createError = err instanceof ApiError ? (err.fieldMessage('id') ?? err.fieldMessage('name') ?? err.message) : handle(err);
    }
  }

  async function remove(d: Dashboard) {
    if (!confirm(`Delete dashboard "${d.name}"? Displays showing it switch to the default dashboard.`)) return;
    try {
      await api.deleteDashboard(d.id);
      await load();
    } catch (err) {
      loadError = handle(err);
    }
  }

  /** Saves the dashboard as a file the browser downloads. */
  function exportDashboard(d: Dashboard) {
    const url = URL.createObjectURL(new Blob([toFile(d)], { type: 'application/json' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = `${d.id}.nookboard.json`;
    link.click();
    URL.revokeObjectURL(url);
  }

  let importError = $state('');

  /** Creates a new dashboard from an exported file, next to the existing ones. */
  async function importDashboard(event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;
    importError = '';
    try {
      const imported = fromFile(await file.text());
      const id = freeId(imported.name, dashboards.map((d) => d.id));
      await api.createDashboard({ id, ...imported });
      await load();
    } catch (err) {
      importError = describeImportError(err);
    }
  }

  function describeImportError(err: unknown): string {
    if (err instanceof ApiError && err.fields.length > 0) {
      return `This dashboard cannot be imported: ${err.fields.map((f) => f.message).join('; ')}`;
    }
    if (err instanceof ApiError) return handle(err);
    return err instanceof Error ? err.message : 'The file could not be read.';
  }

  async function closeEditor() {
    editing = null;
    await load();
  }
</script>

{#if editing && catalogue}
  <DashboardEditor dashboard={editing} {catalogue} onClose={closeEditor} {onSessionEnded} />
{:else}
  <section class="card">
    <h1>Dashboards</h1>
    {#if loadError}<p class="error">{loadError}</p>{/if}
    <ul class="list">
      {#each dashboards as d (d.id)}
        <li>
          <div class="grow">
            <strong>{d.name}</strong>
            <span class="muted">{d.widgets.length === 1 ? '1 widget' : `${d.widgets.length} widgets`}{d.id === 'default' ? ' · shown on displays without a choice' : ''}</span>
          </div>
          <button class="link" onclick={() => (editing = d)}>Edit</button>
          <button class="link" onclick={() => exportDashboard(d)}>Export</button>
          {#if d.id !== 'default'}
            <button class="link danger" onclick={() => remove(d)}>Delete</button>
          {/if}
        </li>
      {/each}
    </ul>
  </section>

  <form class="card" onsubmit={create}>
    <h2>New dashboard</h2>
    <label>
      Name
      <input bind:value={newName} placeholder="Evening" required />
    </label>
    {#if createError}<p class="error">{createError}</p>{/if}
    <button type="submit">Create and edit</button>
  </form>

  <section class="card">
    <h2>Import a dashboard</h2>
    <p class="muted">Choose a file that Export saved, from this Nookboard or another one. It is added as a new dashboard.</p>
    <input type="file" accept=".json,application/json" aria-label="Dashboard file" onchange={importDashboard} />
    {#if importError}<p class="error">{importError}</p>{/if}
  </section>
{/if}
