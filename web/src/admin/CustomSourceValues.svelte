<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError, type CustomSource, type NamedList, type NamedValue } from './api';
  import { MAX_COLUMNS, describeSize, suggestName } from './customSources';
  import { previewOf } from './customWidget';
  import { relativeTo } from './jsonPath';
  import JsonTree from './JsonTree.svelte';

  // internal/custom allows this many per source.
  const MAX_VALUES = 40;
  const MAX_LISTS = 10;

  // The values of one API with a name, so the dashboard editor offers
  // "Outside" instead of current.temperature_2m. Values are picked in the
  // answer; paths can also be typed, as this is the technical side.
  let {
    source,
    onsaved,
    onclose,
    onerror,
  }: {
    source: CustomSource;
    onsaved: (saved: CustomSource) => Promise<void>;
    onclose: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  // The form starts from the source once; a reload of the list must not
  // overwrite what is being named.
  // svelte-ignore state_referenced_locally
  let values = $state<NamedValue[]>(source.values.map((v) => ({ ...v })));
  // svelte-ignore state_referenced_locally
  let lists = $state<NamedList[]>(source.lists.map((l) => ({ ...l, columns: l.columns.map((c) => ({ ...c })) })));
  let doc = $state<unknown>(undefined);
  let docNote = $state('');
  let fetching = $state(false);
  // Where a click in the answer goes: a new value, or a column of a list.
  let target = $state<'value' | number>('value');
  let notice = $state('');
  let fieldErrors = $state<Record<string, string>>({});
  let error = $state('');
  let busy = $state(false);

  const taken = $derived([...values.map((v) => v.name), ...lists.map((l) => l.name)]);

  onMount(() => {
    if (source.sample !== null && source.sample !== undefined) useSample();
  });

  function useSample() {
    doc = source.sample;
    docNote = 'The sample answer saved with this API.';
  }

  async function fetchNow() {
    fetching = true;
    try {
      const test = await api.testCustomSource(source.id);
      doc = test.data;
      docNote = `A fresh answer (${describeSize(test.bytes)})${test.truncated ? ', shortened for this view' : ''}.`;
    } catch (err) {
      docNote = onerror(err);
    } finally {
      fetching = false;
    }
  }

  function pick(path: string, value: unknown) {
    if (typeof target === 'number') {
      addColumn(target, path);
    } else if (Array.isArray(value)) {
      if (lists.length >= MAX_LISTS) return say(`An API has at most ${MAX_LISTS} lists.`);
      const name = suggestName(path, taken);
      lists = [...lists, { name, path, columns: [] }];
      target = lists.length - 1;
      say(`Added the list ${name}. Click values inside it to add columns.`);
    } else {
      if (values.length >= MAX_VALUES) return say(`An API has at most ${MAX_VALUES} named values.`);
      const name = suggestName(path, taken);
      values = [...values, { name, path }];
      say(`Added ${name}.`);
    }
  }

  function addColumn(index: number, clicked: string) {
    const list = lists[index]!;
    const path = relativeTo(list.path, clicked);
    if (path === null) return say(`That value is not inside the list ${list.name}. Click a value inside it.`);
    if (list.columns.length >= MAX_COLUMNS) return say(`A list shows at most ${MAX_COLUMNS} columns.`);
    const name = suggestName(path, list.columns.map((c) => c.name));
    list.columns = [...list.columns, { name, path }];
    say(`Added the column ${name} to ${list.name}.`);
  }

  function say(text: string) {
    notice = text;
  }

  function removeList(index: number) {
    lists = lists.filter((_, i) => i !== index);
    if (target === index) target = 'value';
    else if (typeof target === 'number' && target > index) target--;
  }

  async function save() {
    error = '';
    fieldErrors = {};
    busy = true;
    try {
      // Unset units and decimals are left out, as the server stores them.
      const clean = values.map(({ name, path, unit, decimals }) => ({
        name,
        path,
        ...(unit ? { unit } : {}),
        ...(typeof decimals === 'number' ? { decimals } : {}),
      }));
      await onsaved(await api.saveCustomSourceValues(source.id, { values: clean, lists }));
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = Object.fromEntries(err.fields.map((f) => [f.field, f.message]));
        error = 'Some values need attention; see the messages above.';
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  function decimalsFrom(raw: string): number | undefined {
    return raw === '' ? undefined : Number(raw);
  }
</script>

<div class="named">
  <div class="intro">
    <h3>Values of {source.name}</h3>
    <p class="small muted">
      Give the values a dashboard should show a name. In the dashboard editor you then choose them by name, without
      paths.
    </p>
  </div>

  <div class="answer">
    <div class="inline">
      {#if source.sample !== null && source.sample !== undefined}
        <button type="button" class="link" onclick={useSample}>Use the sample</button>
      {/if}
      <button type="button" class="link" disabled={fetching} onclick={fetchNow}>{fetching ? 'Fetching…' : 'Fetch the API now'}</button>
    </div>
    {#if docNote}<p class="small muted">{docNote}</p>{/if}
    {#if doc !== undefined}
      {#if lists.length > 0}
        <fieldset class="target">
          <legend>A click adds</legend>
          <label class="check"><input type="radio" name="target-{source.id}" checked={target === 'value'} onchange={() => (target = 'value')} /> A value, or a list with Use this list</label>
          {#each lists as list, i (i)}
            <label class="check"><input type="radio" name="target-{source.id}" checked={target === i} onchange={() => (target = i)} /> A column to {list.name || `list ${i + 1}`}</label>
          {/each}
        </fieldset>
      {:else}
        <p class="small muted">Click a value to add it, or Use this list for a list of items.</p>
      {/if}
      <JsonTree value={doc} onpick={pick} />
      <p class="small" role="status">{notice}</p>
    {/if}
  </div>

  <fieldset class="group">
    <legend>Values</legend>
    {#if values.length === 0}
      <p class="small muted">None yet: click a value in the answer, or add one by its path.</p>
    {:else}
      <div class="grid head-row" aria-hidden="true">
        <span>Name</span><span>Path</span><span>Unit</span><span>Decimals</span><span></span>
      </div>
    {/if}
    {#each values as value, i (i)}
      <div class="grid">
        <input bind:value={value.name} maxlength="40" aria-label="Name of value {i + 1}" placeholder="Name"
          aria-invalid={fieldErrors[`values[${i}].name`] ? 'true' : undefined} />
        <input class="mono" bind:value={value.path} spellcheck="false" autocomplete="off" aria-label="Path of value {i + 1}" placeholder="Path"
          aria-invalid={fieldErrors[`values[${i}].path`] ? 'true' : undefined} />
        <input bind:value={value.unit} maxlength="12" aria-label="Unit of value {i + 1}" placeholder="Unit" />
        <input type="number" min="0" max="6" value={value.decimals ?? ''} aria-label="Decimals of value {i + 1}" placeholder="Decimals"
          onchange={(e) => (value.decimals = decimalsFrom((e.target as HTMLInputElement).value))} />
        <button type="button" class="link danger" onclick={() => (values = values.filter((_, j) => j !== i))}>Remove</button>
        {#if previewOf(doc, value.path)}<span class="now small muted">Now: {previewOf(doc, value.path)}</span>{/if}
      </div>
      {#each ['name', 'path', 'unit', 'decimals'] as key (key)}
        {#if fieldErrors[`values[${i}].${key}`]}<span class="field-error">{value.name || `Value ${i + 1}`}: {fieldErrors[`values[${i}].${key}`]}</span>{/if}
      {/each}
    {/each}
    {#if fieldErrors.values}<span class="field-error">{fieldErrors.values}</span>{/if}
    {#if values.length < MAX_VALUES}
      <button type="button" class="link" onclick={() => (values = [...values, { name: '', path: '' }])}>Add a value by its path</button>
    {/if}
  </fieldset>

  <fieldset class="group">
    <legend>Lists</legend>
    {#if lists.length === 0}
      <p class="small muted">None yet: choose Use this list next to a list in the answer.</p>
    {/if}
    {#each lists as list, i (i)}
      <div class="list-block">
        <div class="pair">
          <input bind:value={list.name} maxlength="40" aria-label="Name of list {i + 1}" placeholder="Name"
            aria-invalid={fieldErrors[`lists[${i}].name`] ? 'true' : undefined} />
          <input class="mono" bind:value={list.path} spellcheck="false" autocomplete="off" aria-label="Path of list {i + 1}" placeholder="Path of the list"
            aria-invalid={fieldErrors[`lists[${i}].path`] ? 'true' : undefined} />
          <button type="button" class="link danger" onclick={() => removeList(i)}>Remove</button>
        </div>
        {#each list.columns as column, j (j)}
          <div class="pair column">
            <input bind:value={column.name} maxlength="40" aria-label="Name of column {j + 1} of list {i + 1}" placeholder="Column name" />
            <input class="mono" bind:value={column.path} spellcheck="false" autocomplete="off" aria-label="Path of column {j + 1} of list {i + 1}" placeholder="Path in an item" />
            <button type="button" class="link danger" onclick={() => (list.columns = list.columns.filter((_, k) => k !== j))}>Remove</button>
          </div>
        {/each}
        {#if list.columns.length === 0}
          <p class="small muted column">Choose “A column to {list.name || `list ${i + 1}`}” above, then click values inside the list.</p>
        {/if}
        {#each Object.entries(fieldErrors).filter(([f]) => f.startsWith(`lists[${i}]`)) as [field, message] (field)}
          <span class="field-error">{list.name || `List ${i + 1}`}: {message}</span>
        {/each}
      </div>
    {/each}
    {#if fieldErrors.lists}<span class="field-error">{fieldErrors.lists}</span>{/if}
  </fieldset>

  <div class="inline">
    <button type="button" disabled={busy} onclick={save}>{busy ? 'Saving…' : 'Save values'}</button>
    <button type="button" class="link" onclick={onclose}>Close</button>
  </div>
  {#if error}<p class="error">{error}</p>{/if}
</div>

<style>
  .named {
    display: grid;
    gap: var(--s-4);
    min-width: 0;
    padding: var(--s-4) 0 var(--s-2);
  }
  .intro {
    display: grid;
    gap: var(--s-1);
  }
  h3 {
    font-size: var(--fs-md);
  }
  .answer {
    display: grid;
    gap: var(--s-2);
    min-width: 0;
    padding: var(--s-3);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  fieldset {
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  legend {
    padding: 0;
    margin-bottom: var(--s-2);
    font-size: var(--fs-sm);
    font-weight: 550;
    color: var(--text-soft);
  }
  .group {
    display: grid;
    gap: var(--s-2);
  }
  .target {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1) var(--s-4);
  }
  .target legend {
    margin-bottom: var(--s-1);
  }
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 400;
  }
  /* A value per row: name, path, unit, decimals, remove. */
  .grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr) 5.5rem 6.5rem 4rem;
    gap: var(--s-2);
    align-items: center;
  }
  .head-row {
    color: var(--muted);
    font-size: var(--fs-xs);
    font-weight: 550;
  }
  /* The header row names the columns; the placeholders only stand in for
     it on a narrow screen, where it is hidden. */
  .grid input::placeholder {
    color: transparent;
  }
  .now {
    grid-column: 1 / -1;
    font-family: var(--mono);
    overflow-wrap: anywhere;
  }
  .list-block {
    display: grid;
    gap: var(--s-2);
    padding: var(--s-3) 0;
    border-top: 1px solid var(--border);
  }
  /* A list or a column: name and path, in the columns of the values. */
  .pair {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr) 5.5rem 6.5rem 4rem;
    gap: var(--s-2);
    align-items: center;
  }
  .pair > input:nth-child(2) {
    grid-column: 2 / 5;
  }
  /* A column sits under its list: its name is indented, its path lines up. */
  .column > input:first-child,
  p.column {
    margin-left: var(--s-5);
    width: calc(100% - var(--s-5));
  }
  .mono {
    font-family: var(--mono);
    font-size: var(--fs-sm);
  }
  @container page (max-width: 640px) {
    .head-row {
      display: none;
    }
    .grid input::placeholder {
      color: var(--muted);
    }
    /* Name and path on lines of their own, unit and decimals below. */
    .grid {
      grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
    }
    .grid > input:nth-child(-n + 2) {
      grid-column: 1 / -1;
    }
    .pair {
      grid-template-columns: minmax(0, 1fr) auto;
    }
    .pair > input:nth-child(2) {
      grid-column: 1 / -1;
      grid-row: 2;
    }
    /* On two lines the whole column is indented. */
    .column > input:first-child,
    p.column {
      margin-left: 0;
      width: 100%;
    }
    .column {
      padding-left: var(--s-3);
    }
  }
</style>
