<script lang="ts">
  import { shrinkToFit } from './fit';

  // Smallest text that stays readable across a kitchen.
  const MIN_FONT_PX = 28;

  let { settings }: { settings: Record<string, unknown> | undefined } = $props();

  // Rendered as text: the note is never parsed as HTML.
  const text = $derived(typeof settings?.text === 'string' ? settings.text : '');
</script>

<p class="note" use:shrinkToFit={MIN_FONT_PX}>{text}</p>

<style>
  .note {
    margin: 0;
    width: 100%;
    height: 100%;
    font-size: 48px;
    font-size: min(48px, 42cqh);
    line-height: 1.3;
    color: var(--text-primary);
    white-space: pre-line;
    overflow-wrap: anywhere;
    overflow: hidden;
  }
</style>
