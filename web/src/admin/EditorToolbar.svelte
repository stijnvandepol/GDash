<script lang="ts">
  import type { WidgetCatalogue } from './api';
  import { WIDGET_LABELS } from './widgetSettings';
  import { fontName, themeName } from './styles';
  import StylePicker from './StylePicker.svelte';

  // The top of the dashboard editor: back, name, style, preview and save,
  // and a button per widget type to add one.
  let {
    name = $bindable(),
    theme = $bindable(),
    font = $bindable(),
    nightTheme = $bindable(),
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
    font: string;
    nightTheme: string;
    catalogue: WidgetCatalogue;
    saving: boolean;
    /** Called when the name or style changes, so the editor knows it is dirty. */
    onchange: () => void;
    onclose: () => void;
    onpreview: () => void;
    onsave: () => void;
    onadd: (type: string) => void;
  } = $props();

  let picking = $state(false);
</script>

<div class="editor-head">
  <button class="link" onclick={onclose}>← Dashboards</button>
  <input class="name" bind:value={name} oninput={onchange} aria-label="Dashboard name" />
  <button class="secondary style" aria-haspopup="dialog" onclick={() => (picking = true)}>
    <span class="style-label">Style</span>
    {themeName(theme)}<span class="muted">· {fontName(theme, font)}</span>{#if nightTheme}<span class="muted">· {themeName(nightTheme)} after sunset</span>{/if}
  </button>
  <button class="secondary" onclick={onpreview}>Preview</button>
  <button onclick={onsave} disabled={saving}>{saving ? 'Saving…' : 'Save'}</button>
</div>

{#if picking}
  <StylePicker
    bind:theme
    bind:font
    bind:nightTheme
    themes={catalogue.themes}
    fonts={catalogue.fonts ?? []}
    {onchange}
    onclose={() => (picking = false)}
  />
{/if}

<div class="palette">
  <span class="palette-label">Add a widget</span>
  {#each Object.keys(catalogue.types) as type (type)}
    <button class="chip" onclick={() => onadd(type)}><span class="plus" aria-hidden="true">+</span>{WIDGET_LABELS[type] ?? type}</button>
  {/each}
</div>

<style>
  .editor-head {
    display: flex;
    gap: var(--s-2) var(--s-3);
    align-items: center;
    flex-wrap: wrap;
  }
  .name {
    flex: 1;
    min-width: 160px;
    font-size: var(--fs-lg);
    font-weight: 650;
    letter-spacing: -0.02em;
  }
  .style {
    gap: 6px;
  }
  .style-label {
    color: var(--muted);
    font-weight: 450;
  }
  .palette {
    display: flex;
    gap: 6px;
    align-items: center;
    flex-wrap: wrap;
    padding: var(--s-3) 0;
    border-block: 1px solid var(--border);
  }
  .palette-label {
    margin-right: var(--s-2);
    color: var(--muted);
    font-size: var(--fs-sm);
    font-weight: 550;
  }
  /* A widget to add: a small key, like the buttons on a remote. */
  .chip {
    min-height: 32px;
    gap: 6px;
    padding: 0 var(--s-3) 0 var(--s-2);
    background: var(--surface);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    font-size: var(--fs-sm);
  }
  .chip:hover:not(:disabled) {
    background: var(--surface);
    border-color: var(--text);
  }
  .plus {
    color: var(--muted);
    font-family: var(--mono);
  }
</style>
