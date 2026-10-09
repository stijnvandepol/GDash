<script lang="ts">
  import type { DashboardTemplate } from './api';
  import DashboardThumb from './DashboardThumb.svelte';

  let {
    templates,
    columns,
    rows,
    busyId,
    onUse,
  }: {
    templates: DashboardTemplate[];
    columns: number;
    rows: number;
    /** The template being created, so its button waits and the others cannot start a second one. */
    busyId: string;
    onUse: (template: DashboardTemplate) => void;
  } = $props();
</script>

<div class="gallery">
  {#each templates as template (template.id)}
    <article class="template">
      <DashboardThumb widgets={template.widgets} theme={template.theme} {columns} {rows} />
      <strong>{template.name}</strong>
      <p class="muted small">{template.description}</p>
      <button type="button" class="secondary" disabled={busyId !== ''} onclick={() => onUse(template)}>
        {busyId === template.id ? 'Creating…' : 'Use this template'}
      </button>
    </article>
  {/each}
</div>

<style>
  .gallery {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 230px), 1fr));
    gap: 14px;
  }
  .template {
    display: grid;
    grid-template-rows: auto auto 1fr auto;
    gap: 8px;
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    min-width: 0;
  }
</style>
