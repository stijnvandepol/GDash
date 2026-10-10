<script lang="ts">
  import type { WidgetType } from './api';
  import WidgetSettings from './WidgetSettings.svelte';

  // The selected widget's settings, under the TV like the controls under a
  // screen: its name and removal on one line, the settings below.
  let {
    label,
    type,
    settings,
    onchange,
    onremove,
  }: {
    label: string;
    /** Undefined for a widget type this server does not know. */
    type: WidgetType | undefined;
    settings: Record<string, unknown>;
    onchange: (settings: Record<string, unknown>) => void;
    onremove: () => void;
  } = $props();
</script>

<div class="inspector">
  <div class="head">
    <h2><span class="muted">Selected</span> {label}</h2>
    <button class="link danger" onclick={onremove}>Remove widget</button>
  </div>
  {#if type}
    <WidgetSettings {type} {settings} {onchange} />
  {/if}
</div>

<style>
  .inspector {
    display: grid;
    gap: var(--s-3);
    max-width: 52rem;
    padding: var(--s-4) 0;
    border-top: 1px solid var(--rule);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--s-2) var(--s-4);
  }
  h2 {
    font-size: var(--fs-lg);
  }
  .muted {
    font-weight: 450;
  }
</style>
