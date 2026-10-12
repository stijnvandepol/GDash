<script lang="ts">
  import type { WidgetCatalogue } from './api';
  import { fontName, themeName } from './styles';
  import StylePicker from './StylePicker.svelte';

  // The top of the dashboard editor: back, name, style, preview and save.
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
</style>
