<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, type CustomSource, type CustomTest, type SourceStatus } from './api';
  import { describeStatus } from './status';
  import { describeInterval, describeSize, hostOf } from './customSources';
  import CustomSourceForm from './CustomSourceForm.svelte';
  import JsonTree from './JsonTree.svelte';

  // internal/custom allows this many sources.
  const MAX_SOURCES = 20;

  let {
    status,
    onchanged,
    onerror,
  }: {
    /** Data hub status by key; a source's key is custom.source.<id>. */
    status: Record<string, SourceStatus>;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let sources = $state<CustomSource[]>([]);
  let error = $state('');
  // 'new', the id of the source being edited, or null.
  let editing = $state<'new' | number | null>(null);
  let testing = $state<number | null>(null);
  let result = $state<{ id: number; test: CustomTest } | null>(null);
  let picked = $state('');

  onMount(() => void load());

  async function load() {
    try {
      sources = await api.customSources();
    } catch (err) {
      error = onerror(err);
    }
  }

  async function saved(source: CustomSource) {
    toast.success(`${source.name} is saved.`);
    editing = null;
    await load();
    await onchanged();
  }

  async function remove(source: CustomSource) {
    const message = 'Custom widgets that show it then say that their API is not available. Its secrets are deleted.';
    if (!(await ask({ title: `Remove ${source.name}?`, message, action: 'Remove' }))) return;
    error = '';
    try {
      await api.deleteCustomSource(source.id);
      if (result?.id === source.id) result = null;
      await load();
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }

  async function test(source: CustomSource) {
    error = '';
    picked = '';
    testing = source.id;
    try {
      result = { id: source.id, test: await api.testCustomSource(source.id) };
    } catch (err) {
      result = null;
      error = onerror(err);
    } finally {
      testing = null;
    }
  }
</script>

<section class="card">
  <h2>Custom APIs</h2>
  <p class="muted">
    Show data from any API that answers with JSON, such as a public transport, energy or weather service, or a device
    on your network. Add the API here, then a Custom widget to a dashboard to choose what it shows. Only APIs that a
    dashboard shows are fetched.
  </p>
  {#if sources.length > 0}
    <ul class="list">
      {#each sources as source (source.id)}
        {@const st = status[`custom.source.${source.id}`]}
        <li>
          <div class="grow">
            <strong>{source.name}</strong>
            <span class="muted small">{source.method} {hostOf(source.url)} · {describeInterval(source.intervalSeconds)}</span>
            {#if source.unreadable}
              <span class="field-error">Its secrets cannot be read on this server. Edit it and enter them again.</span>
            {:else if st}
              <span class="small" class:error={st.failing}>{describeStatus(st)}</span>
            {:else}
              <span class="small muted">Not fetched: no dashboard shows it.</span>
            {/if}
          </div>
          <button class="link" disabled={testing !== null} onclick={() => test(source)}>
            {testing === source.id ? 'Testing…' : 'Test'}
          </button>
          <button class="link" onclick={() => (editing = source.id)}>Edit</button>
          <button class="link danger" onclick={() => remove(source)}>Remove</button>
        </li>
        {#if editing === source.id}
          <li class="form-row">
            <CustomSourceForm {source} onsaved={saved} oncancel={() => (editing = null)} {onerror} />
          </li>
        {/if}
        {#if result?.id === source.id}
          <li class="form-row">
            <div class="result">
              <p class="small muted">
                The answer ({describeSize(result.test.bytes)}){result.test.truncated ? ', shortened for this view' : ''}.
                Click a value to see its path.
              </p>
              {#if picked}<p class="small">Path: <code>{picked}</code></p>{/if}
              <JsonTree value={result.test.data} onpick={(path) => (picked = path)} />
              <button class="link" onclick={() => (result = null)}>Close</button>
            </div>
          </li>
        {/if}
      {/each}
    </ul>
  {/if}
  {#if editing === 'new'}
    <CustomSourceForm onsaved={saved} oncancel={() => (editing = null)} {onerror} />
  {:else if sources.length < MAX_SOURCES}
    <div><button class="secondary" onclick={() => (editing = 'new')}>Add a custom API</button></div>
  {:else}
    <p class="small muted">The limit of {MAX_SOURCES} custom APIs is reached.</p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .form-row {
    display: block;
  }
  .result {
    display: grid;
    gap: var(--s-2);
    min-width: 0;
  }
  code {
    font-family: var(--mono);
    overflow-wrap: anywhere;
  }
</style>
