<script lang="ts">
  // The themes' palettes, shapes and typefaces.
  import '../themes.css';

  // A small screen in a theme: a clock tile and a weather tile, drawn with
  // the theme's own colours, tile shape, texture and typeface, so a theme can
  // be judged by what it does to figures and words.
  let { theme, font = '' }: { theme: string; font?: string } = $props();
</script>

<!-- The outer box is the size container; the inner one can then use its
     cqw units for its own padding and gaps too. -->
<div class="sample screen" data-theme={theme} data-font={font || undefined} aria-hidden="true">
  <div class="inner">
    <div class="tile clock">
      <span class="figure">21:45</span>
      <span class="muted">Saturday 10 October</span>
    </div>
    <div class="tile weather">
      <span class="figure">14°</span>
      <span class="accent"></span>
    </div>
  </div>
</div>

<style>
  .sample {
    aspect-ratio: 16 / 9;
    overflow: hidden;
    border-radius: var(--r-md);
    background: var(--backdrop), var(--surface-0);
    color: var(--text-primary);
    font-family: var(--font-ui);
    font-feature-settings: var(--numerals);
    letter-spacing: var(--tracking);
    text-shadow: var(--text-glow);
    container-type: inline-size;
  }
  .inner {
    display: grid;
    grid-template-columns: 3fr 2fr;
    gap: 4cqw;
    height: 100%;
    padding: 4cqw;
    box-sizing: border-box;
  }
  .tile {
    display: grid;
    align-content: center;
    gap: 2cqw;
    min-width: 0;
    padding: 4cqw;
    background: var(--tile-image), var(--surface-1);
    border: min(var(--widget-border-width, 1px), 2px) solid var(--widget-border);
    border-radius: calc(3cqw * var(--radius-scale, 1));
    overflow: hidden;
  }
  .figure {
    font-size: 13cqw;
    font-weight: var(--weight-figure, 500);
    line-height: 1;
    letter-spacing: -0.02em;
    font-variant-numeric: tabular-nums;
  }
  .muted {
    color: var(--text-muted);
    font-size: 4.2cqw;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .accent {
    width: 40%;
    height: 1.4cqw;
    border-radius: 1cqw;
    background: var(--accent);
  }
</style>
