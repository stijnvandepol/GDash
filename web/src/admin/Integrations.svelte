<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { api, type CustomSource, type Settings, type SourceStatus } from './api';
  import { errorMessage } from './errors';
  import { newestOnly } from './newest';
  import Icon from './Icon.svelte';
  import { INTEGRATIONS, type Integration, type PanelContext } from './integrationRegistry';
  import {
    FILTERS,
    TAGS,
    groupByCategory,
    hashFor,
    isAvailable,
    matches,
    searchFromHash,
    type FilterId,
    type IntegrationContext,
  } from './integrations';

  let {
    onSessionEnded,
    onadd,
  }: {
    onSessionEnded: () => void;
    /** Goes to the dashboards, to add a built-in source's widget. */
    onadd: () => void;
  } = $props();

  // A new source is fetched within seconds; reloading shows its status.
  const STATUS_REFRESH_MS = 5_000;

  let settings = $state<Settings | null>(null);
  let status = $state<Record<string, SourceStatus>>({});
  let sources = $state<CustomSource[]>([]);
  let photoCount = $state(0);
  let tokenCount = $state(0);
  let cameraCount = $state(0);
  let webPageCount = $state(0);
  let widgetTypes = $state<string[] | null>(null);
  let loadError = $state('');

  const start = searchFromHash(location.hash);
  let query = $state(start?.query ?? '');
  let filter = $state<FilterId>(start?.filter ?? 'all');
  // The integration whose setup is open, or null.
  let open = $state<string | null>(null);

  const handle = (err: unknown) => errorMessage(err, onSessionEnded);

  const context = $derived<IntegrationContext | null>(
    settings && widgetTypes ? { settings, status, sources, photoCount, tokenCount, cameraCount, webPageCount, widgetTypes } : null,
  );
  const available = $derived(context ? INTEGRATIONS.filter((i) => isAvailable(i, context.widgetTypes, context.settings)) : []);
  const states = $derived(context ? new Map(available.map((i) => [i.id, i.state(context)])) : new Map());
  const shown = $derived(available.filter((i) => matches(i, states.get(i.id)!, query, filter)));
  const groups = $derived(groupByCategory(shown));
  const connected = $derived([...states.values()].filter((s) => s.kind !== 'idle').length);
  const searching = $derived(query.trim() !== '' || filter !== 'all');

  const panelContext = $derived<PanelContext | null>(
    context ? { ...context, onchanged: load, onerror: handle, onopen: openSetup } : null,
  );

  // The search and filter stay in the address, so a link or a reload keeps them.
  $effect(() => {
    const hash = hashFor(query, filter);
    if (location.hash !== hash) history.replaceState(history.state, '', hash);
  });

  onMount(() => {
    void load();
    api
      .widgetTypes()
      .then((c) => (widgetTypes = Object.keys(c.types)))
      .catch((err: unknown) => {
        widgetTypes = [];
        loadError = handle(err);
      });
    const timer = setInterval(() => void load(), STATUS_REFRESH_MS);
    const followHash = () => {
      const next = searchFromHash(location.hash);
      if (next) ({ query, filter } = next);
    };
    window.addEventListener('hashchange', followHash);
    return () => {
      clearInterval(timer);
      window.removeEventListener('hashchange', followHash);
    };
  });

  const fetchAll = newestOnly(() => Promise.all([api.settings(), api.customSources(), api.photos(), api.apiTokens(), api.cameras(), api.webPages()]));

  async function load() {
    try {
      const newest = await fetchAll();
      if (!newest) return;
      const [current, custom, photos, tokens, cameras, pages] = newest.value;
      settings = current.settings;
      status = current.status;
      sources = custom;
      photoCount = photos.length;
      tokenCount = tokens.length;
      cameraCount = cameras.cameras.length;
      webPageCount = pages.length;
      loadError = '';
    } catch (err) {
      loadError = handle(err);
    }
  }

  async function toggle(item: Integration) {
    if (open === item.id) {
      await close();
    } else {
      await openSetup(item.id);
    }
  }

  /** Opens an integration's setup; one hidden by the search is shown first. */
  async function openSetup(id: string) {
    if (!shown.some((i) => i.id === id)) {
      query = '';
      filter = 'all';
    }
    open = id;
    await tick();
    const panel = document.getElementById(`setup-${id}`);
    panel?.focus({ preventScroll: true });
    panel?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }

  async function close() {
    const id = open;
    open = null;
    void load();
    await tick();
    if (id) document.getElementById(`action-${id}`)?.focus();
  }

  function useCustomApi() {
    void openSetup('custom');
  }

  function actionLabel(item: Integration): string {
    if (item.kind === 'builtin') return 'Add to a dashboard';
    if (item.kind === 'service' && states.get(item.id)?.kind !== 'idle') return 'Manage';
    return 'Set up';
  }
</script>

<header class="page-head">
  <div>
    <h1 class="page-title">Integrations</h1>
    <p class="page-lede">
      Everything GDash shows data from. Set up a service, start from a preset, or add a built-in source to a dashboard.
    </p>
  </div>
  {#if context}
    <p class="tally num">
      <span class="lit" aria-hidden="true"></span>{connected} connected <span class="muted">of {available.length}</span>
    </p>
  {/if}
</header>

<div class="finder">
  <div class="search">
    <Icon name="search" size={20} />
    <input
      type="search"
      bind:value={query}
      placeholder="Search, such as solar or train"
      aria-label="Search integrations"
      autocomplete="off"
      spellcheck="false"
      onkeydown={(e) => {
        if (e.key === 'Escape' && query !== '') {
          e.preventDefault();
          query = '';
        }
      }}
    />
  </div>
  <div class="filters" role="group" aria-label="Show">
    {#each FILTERS as f (f.id)}
      <button type="button" class="chip" aria-pressed={filter === f.id} onclick={() => (filter = f.id)}>{f.label}</button>
    {/each}
  </div>
  <p class="sr-only" role="status">
    {searching && context ? `${shown.length} ${shown.length === 1 ? 'integration matches' : 'integrations match'}` : ''}
  </p>
</div>

{#if loadError}<p class="error">{loadError}</p>{/if}

{#if !context}
  {#if !loadError}<p class="loading muted" role="status">Loading…</p>{/if}
{:else if groups.length === 0}
  <section class="empty" aria-labelledby="no-match">
    <h2 id="no-match">Nothing matches {query.trim() ? `“${query.trim()}”` : 'this filter'}</h2>
    <p class="muted">
      If the service has an API that answers with JSON, GDash can show it as a custom API: give it the address and
      name the values to show.
    </p>
    <div class="inline">
      <button type="button" onclick={useCustomApi}>Use a Custom API</button>
      <button type="button" class="link" onclick={() => ((query = ''), (filter = 'all'))}>Show all integrations</button>
    </div>
  </section>
{:else}
  {#each groups as group (group.id)}
    <section class="group" aria-labelledby="group-{group.id}">
      <h2 class="group-title" id="group-{group.id}">
        {group.label} <span class="group-count num" aria-hidden="true">{group.items.length}</span>
      </h2>
      <ul class="grid">
        {#each group.items as item (item.id)}
          {@const state = states.get(item.id)!}
          <li class="item {item.kind}" class:on={state.kind !== 'idle'} class:open={open === item.id}>
            <div class="top">
              <span class="glyph"><Icon name={item.icon} size={20} /></span>
              <div class="text">
                <h3 class="name" id="name-{item.id}">{item.name}</h3>
                <p class="description">{item.description}</p>
              </div>
            </div>
            {#if item.tags.length > 0}
              <ul class="tags" aria-label="Labels">
                {#each item.tags as tag (tag)}<li class="tag">{TAGS[tag]}</li>{/each}
              </ul>
            {/if}
            <div class="foot">
              <p class="state">
                {#if state.kind === 'connected'}
                  <span class="badge ok">Connected</span>
                  {#if state.detail}<span class="detail">{state.detail}</span>{/if}
                {:else if state.kind === 'problem'}
                  <span class="badge warn">{state.detail}</span>
                {:else}
                  <span class="badge">{state.detail}</span>
                {/if}
              </p>
              {#if item.kind === 'builtin'}
                <button type="button" class="link action" id="action-{item.id}" onclick={onadd}>
                  {actionLabel(item)}<span class="sr-only">: {item.name}</span>
                </button>
              {:else if item.panel}
                <button
                  type="button"
                  class="secondary action"
                  id="action-{item.id}"
                  aria-expanded={open === item.id}
                  aria-controls={open === item.id ? `setup-${item.id}` : undefined}
                  onclick={() => toggle(item)}
                >
                  {actionLabel(item)}<span class="sr-only"> {item.name}</span>
                </button>
              {/if}
            </div>
          </li>
          {#if open === item.id && item.panel && panelContext}
            {@const panel = item.panel(panelContext)}
            <li class="setup-row">
              <div class="setup" id="setup-{item.id}" tabindex="-1" role="region" aria-label="Set up {item.name}">
                <div class="setup-head">
                  <span class="small muted">{item.kind === 'preset' ? 'New custom API from a preset' : `Setup of ${item.name}`}</span>
                  <button type="button" class="link" onclick={close}>Close<span class="sr-only"> {item.name}</span></button>
                </div>
                <panel.component {...panel.props} />
              </div>
            </li>
          {/if}
        {/each}
      </ul>
    </section>
  {/each}
  <p class="small muted closing">
    Missing a service? If it has a JSON API, <button type="button" class="link" onclick={useCustomApi}>use a Custom API</button>.
  </p>
{/if}

<style>
  .tally {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    font-weight: 550;
  }
  .lit {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--success);
    box-shadow: 0 0 0 3px var(--success-soft);
  }

  /* The search is the way in: a wide field like the dial of the device,
     the filters under it as the positions of a switch. */
  .finder {
    display: grid;
    gap: var(--s-3);
    padding-bottom: var(--s-6);
  }
  .search {
    position: relative;
    display: flex;
    align-items: center;
    color: var(--muted);
  }
  .search :global(.icon) {
    position: absolute;
    left: 16px;
    pointer-events: none;
  }
  .search input {
    min-height: 52px;
    padding-left: 48px;
    border-width: 1.5px;
    border-color: var(--text);
    font-size: var(--fs-lg);
  }
  .search input::-webkit-search-cancel-button {
    appearance: none;
  }
  .filters {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  button.chip {
    min-height: 34px;
    padding: 0 var(--s-3);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-pill);
    background: transparent;
    color: var(--text-soft);
    font-size: var(--fs-sm);
    transition: none;
  }
  button.chip:hover:not(:disabled) {
    background: var(--neutral-soft);
  }
  button.chip[aria-pressed='true'],
  button.chip[aria-pressed='true']:hover:not(:disabled) {
    background: var(--primary);
    border-color: var(--primary);
    color: var(--on-primary);
  }
  button.chip:active:not(:disabled) {
    transform: none;
  }

  /* A category: a ruled section, its name and how many are in it. */
  .group {
    display: grid;
    gap: var(--s-4);
    padding: var(--s-4) 0 var(--s-6);
    border-top: 1px solid var(--rule);
  }
  .group-title {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    font-size: var(--fs-lg);
  }
  .group-count {
    color: var(--muted);
    font-family: var(--mono);
    font-size: var(--fs-sm);
    font-weight: 500;
  }

  /* The cards. A setup's panel takes a whole row under the card it belongs
     to; dense packing lets the cards after it fill the row it left. */
  .grid {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 300px), 1fr));
    grid-auto-flow: row dense;
    gap: var(--s-3);
  }
  /* What it is says how it looks: a service you connect is a thing on the
     desk (a surface); a built-in source is printed on the panel (a line);
     a preset is a stencil to cut a custom API from (a dashed line). */
  .item {
    display: grid;
    grid-template-rows: auto 1fr auto;
    gap: var(--s-3);
    padding: var(--s-4);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    min-width: 0;
  }
  .item.service {
    background: var(--surface);
  }
  .item.preset {
    border-style: dashed;
    border-color: var(--border-strong);
  }
  .item.open {
    border-color: var(--text);
    box-shadow: inset 0 3px 0 var(--spark);
  }
  .top {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: var(--s-3);
    align-items: start;
  }
  .glyph {
    display: grid;
    place-items: center;
    width: 40px;
    height: 40px;
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    background: var(--bg);
    color: var(--text-soft);
  }
  .item.on .glyph {
    border-color: var(--border-strong);
    color: var(--text);
  }
  .text {
    display: grid;
    gap: 2px;
  }
  .name {
    font-size: var(--fs-md);
    font-weight: 600;
  }
  .description {
    color: var(--muted);
    font-size: var(--fs-sm);
    line-height: 1.4;
  }
  .tags {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1);
    align-self: end;
  }
  .tag {
    padding: 1px 7px;
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    color: var(--text-soft);
    font-size: var(--fs-xs);
    font-weight: 550;
    line-height: 1.5;
  }
  .foot {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--s-2) var(--s-3);
    padding-top: var(--s-3);
    border-top: 1px solid var(--border);
  }
  .state {
    display: grid;
    gap: 2px;
    min-width: 0;
  }
  .detail {
    color: var(--muted);
    font-size: var(--fs-xs);
    padding-left: 15px;
    overflow-wrap: anywhere;
  }
  .action {
    min-height: 34px;
    flex: none;
  }
  button.secondary.action {
    padding: 0 var(--s-3);
  }

  /* The setup of one integration, opened under its card: the section it
     always was, on a surface with the way back above it. */
  .setup-row {
    grid-column: 1 / -1;
    min-width: 0;
  }
  .setup {
    display: grid;
    gap: var(--s-2);
    padding: var(--s-3) var(--s-5) 0;
    background: var(--surface);
    border: 1px solid var(--text);
    border-top: 3px solid var(--spark);
    border-radius: var(--r-lg);
    min-width: 0;
    container: page / inline-size;
  }
  .setup:focus {
    outline: none;
  }
  .setup:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }
  .setup-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--s-3);
  }
  .setup > :global(.card) {
    border-top: 0;
    padding-top: var(--s-2);
  }

  .empty {
    display: grid;
    gap: var(--s-3);
    justify-items: start;
    padding: var(--s-5) 0 var(--s-7);
    border-top: 1px solid var(--rule);
  }
  .empty p {
    max-width: 60ch;
  }
  .closing {
    padding-top: var(--s-2);
  }
  .closing button {
    min-height: 0;
    font-size: inherit;
  }

  @media (max-width: 640px) {
    .setup {
      padding: var(--s-3) var(--s-4) 0;
    }
    .search input {
      font-size: var(--fs-md);
    }
  }
</style>
