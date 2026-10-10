<script lang="ts">
  import type { WidgetCatalogue } from './api';
  import { WIDGET_LABELS } from './widgetSettings';

  // The top of the dashboard editor: back, name, theme, preview and save,
  // and a button per widget type to add one.
  let {
    name = $bindable(),
    theme = $bindable(),
    catalogue,
    saving,
    onchange,
    onclose,
    onpreview,
    onsave,
    onadd,
  }: {
    name: string;
    theme: string;
    catalogue: WidgetCatalogue;
    saving: boolean;
    /** Called when the name or theme changes, so the editor knows it is dirty. */
    onchange: () => void;
    onclose: () => void;
    onpreview: () => void;
    onsave: () => void;
    onadd: (type: string) => void;
  } = $props();

  const THEME_NAMES: Record<string, string> = {
    'minimal-dark': 'Minimal dark',
    'minimal-light': 'Minimal light',
    glass: 'Glass',
    'warm-light': 'Warm light',
    'high-contrast': 'High contrast',
    night: 'Night',
  };
</script>

<div class="editor-head">
  <button class="link" onclick={onclose}>← Dashboards</button>
  <input class="name" bind:value={name} oninput={onchange} aria-label="Dashboard name" />
  <select bind:value={theme} onchange={onchange} aria-label="Theme">
    {#each catalogue.themes as t (t)}
      <option value={t}>{THEME_NAMES[t] ?? t}</option>
    {/each}
  </select>
  <button class="chip" onclick={onpreview}>Preview</button>
  <button onclick={onsave} disabled={saving}>{saving ? 'Saving…' : 'Save'}</button>
</div>

<div class="palette">
  <span class="muted small">Add:</span>
  {#each Object.keys(catalogue.types) as type (type)}
    <button class="chip" onclick={() => onadd(type)}>{WIDGET_LABELS[type] ?? type}</button>
  {/each}
</div>

<style>
  .editor-head {
    display: flex;
    gap: 10px;
    align-items: center;
    flex-wrap: wrap;
  }
  .name {
    flex: 1;
    min-width: 160px;
    font-weight: 600;
  }
  .palette {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
  }
  .chip {
    padding: 6px 12px;
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--border);
  }
</style>
