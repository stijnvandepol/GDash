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
      <h3>{template.name}</h3>
      <p class="muted small">{template.description}</p>
      <button
        type="button"
        class="link use"
        class:busy={busyId === template.id}
        aria-label={busyId === template.id ? undefined : `Use this template: ${template.name}`}
        disabled={busyId !== ''}
        onclick={() => onUse(template)}
      >
        {busyId === template.id ? 'Creating…' : 'Use this template →'}
      </button>
    </article>
  {/each}
</div>

<style>
  .gallery {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 240px), 1fr));
    gap: var(--s-6) var(--s-5);
  }
  /* A phone: two small screens side by side rather than one long column. */
  @container page (max-width: 520px) {
    .gallery {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: var(--s-5) var(--s-3);
    }
  }
  /* No box around a template: the drawn screen is the object. */
  .template {
    display: grid;
    grid-template-rows: auto auto 1fr auto;
    gap: var(--s-2);
    min-width: 0;
  }
  .template :global(.thumbnail) {
    margin-bottom: var(--s-2);
    border-radius: var(--r-md);
  }
  h3 {
    font-size: var(--fs-md);
    font-weight: 650;
  }
  .use {
    justify-self: start;
    margin-left: calc(-1 * var(--s-2));
  }
</style>
