<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError, type CustomSource } from './api';
  import { errorMessage } from './errors';
  import CustomRules from './CustomRules.svelte';
  import {
    MAX_PAIRS,
    MODES,
    checkContext,
    defaultLabel,
    findList,
    pairs,
    rules,
    strings,
    subjects,
    valueSettings,
  } from './customWidget';

  // The Custom widget's settings are about how it looks: which API, how to
  // show it and which of its named values. The values are named under
  // Settings; a pick writes the value's path, so a stored widget keeps
  // working when a name changes, and a path without a name shows as
  // "(custom path)".
  let {
    settings,
    onchange,
  }: {
    settings: Record<string, unknown>;
    /** Called with the full new settings after every edit. */
    onchange: (settings: Record<string, unknown>) => void;
  } = $props();

  let sources = $state<CustomSource[]>([]);
  let loaded = $state(false);
  let loadError = $state('');

  const text = (key: string) => (typeof settings[key] === 'string' ? (settings[key] as string) : '');
  const mode = $derived(text('mode') || 'value');
  const sourceId = $derived(typeof settings.source === 'number' ? settings.source : undefined);
  const source = $derived(sources.find((s) => s.id === sourceId));
  const values = $derived(source?.values ?? []);
  const lists = $derived(source?.lists ?? []);
  const fields = $derived(strings(settings.fields));
  const pairList = $derived(pairs(settings.pairs));
  const list = $derived(findList(lists, text('path'), fields));
  const ruleSubjects = $derived(subjects(mode, values, list));
  const named = (path: string) => values.find((v) => v.path === path);

  onMount(async () => {
    try {
      sources = await api.customSources();
    } catch (err) {
      loadError = errorMessage(err, () => {});
    } finally {
      loaded = true;
    }
  });

  /** Applies changes to several settings at once; an empty one is removed. */
  function update(changes: Record<string, unknown>) {
    const next = { ...settings };
    for (const [key, value] of Object.entries(changes)) {
      if (value === '' || value === undefined || (Array.isArray(value) && value.length === 0)) delete next[key];
      else next[key] = value;
    }
    onchange(next);
  }

  function pickValue(path: string) {
    const value = named(path);
    if (value) update(valueSettings(value));
  }

  function pickList(index: string) {
    const picked = lists[Number(index)];
    if (picked) update({ path: picked.path, fields: picked.columns.map((c) => c.path) });
  }

  function pickPair(index: number, path: string) {
    const value = named(path);
    if (!value) return;
    update({ pairs: pairList.map((p, i) => (i === index ? { label: p.label || defaultLabel(mode, value.name), path } : p)) });
  }

  // A rule is checked against the sample, or a fresh answer without one.
  async function checkRule(when: string): Promise<string> {
    if (sourceId === undefined) return 'Choose an API first.';
    try {
      const doc = source?.sample ?? (await api.testCustomSource(sourceId)).data;
      const answer = await api.checkRule({ when, data: doc, ...checkContext(doc, mode, text('path')) });
      return answer.error ? `Does not apply now: ${answer.error}` : `Gives ${JSON.stringify(answer.result)} now`;
    } catch (err) {
      const reason = err instanceof ApiError ? err.fieldMessage('when') : undefined;
      return reason ? `Cannot be read: ${reason}` : errorMessage(err, () => {});
    }
  }
</script>

<label>
  API
  <select value={sourceId ?? ''} onchange={(e) => update({ source: Number((e.target as HTMLSelectElement).value) || '' })}>
    <option value="">Choose…</option>
    {#each sources as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
  </select>
</label>
{#if loadError}<p class="error small">{loadError}</p>{/if}
{#if loaded && sources.length === 0 && !loadError}
  <p class="small muted">Add an API under Integrations, Custom APIs first.</p>
{:else if source && values.length + lists.length === 0}
  <p class="small muted">Name the values of {source.name} under Integrations, Custom APIs, Values; they are listed here.</p>
{/if}
<label>
  Show
  <select value={mode} onchange={(e) => update({ mode: (e.target as HTMLSelectElement).value })}>
    {#each MODES as m (m.mode)}<option value={m.mode}>{m.label}</option>{/each}
  </select>
</label>
<label>
  Title <input value={text('title')} maxlength="60" oninput={(e) => update({ title: (e.target as HTMLInputElement).value })} />
</label>

{#if mode === 'value'}
  <label>
    Value
    <select value={text('path')} onchange={(e) => pickValue((e.target as HTMLSelectElement).value)}>
      {#if text('path') === ''}<option value="">Choose…</option>{:else if !named(text('path'))}<option value={text('path')}>(custom path)</option>{/if}
      {#each values as v (v.path)}<option value={v.path}>{v.name}</option>{/each}
    </select>
  </label>
  <label>Label <input value={text('label')} maxlength="40" oninput={(e) => update({ label: (e.target as HTMLInputElement).value })} /></label>
  <div class="row">
    <label>Unit <input value={text('unit')} maxlength="12" oninput={(e) => update({ unit: (e.target as HTMLInputElement).value })} /></label>
    <label>
      Decimals
      <input
        type="number"
        min="0"
        max="6"
        value={typeof settings.decimals === 'number' ? settings.decimals : ''}
        onchange={(e) => {
          const raw = (e.target as HTMLInputElement).value;
          update({ decimals: raw === '' ? '' : Number(raw) });
        }}
      />
    </label>
  </div>
{:else if mode === 'list'}
  <label>
    List
    <select value={list ? String(lists.indexOf(list)) : text('path') ? 'custom' : ''} onchange={(e) => pickList((e.target as HTMLSelectElement).value)}>
      {#if !list}<option value={text('path') ? 'custom' : ''}>{text('path') ? '(custom list)' : 'Choose…'}</option>{/if}
      {#each lists as l, i (i)}<option value={String(i)}>{l.name}</option>{/each}
    </select>
  </label>
  {#if list}<p class="small muted">Each item shows {list.columns.map((c) => c.name).join(', ')}.</p>{/if}
{:else}
  {#if mode === 'badges'}
    <p class="small muted">A badge shows its label, such as initials, while its value is on: true, a number other than 0, or the text on, home, yes, open or true.</p>
  {/if}
  {#each pairList as pair, i (i)}
    <div class="pair">
      <select aria-label="Value {i + 1}" value={pair.path} onchange={(e) => pickPair(i, (e.target as HTMLSelectElement).value)}>
        {#if pair.path === ''}<option value="">Choose…</option>{:else if !named(pair.path)}<option value={pair.path}>(custom path)</option>{/if}
        {#each values as v (v.path)}<option value={v.path}>{v.name}</option>{/each}
      </select>
      <input
        class="label"
        value={pair.label}
        maxlength="40"
        aria-label="Label {i + 1}"
        placeholder={mode === 'badges' ? 'ST' : 'Label'}
        oninput={(e) => update({ pairs: pairList.map((p, j) => (j === i ? { ...p, label: (e.target as HTMLInputElement).value } : p)) })}
      />
      <button type="button" class="link danger" aria-label="Remove {mode === 'badges' ? 'badge' : 'line'} {i + 1}" onclick={() => update({ pairs: pairList.filter((_, j) => j !== i) })}>
        Remove
      </button>
    </div>
  {/each}
  {#if pairList.length < MAX_PAIRS}
    <button type="button" class="link" onclick={() => update({ pairs: [...pairList, { label: '', path: '' }] })}>
      {mode === 'badges' ? 'Add a badge' : 'Add a line'}
    </button>
  {/if}
{/if}

<CustomRules rules={rules(settings.rules)} {mode} subjects={ruleSubjects} onchange={(next) => update({ rules: next })} oncheck={checkRule} />

<style>
  .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: var(--s-3);
  }
  .pair {
    display: grid;
    grid-template-columns: minmax(0, 1.3fr) minmax(4.5rem, 1fr) auto;
    align-items: center;
    gap: var(--s-2);
  }
</style>
