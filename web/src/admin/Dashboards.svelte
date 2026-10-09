<script lang="ts">
  import FileButton from './FileButton.svelte';
  import { onMount } from 'svelte';
  import { api, ApiError, slugify, type Dashboard, type DashboardTemplate, type WidgetCatalogue } from './api';
  import { errorMessage } from './errors';
  import { freeId, fromFile, toFile } from './dashboardFile';
  import { fromTemplate } from './templates';
  import DashboardEditor from './DashboardEditor.svelte';
  import TemplateGallery from './TemplateGallery.svelte';

  let { onSessionEnded }: { onSessionEnded: () => void } = $props();

  let dashboards = $state<Dashboard[]>([]);
  let catalogue = $state<WidgetCatalogue | null>(null);
  let templates = $state<DashboardTemplate[]>([]);
  let creatingTemplate = $state('');
  let templateError = $state('');
  let editing = $state<Dashboard | null>(null);
  let loadError = $state('');
  let newName = $state('');
  let createError = $state('');

  onMount(() => void load());

  const handle = (err: unknown) => errorMessage(err, onSessionEnded);

  async function load() {
    try {
      [dashboards, catalogue, templates] = await Promise.all([api.dashboards(), api.widgetTypes(), api.dashboardTemplates()]);
      loadError = '';
    } catch (err) {
      loadError = handle(err);
    }
  }

  /** Creates a dashboard with a template's layout and opens it in the editor. */
  async function createFromTemplate(template: DashboardTemplate) {
    templateError = '';
    creatingTemplate = template.id;
    try {
      const created = await api.createDashboard(fromTemplate(template, dashboards.map((d) => d.id)));
      await load();
      editing = created;
    } catch (err) {
      templateError = handle(err);
    } finally {
      creatingTemplate = '';
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
    link.download = `${d.id}.gdash.json`;
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

  {#if catalogue && templates.length > 0}
    <section class="card" id="templates">
      <h2>Start from a template</h2>
      <p class="muted small">A ready-made layout with a fitting theme. It is added as a new dashboard that you can change in the editor.</p>
      <TemplateGallery {templates} columns={catalogue.columns} rows={catalogue.rows} busyId={creatingTemplate} onUse={createFromTemplate} />
      {#if templateError}<p class="error">{templateError}</p>{/if}
    </section>
  {/if}

  <form class="card" onsubmit={create}>
    <h2>New empty dashboard</h2>
    <label>
      Name
      <input bind:value={newName} placeholder="Evening" required />
    </label>
    {#if createError}<p class="error">{createError}</p>{/if}
    <button type="submit">Create and edit</button>
  </form>

  <section class="card">
    <h2>Import a dashboard</h2>
    <p class="muted">Choose a file that Export saved, from this GDash or another one. It is added as a new dashboard.</p>
    <FileButton label="Choose a dashboard file" accept=".json,application/json" onchange={importDashboard} />
    {#if importError}<p class="error">{importError}</p>{/if}
  </section>
{/if}
