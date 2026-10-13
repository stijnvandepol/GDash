<script lang="ts">
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, ApiError, type CustomSource, type SourceStatus } from './api';
  import { hostOf, toInput } from './customSources';
  import { describeStatus } from './status';
  import PresetAnswer from './PresetAnswer.svelte';
  import PresetSourceForm from './PresetSourceForm.svelte';
  import { madeFrom, sourcesFrom, type Preset } from './presets';

  // The setup of a device or service with a preset: each one set up, such
  // as two P1 meters, with how it is doing, and the form for another.
  let {
    preset,
    sources,
    status,
    onchanged,
    onerror,
  }: {
    preset: Preset;
    /** Every custom source; this card shows those set up with the preset. */
    sources: CustomSource[];
    /** Data hub status by key; a source's key is custom.source.<id>. */
    status: Record<string, SourceStatus>;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  // internal/custom allows this many sources, of all presets and own APIs together.
  const MAX_SOURCES = 20;

  const made = $derived(madeFrom(preset, sources));
  // Own APIs that look made from this preset before sources kept it.
  const earlier = $derived(sourcesFrom(preset, sources));
  // 'new', the id of the setup being changed, or null.
  // svelte-ignore state_referenced_locally
  let editing = $state<'new' | number | null>(madeFrom(preset, sources).length === 0 ? 'new' : null);
  let tested = $state<{ id: number; answer: { data: unknown } | { problem: string } | null } | null>(null);
  let error = $state('');

  const ids = $derived(`preset-${preset.id}`);
  const defaultName = $derived(made.length === 0 ? preset.source.name : `${preset.source.name} ${made.length + 1}`);

  async function saved(source: CustomSource) {
    toast.success(editing === 'new' ? `${source.name} is set up.` : `${source.name} is saved.`);
    editing = null;
    await onchanged();
  }

  async function test(source: CustomSource) {
    tested = { id: source.id, answer: null };
    try {
      tested = { id: source.id, answer: { data: (await api.testCustomSource(source.id)).data } };
    } catch (err) {
      tested = { id: source.id, answer: { problem: err instanceof ApiError && err.status !== 401 ? err.message : onerror(err) } };
    }
  }

  async function remove(source: CustomSource) {
    const message = 'Widgets that show it then say that it is not available. Its secrets are deleted.';
    if (!(await ask({ title: `Remove ${source.name}?`, message, action: 'Remove' }))) return;
    error = '';
    try {
      await api.deleteCustomSource(source.id);
      if (tested?.id === source.id) tested = null;
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }

  /** Moves an own API that was made from this preset before under this card. */
  async function adopt(source: CustomSource) {
    error = '';
    try {
      await api.saveCustomSource(source.id, { ...toInput(source), preset: preset.id });
      toast.success(`${source.name} is now under ${preset.name}.`);
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card preset" aria-labelledby="{ids}-title">
  <h2 id="{ids}-title">{preset.name}</h2>
  <p class="muted">{preset.description}.</p>
  {#if made.length > 0}
    <ul class="list" aria-label="{preset.name} set up">
      {#each made as source (source.id)}
        {@const st = status[`custom.source.${source.id}`]}
        <li>
          <div class="grow">
            <strong>{source.name}</strong>
            <span class="muted small">{hostOf(source.url)}</span>
            {#if source.unreadable}
              <span class="field-error">Its secrets cannot be read on this server. Change it and enter them again.</span>
            {:else if st?.failing}
              <span class="small error">{describeStatus(st)}</span>
            {:else}
              <span class="small"><span class="badge ok">Connected</span> <span class="muted">{st ? describeStatus(st) : 'Fetched once a dashboard shows it.'}</span></span>
            {/if}
          </div>
          <button class="link" disabled={tested?.id === source.id && tested.answer === null} onclick={() => test(source)}>Test<span class="sr-only"> {source.name}</span></button>
          <button class="link" aria-expanded={editing === source.id} onclick={() => (editing = editing === source.id ? null : source.id)}>Change<span class="sr-only"> {source.name}</span></button>
          <button class="link danger" onclick={() => remove(source)}>Remove<span class="sr-only"> {source.name}</span></button>
        </li>
        {#if tested?.id === source.id}
          <li class="form-row">
            <PresetAnswer {preset} answer={tested.answer} />
            <button class="link" onclick={() => (tested = null)}>Close</button>
          </li>
        {/if}
        {#if editing === source.id}
          <li class="form-row">
            <PresetSourceForm {preset} {source} {defaultName} onsaved={saved} oncancel={() => (editing = null)} {onerror} />
          </li>
        {/if}
      {/each}
    </ul>
  {/if}
  {#each earlier as source (source.id)}
    <p class="small earlier">
      <span><strong>{source.name}</strong> under Custom APIs looks set up from {preset.name} before it had a card of its own.</span>
      <button class="link" onclick={() => adopt(source)}>Move it here<span class="sr-only">: {source.name}</span></button>
    </p>
  {/each}
  {#if editing === 'new'}
    {#if made.length > 0}<h3 class="another">Another {preset.instance}</h3>{/if}
    <PresetSourceForm {preset} {defaultName} onsaved={saved} oncancel={made.length > 0 ? () => (editing = null) : undefined} {onerror} />
  {:else if sources.length < MAX_SOURCES}
    <div><button class="secondary" onclick={() => (editing = 'new')}>Add another {preset.instance}</button></div>
  {:else}
    <p class="small muted">The limit of {MAX_SOURCES} devices, services and custom APIs together is reached.</p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .form-row {
    display: grid;
    gap: var(--s-2);
  }
  .another {
    font-size: var(--fs-md);
    padding-top: var(--s-2);
  }
  .earlier {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--s-1) var(--s-3);
  }
</style>
