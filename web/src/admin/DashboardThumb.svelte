<script lang="ts">
  import type { Widget } from './api';
  import { thumbnailBoxes } from './templates';
  // The themes' palettes, so a thumbnail shows a dashboard in its colours.
  import '../themes.css';

  // A small schematic of a dashboard: its widgets as labelled boxes on the
  // TV grid, in the colours of its theme.
  let {
    widgets,
    theme,
    font = '',
    columns,
    rows,
    labels = true,
  }: {
    widgets: Widget[];
    theme: string;
    /** The dashboard's own typeface, or empty for the theme's. */
    font?: string;
    columns: number;
    rows: number;
    /** False for thumbnails too small to read a label in. */
    labels?: boolean;
  } = $props();

  const boxes = $derived(thumbnailBoxes(widgets, columns, rows));
</script>

<div
  class="thumbnail screen"
  data-theme={theme}
  data-font={font || undefined}
  role="img"
  aria-label={boxes.length > 0 ? `Layout with ${boxes.map((b) => b.label).join(', ')}` : 'Empty layout'}
>
  {#each boxes as box (box.id)}
    <span class="box" style:left="{box.left}%" style:top="{box.top}%" style:width="{box.width}%" style:height="{box.height}%">
      <span class="tile"><span class="label">{labels ? box.label : ''}</span></span>
    </span>
  {/each}
</div>

<style>
  .thumbnail {
    position: relative;
    aspect-ratio: 16 / 9;
    background: var(--backdrop), var(--surface-0);
    font-family: var(--font-ui);
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
    container-type: inline-size;
  }
  .box {
    position: absolute;
    box-sizing: border-box;
    padding: 1px;
  }
  .tile {
    display: grid;
    place-items: center;
    width: 100%;
    height: 100%;
    background: var(--tile-image), var(--surface-1);
    color: var(--text-muted);
    border: min(var(--widget-border-width, 1px), 2px) solid var(--widget-border);
    border-radius: calc(4px * var(--radius-scale, 1));
    /* A TV tile's 16 pixels on a 1920-pixel screen, for motifs sized in em. */
    font-size: calc(100cqw / 120);
    overflow: hidden;
  }
  /* Every second and third tile in its theme's variant, as on a screen. */
  .box:nth-child(3n + 2) > .tile {
    --tile-image: var(--tile-image-2);
  }
  .box:nth-child(3n) > .tile {
    --tile-image: var(--tile-image-3);
  }
  .label {
    max-width: 100%;
    font-size: max(9px, 3.4cqw);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
