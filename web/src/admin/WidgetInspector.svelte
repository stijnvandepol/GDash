<script lang="ts">
  import type { WidgetType } from './api';
  import WidgetSettings from './WidgetSettings.svelte';

  // The selected widget's settings, in the panel beside the canvas: the
  // way back to the widgets to add, its name and removal, the settings below.
  let {
    label,
    type,
    settings,
    onchange,
    onremove,
    onback,
  }: {
    label: string;
    /** Undefined for a widget type this server does not know. */
    type: WidgetType | undefined;
    settings: Record<string, unknown>;
    onchange: (settings: Record<string, unknown>) => void;
    onremove: () => void;
    /** Leaves the settings for the widgets to add. */
    onback: () => void;
  } = $props();
</script>

<div class="inspector">
  <button type="button" class="link back" onclick={onback}>← Add widgets</button>
  <div class="head">
    <h2 tabindex="-1"><span class="muted">Selected</span> {label}</h2>
    <button type="button" class="link danger" onclick={onremove}>Remove widget</button>
  </div>
  {#if type}
    <div class="settings">
      <WidgetSettings {type} {settings} {onchange} />
    </div>
  {/if}
</div>

<style>
  .inspector {
    display: grid;
    gap: var(--s-3);
    min-width: 0;
  }
  .back {
    justify-self: start;
    color: var(--muted);
  }
  .back:hover:not(:disabled) {
    color: var(--text);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--s-1) var(--s-4);
    padding-bottom: var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  h2 {
    font-size: var(--fs-lg);
  }
  h2:focus {
    outline: none;
  }
  h2:focus-visible {
    outline: 2px solid var(--focus);
  }
  .muted {
    font-weight: 450;
  }
  .settings {
    display: grid;
    gap: var(--s-3);
    min-width: 0;
  }
</style>
