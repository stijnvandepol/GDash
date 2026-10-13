<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, type CustomSource, type CustomTest, type SourceStatus } from './api';
  import { describeStatus } from './status';
  import { describeInterval, describeNamed, describePush, describeSize, hostOf, isPushLate } from './customSources';
  import CustomPushInstructions from './CustomPushInstructions.svelte';
  import CustomSourceForm from './CustomSourceForm.svelte';
  import CustomSourceValues from './CustomSourceValues.svelte';
  import JsonTree from './JsonTree.svelte';
  import { ownSources } from './presets';

  // internal/custom allows this many sources.
  const MAX_SOURCES = 20;
  const TIME = new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' });

  let {
    status,
    onchanged,
    onmanagetokens,
    onerror,
  }: {
    /** Data hub status by key; a source's key is custom.source.<id>. */
    status: Record<string, SourceStatus>;
    onchanged: () => Promise<void>;
    /** Opens the API tokens, which a system that sends data needs. */
    onmanagetokens: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  // The household's own APIs; those set up with a preset are under its own card.
  let sources = $state<CustomSource[]>([]);
  // All sources, which count towards the limit together.
  let total = $state(0);
  let error = $state('');
  // 'new', the id of the source being edited, or null.
  let editing = $state<'new' | number | null>(null);
  // The id of the source whose values are being named, or null.
  let naming = $state<number | null>(null);
  // The id of the source whose sending instructions are open, or null.
  let sending = $state<number | null>(null);
  let testing = $state<number | null>(null);
  let result = $state<{ id: number; test: CustomTest } | null>(null);
  let picked = $state('');

  onMount(() => void load());

  async function load() {
    try {
      const all = await api.customSources();
      sources = ownSources(all);
      total = all.length;
    } catch (err) {
      error = onerror(err);
    }
  }

  async function saved(source: CustomSource) {
    toast.success(`${source.name} is saved.`);
    // A new source that is sent to GDash needs its address and token next.
    if (editing === 'new' && source.mode === 'push') sending = source.id;
    editing = null;
    await load();
    await onchanged();
  }

  async function savedValues(source: CustomSource) {
    toast.success(`The values of ${source.name} are saved.`);
    naming = null;
    await load();
  }

  async function remove(source: CustomSource) {
    const message = 'Custom widgets that show it then say that their API is not available. Its secrets are deleted.';
    if (!(await ask({ title: `Remove ${source.name}?`, message, action: 'Remove' }))) return;
    error = '';
    try {
      await api.deleteCustomSource(source.id);
      if (result?.id === source.id) result = null;
      if (sending === source.id) sending = null;
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
    Show data from your own API that answers with JSON, or JSON that your own system, such as a Homey or Home
    Assistant, sends to GDash. Add it here and name the values it should show under Values; then add a Custom API
    widget to a dashboard and choose them by name. Only APIs that a dashboard shows are fetched. For a device or
    service that has its own card under Integrations, such as a HomeWizard meter or Pi-hole, use that card instead.
  </p>
  {#if sources.length > 0}
    <ul class="list">
      {#each sources as source (source.id)}
        {@const st = status[`custom.source.${source.id}`]}
        {@const pushed = source.mode === 'push'}
        {@const lastPushAt = st?.updatedAt ?? source.lastPushAt}
        <li>
          <div class="grow">
            <strong>{source.name}</strong>
            {#if pushed}
              <span class="muted small">Sent to GDash · Expected {describeInterval(source.intervalSeconds).toLowerCase()} · {describeNamed(source)}</span>
            {:else}
              <span class="muted small">{source.method} {hostOf(source.url)} · {describeInterval(source.intervalSeconds)} · {describeNamed(source)}</span>
            {/if}
            {#if source.unreadable}
              <span class="field-error">Its secrets cannot be read on this server. Edit it and enter them again.</span>
            {:else if pushed}
              <span class="small" class:late={isPushLate(lastPushAt, source.intervalSeconds)}>{describePush(lastPushAt, source.intervalSeconds)}</span>
            {:else if st}
              <span class="small" class:error={st.failing}>{describeStatus(st)}</span>
            {:else}
              <span class="small muted">Not fetched: no dashboard shows it.</span>
            {/if}
          </div>
          {#if pushed}
            <button class="link" onclick={() => (sending = sending === source.id ? null : source.id)} aria-expanded={sending === source.id}>How to send</button>
          {/if}
          <button class="link" disabled={testing !== null} onclick={() => test(source)}>
            {testing === source.id ? 'Loading…' : pushed ? 'Last data received' : 'Test'}
          </button>
          <button class="link" onclick={() => (naming = naming === source.id ? null : source.id)} aria-expanded={naming === source.id}>Values</button>
          <button class="link" onclick={() => (editing = source.id)}>Edit</button>
          <button class="link danger" onclick={() => remove(source)}>Remove</button>
        </li>
        {#if editing === source.id}
          <li class="form-row">
            <CustomSourceForm {source} onsaved={saved} oncancel={() => (editing = null)} {onerror} />
          </li>
        {/if}
        {#if sending === source.id}
          <li class="form-row">
            <CustomPushInstructions {source} {lastPushAt} onclose={() => (sending = null)} {onmanagetokens} {onerror} />
          </li>
        {/if}
        {#if naming === source.id}
          <li class="form-row">
            <CustomSourceValues {source} onsaved={savedValues} onclose={() => (naming = null)} {onerror} />
          </li>
        {/if}
        {#if result?.id === source.id}
          <li class="form-row">
            <div class="result">
              <p class="small muted">
                {result.test.receivedAt ? `The last data received, at ${TIME.format(new Date(result.test.receivedAt))}` : 'The answer'}
                ({describeSize(result.test.bytes)}){result.test.truncated ? ', shortened for this view' : ''}. Click a value to see
                its path.
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
  {:else if total < MAX_SOURCES}
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
  .late {
    color: var(--warning);
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
