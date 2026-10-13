<script lang="ts">
  import type { CustomSource } from './api';
  import CustomRules from './CustomRules.svelte';
  import { MAX_PAIRS, checkRule, pairs, rules, strings, subjects, valueSettings } from './customWidget';
  import type { Preset } from './presets';
  import { togglePair, viewOf, viewSettings } from './presetWidget';

  // The settings of a widget that shows a device or service with a preset,
  // such as a P1 meter: which of its views, the title and which of its
  // values, all by name. Underneath it is a Custom widget; its source is
  // fixed to the preset's setups, so there are no addresses or paths here.
  let {
    preset,
    sources,
    settings,
    onchange,
  }: {
    preset: Preset;
    /** The setups of the preset; the widget shows one of them. */
    sources: CustomSource[];
    settings: Record<string, unknown>;
    /** Called with the full new settings after every edit. */
    onchange: (settings: Record<string, unknown>) => void;
  } = $props();

  // A list widget shows 1 to 3 columns; internal/custom allows the same.
  const MAX_COLUMNS = 3;

  const text = (key: string) => (typeof settings[key] === 'string' ? (settings[key] as string) : '');
  const mode = $derived(text('mode') || 'value');
  const source = $derived(sources.find((s) => s.id === settings.source));
  const view = $derived(viewOf(preset, settings));
  const shownPairs = $derived(pairs(settings.pairs));
  const fields = $derived(strings(settings.fields));
  const list = $derived(preset.lists.find((l) => l.path === text('path')));
  const ruleSubjects = $derived(subjects(mode, preset.values, list && { ...list, columns: list.columns.filter((c) => fields.includes(c.path)) }));
  const instanceLabel = $derived(preset.instance.charAt(0).toUpperCase() + preset.instance.slice(1));
  const ids = $derived(`widget-${preset.id}`);

  /** Applies changes to several settings at once; an empty one is removed. */
  function update(changes: Record<string, unknown>) {
    const next = { ...settings };
    for (const [key, value] of Object.entries(changes)) {
      if (value === '' || value === undefined || (Array.isArray(value) && value.length === 0)) delete next[key];
      else next[key] = value;
    }
    onchange(next);
  }

  function pickView(id: string) {
    const picked = preset.views.find((v) => v.id === id);
    if (picked) onchange(viewSettings(preset, picked, Number(settings.source)));
  }

  function pickValue(path: string) {
    const value = preset.values.find((v) => v.path === path);
    if (value) update(valueSettings(value));
  }

  function toggleColumn(path: string, on: boolean) {
    if (!list) return;
    update({ fields: list.columns.map((c) => c.path).filter((p) => (p === path ? on : fields.includes(p))) });
  }
</script>

<p class="small muted">
  Shows {source?.name ?? preset.name}. Set it up or change it under Integrations, {preset.name}.
</p>
{#if sources.length > 1}
  <label>
    {instanceLabel}
    <select value={settings.source} onchange={(e) => update({ source: Number((e.target as HTMLSelectElement).value) })}>
      {#each sources as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
    </select>
  </label>
{/if}
{#if preset.views.length > 1}
  <label>
    Show
    <select value={view?.id ?? ''} onchange={(e) => pickView((e.target as HTMLSelectElement).value)}>
      {#if !view}<option value="">As changed</option>{/if}
      {#each preset.views as v (v.id)}<option value={v.id}>{v.name}</option>{/each}
    </select>
  </label>
{/if}
<label>
  Title <input value={text('title')} maxlength="60" oninput={(e) => update({ title: (e.target as HTMLInputElement).value })} />
</label>

{#if mode === 'value'}
  <label>
    Value
    <select value={text('path')} onchange={(e) => pickValue((e.target as HTMLSelectElement).value)}>
      {#each preset.values as v (v.path)}<option value={v.path}>{v.name}</option>{/each}
    </select>
  </label>
{:else if mode === 'list' && list}
  <fieldset>
    <legend>Columns</legend>
    {#each list.columns as c (c.path)}
      {@const on = fields.includes(c.path)}
      <label class="check">
        <input type="checkbox" checked={on} disabled={on ? fields.length === 1 : fields.length >= MAX_COLUMNS} onchange={(e) => toggleColumn(c.path, (e.target as HTMLInputElement).checked)} />
        {c.name}
      </label>
    {/each}
  </fieldset>
{:else if mode === 'keyValues' || mode === 'badges'}
  <fieldset aria-describedby="{ids}-values-hint">
    <legend>Values</legend>
    {#each preset.values as v (v.path)}
      {@const on = shownPairs.some((p) => p.path === v.path)}
      <label class="check">
        <input
          type="checkbox"
          checked={on}
          disabled={on ? shownPairs.length === 1 : shownPairs.length >= MAX_PAIRS}
          onchange={(e) => update({ pairs: togglePair(preset, settings.pairs, v, (e.target as HTMLInputElement).checked) })}
        />
        {v.name}
      </label>
    {/each}
    <p class="small muted" id="{ids}-values-hint">Up to {MAX_PAIRS}, in this order.</p>
  </fieldset>
{/if}

<CustomRules
  rules={rules(settings.rules)}
  {mode}
  subjects={ruleSubjects}
  onchange={(next) => update({ rules: next })}
  oncheck={(when) => checkRule(source, when, mode, text('path'))}
/>

<style>
  fieldset {
    display: grid;
    gap: var(--s-1);
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  legend {
    padding: 0 0 var(--s-1);
    font-weight: 550;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-weight: 400;
  }
</style>
