<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError, type CustomSource } from './api';
  import { errorMessage } from './errors';
  import JsonTree from './JsonTree.svelte';
  import CustomRules from './CustomRules.svelte';
  import {
    MAX_FIELDS,
    MAX_PAIRS,
    MODES,
    checkContext,
    pairs,
    pathFor,
    previewOf,
    rules,
    strings,
    type Target,
  } from './customWidget';

  let {
    settings,
    onchange,
  }: {
    settings: Record<string, unknown>;
    /** Called with the full new settings after every edit. */
    onchange: (settings: Record<string, unknown>) => void;
  } = $props();

  let sources = $state<CustomSource[]>([]);
  let loadError = $state('');
  // The answer the tree shows: the source's sample or a fetched one.
  let doc = $state<unknown>(undefined);
  let docNote = $state('');
  let fetching = $state(false);
  let target = $state<Target>({ kind: 'path' });

  const text = (key: string) => (typeof settings[key] === 'string' ? (settings[key] as string) : '');
  const mode = $derived(text('mode') || 'value');
  const sourceId = $derived(typeof settings.source === 'number' ? settings.source : undefined);
  const source = $derived(sources.find((s) => s.id === sourceId));
  const fields = $derived(strings(settings.fields));
  const pairList = $derived(pairs(settings.pairs));

  onMount(async () => {
    try {
      sources = await api.customSources();
    } catch (err) {
      loadError = errorMessage(err, () => {});
    }
  });

  function set(key: string, value: unknown) {
    const next = { ...settings };
    if (value === '' || value === undefined || (Array.isArray(value) && value.length === 0)) delete next[key];
    else next[key] = value;
    onchange(next);
  }

  function useSample() {
    doc = source?.sample ?? undefined;
    docNote = 'The sample answer saved with this API.';
  }

  async function fetchNow() {
    if (sourceId === undefined) return;
    fetching = true;
    try {
      const test = await api.testCustomSource(sourceId);
      doc = test.data;
      docNote = test.truncated ? 'A fresh answer, shortened for this view.' : 'A fresh answer from the API.';
    } catch (err) {
      docNote = errorMessage(err, () => {});
    } finally {
      fetching = false;
    }
  }

  function pick(clicked: string) {
    const to = target;
    const path = pathFor(to, clicked, mode, text('path'));
    if (to.kind === 'path') set('path', path);
    else if (to.kind === 'field') set('fields', fields.map((f, i) => (i === to.index ? path : f)));
    else if (to.kind === 'pair') set('pairs', pairList.map((p, i) => (i === to.index ? { ...p, path } : p)));
    // A rule gets the path added at its end, to build a condition around it.
    else set('rules', rules(settings.rules).map((r, i) => (i === to.index ? { ...r, when: r.when ? `${r.when} ${path}` : path } : r)));
  }

  async function checkRule(when: string): Promise<string> {
    if (doc === undefined) return 'Load the sample or fetch the API first.';
    try {
      const answer = await api.checkRule({ when, data: doc, ...checkContext(doc, mode, text('path')) });
      return answer.error ? `Does not apply: ${answer.error}` : `Gives ${JSON.stringify(answer.result)}`;
    } catch (err) {
      const reason = err instanceof ApiError ? err.fieldMessage('when') : undefined;
      return reason ? `Cannot be read: ${reason}` : errorMessage(err, () => {});
    }
  }
</script>

<label class="inline-label">
  API
  <select value={sourceId ?? ''} onchange={(e) => set('source', Number((e.target as HTMLSelectElement).value) || '')}>
    <option value="">Choose…</option>
    {#each sources as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
  </select>
</label>
{#if loadError}<p class="error small">{loadError}</p>{/if}
{#if sources.length === 0 && !loadError}<p class="small muted">Add an API under Settings, Data sources, Custom APIs first.</p>{/if}
<label class="inline-label">
  Show
  <select value={mode} onchange={(e) => set('mode', (e.target as HTMLSelectElement).value)}>
    {#each MODES as m (m.mode)}<option value={m.mode}>{m.label}</option>{/each}
  </select>
</label>
<label class="inline-label">
  Title <input value={text('title')} maxlength="60" oninput={(e) => set('title', (e.target as HTMLInputElement).value)} />
</label>

{#if sourceId !== undefined}
  <div class="data">
    <div class="inline">
      {#if source?.sample}<button type="button" class="link" onclick={useSample}>Use the sample</button>{/if}
      <button type="button" class="link" disabled={fetching} onclick={fetchNow}>{fetching ? 'Fetching…' : 'Fetch the API now'}</button>
    </div>
    {#if docNote}<p class="small muted">{docNote}</p>{/if}
    {#if doc !== undefined}
      <p class="small muted">Click a value to put its path in the field you last clicked.</p>
      <JsonTree value={doc} onpick={pick} />
    {/if}
  </div>
{/if}

{#if mode !== 'keyValues'}
  <label class="inline-label">
    {mode === 'list' ? 'List' : 'Value'}
    <input value={text('path')} spellcheck="false" autocomplete="off" placeholder={mode === 'list' ? 'data.items' : 'current.temperature_2m'}
      onfocus={() => (target = { kind: 'path' })} oninput={(e) => set('path', (e.target as HTMLInputElement).value)} />
    <span class="hint small muted">{previewOf(doc, text('path'))}</span>
  </label>
{/if}
{#if mode === 'value'}
  <label class="inline-label">Label <input value={text('label')} maxlength="40" oninput={(e) => set('label', (e.target as HTMLInputElement).value)} /></label>
  <label class="inline-label">Unit <input class="short" value={text('unit')} maxlength="12" oninput={(e) => set('unit', (e.target as HTMLInputElement).value)} /></label>
  <label class="inline-label">
    Decimals
    <input type="number" class="short" min="0" max="6" value={typeof settings.decimals === 'number' ? settings.decimals : ''}
      onchange={(e) => { const raw = (e.target as HTMLInputElement).value; set('decimals', raw === '' ? '' : Number(raw)); }} />
  </label>
{:else if mode === 'list'}
  {#each fields as field, i (i)}
    <label class="inline-label">
      Field {i + 1}
      <input value={field} spellcheck="false" autocomplete="off" placeholder="name" onfocus={() => (target = { kind: 'field', index: i })}
        oninput={(e) => set('fields', fields.map((f, j) => (j === i ? (e.target as HTMLInputElement).value : f)))} />
      <button type="button" class="link danger" onclick={() => set('fields', fields.filter((_, j) => j !== i))}>Remove</button>
    </label>
  {/each}
  {#if fields.length < MAX_FIELDS}<button type="button" class="link" onclick={() => set('fields', [...fields, ''])}>Add a field</button>{/if}
{:else}
  {#each pairList as pair, i (i)}
    <div class="pair">
      <input value={pair.label} maxlength="40" aria-label="Label {i + 1}" placeholder="Label"
        oninput={(e) => set('pairs', pairList.map((p, j) => (j === i ? { ...p, label: (e.target as HTMLInputElement).value } : p)))} />
      <input value={pair.path} aria-label="Path {i + 1}" spellcheck="false" autocomplete="off" placeholder="data.temp" onfocus={() => (target = { kind: 'pair', index: i })}
        oninput={(e) => set('pairs', pairList.map((p, j) => (j === i ? { ...p, path: (e.target as HTMLInputElement).value } : p)))} />
      <button type="button" class="link danger" onclick={() => set('pairs', pairList.filter((_, j) => j !== i))}>Remove</button>
      <span class="hint small muted">{previewOf(doc, pair.path)}</span>
    </div>
  {/each}
  {#if pairList.length < MAX_PAIRS}
    <button type="button" class="link" onclick={() => set('pairs', [...pairList, { label: '', path: '' }])}>Add a line</button>
  {/if}
{/if}

<CustomRules
  rules={rules(settings.rules)}
  {mode}
  onchange={(next) => set('rules', next)}
  onfocus={(index) => (target = { kind: 'rule', index })}
  oncheck={doc === undefined ? undefined : checkRule}
/>

<style>
  .inline-label {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    font-weight: 400;
    min-width: 0;
  }
  .inline-label input:not(.short),
  .pair input {
    flex: 1 1 10rem;
  }
  .short {
    width: 5em;
  }
  .pair {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }
  .hint {
    flex-basis: 100%;
    font-family: var(--mono);
    overflow-wrap: anywhere;
  }
  .hint:empty {
    display: none;
  }
  .data {
    display: grid;
    gap: var(--s-2);
    min-width: 0;
  }
</style>
