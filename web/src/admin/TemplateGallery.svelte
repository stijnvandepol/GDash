<script lang="ts">
  import type { DashboardTemplate } from './api';
  import { thumbnailBoxes } from './templates';

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
    {@const boxes = thumbnailBoxes(template.widgets, columns, rows)}
    <article class="template">
      <div class="thumbnail" role="img" aria-label="Layout with {boxes.map((b) => b.label).join(', ')}">
        {#each boxes as box (box.id)}
          <span class="box" style:left="{box.left}%" style:top="{box.top}%" style:width="{box.width}%" style:height="{box.height}%">
            <span>{box.label}</span>
          </span>
        {/each}
      </div>
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
  .thumbnail {
    position: relative;
    aspect-ratio: 16 / 9;
    background: #0b0c0e;
    border-radius: 6px;
    overflow: hidden;
    container-type: inline-size;
  }
  .box {
    position: absolute;
    box-sizing: border-box;
    padding: 1px;
  }
  .box > span {
    display: grid;
    place-items: center;
    width: 100%;
    height: 100%;
    background: #1f232a;
    color: #e8eaee;
    border-radius: 3px;
    font-size: max(9px, 3.4cqw);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
