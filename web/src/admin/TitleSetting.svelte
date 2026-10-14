<script lang="ts">
  import { translate } from '../i18n.svelte';
  import { defaultTitle } from '../widgets/titles';

  // The title over a tile, which every widget takes: empty for the widget's
  // own, typed for another, or none at all.
  let {
    widgetType,
    settings,
    max,
    onchange,
  }: {
    widgetType: string;
    settings: Record<string, unknown>;
    max: number;
    onchange: (settings: Record<string, unknown>) => void;
  } = $props();

  // The web interface is in English; the TVs show the title in their language.
  const fallback = $derived(defaultTitle({ type: widgetType, settings }, undefined, (key, values) => translate('en', key, values)));
  const hidden = $derived(settings.hideTitle === true);
  const own = $derived(typeof settings.title === 'string' ? settings.title : '');

  function setTitle(value: string) {
    const next = { ...settings };
    if (value.trim() === '') delete next.title;
    else next.title = value;
    onchange(next);
  }

  function setHidden(hide: boolean) {
    const next = { ...settings };
    if (hide) next.hideTitle = true;
    else delete next.hideTitle;
    onchange(next);
  }
</script>

<div class="title-setting">
  <label class="inline-label">
    Title
    <input value={own} placeholder={fallback ?? 'None'} maxlength={max} disabled={hidden} oninput={(e) => setTitle((e.target as HTMLInputElement).value)} />
  </label>
  <label class="inline-label">
    <input type="checkbox" checked={hidden} onchange={(e) => setHidden((e.target as HTMLInputElement).checked)} />
    No title
  </label>
  <p class="small muted">
    {#if hidden}
      The tile shows no title.
    {:else if own.trim() !== ''}
      The tile shows this title as it is typed.
    {:else if fallback}
      Empty shows “{fallback}”, in the language of the TVs.
    {:else}
      Empty shows no title: this widget explains itself.
    {/if}
  </p>
</div>

<style>
  .title-setting {
    display: grid;
    gap: var(--s-1);
    padding-bottom: var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .inline-label {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    font-weight: 400;
    min-width: 0;
  }
  .inline-label input:not([type='checkbox']) {
    flex: 1 1 12em;
    min-width: 0;
  }
  p {
    margin: 0;
  }
</style>
