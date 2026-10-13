<script lang="ts">
  import Icon from './Icon.svelte';
  import { paletteGroups, type PaletteEntry, type PaletteItem } from './widgetSettings';

  // The widgets to add, beside the canvas: a search on top, then the
  // widget types by category, with the devices and services that are set
  // up as widgets of their own. A click or Enter adds one.
  let {
    types,
    extra = [],
    onadd,
  }: {
    /** The widget types this server has. */
    types: string[];
    /** Entries beside the types, such as each preset that is set up. */
    extra?: PaletteEntry[];
    onadd: (item: PaletteItem) => void;
  } = $props();

  let query = $state('');
  let list: HTMLDivElement;

  const groups = $derived(paletteGroups(types, query, extra));
  const count = $derived(groups.reduce((n, g) => n + g.items.length, 0));

  function add(item: PaletteItem) {
    query = '';
    onadd(item);
  }

  function items(): HTMLButtonElement[] {
    return [...list.querySelectorAll<HTMLButtonElement>('button.item')];
  }

  function onSearchKey(event: KeyboardEvent) {
    if (event.key === 'Escape' && query !== '') {
      event.preventDefault();
      query = '';
    } else if (event.key === 'ArrowDown') {
      event.preventDefault();
      items()[0]?.focus();
    } else if (event.key === 'Enter' && query.trim() !== '') {
      event.preventDefault();
      const first = groups[0]?.items[0];
      if (first) add(first);
    }
  }

  // Up and down move through the widgets; up from the first goes back to
  // the search.
  function onListKey(event: KeyboardEvent) {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    const all = items();
    const at = all.indexOf(event.target as HTMLButtonElement);
    if (at < 0) return;
    event.preventDefault();
    const next = event.key === 'ArrowDown' ? Math.min(at + 1, all.length - 1) : at - 1;
    if (next < 0) list.parentElement?.querySelector<HTMLInputElement>('input[type="search"]')?.focus();
    else all[next]!.focus();
  }
</script>

<div class="palette">
  <h2 class="title">Add widgets</h2>
  <div class="search">
    <Icon name="search" size={16} />
    <input
      type="search"
      bind:value={query}
      placeholder="Search widgets"
      aria-label="Search widgets"
      autocomplete="off"
      spellcheck="false"
      onkeydown={onSearchKey}
    />
  </div>
  <p class="sr-only" role="status">{query.trim() ? `${count} ${count === 1 ? 'widget matches' : 'widgets match'}` : ''}</p>

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="groups" bind:this={list} onkeydown={onListKey}>
    {#each groups as group (group.label)}
      <section class="group" aria-label={group.label}>
        <h3 class="group-label">{group.label}</h3>
        <ul>
          {#each group.items as item (item.key)}
            <li>
              <button
                type="button"
                class="item"
                aria-labelledby="palette-{item.key}-name"
                aria-describedby={item.description ? `palette-${item.key}-text` : undefined}
                onclick={() => add(item)}
              >
                <span class="glyph"><Icon name={item.icon} size={18} /></span>
                <span class="text">
                  <span class="name" id="palette-{item.key}-name">{item.label}</span>
                  {#if item.description}<span class="description" id="palette-{item.key}-text">{item.description}</span>{/if}
                </span>
                <span class="plus" aria-hidden="true">+</span>
              </button>
            </li>
          {/each}
        </ul>
      </section>
    {:else}
      <p class="muted small empty">No widget matches “{query.trim()}”.</p>
    {/each}
  </div>
</div>

<style>
  .palette {
    display: grid;
    gap: var(--s-3);
    min-width: 0;
  }
  .title {
    font-size: var(--fs-md);
  }
  .search {
    position: relative;
    display: flex;
    align-items: center;
    color: var(--muted);
  }
  .search :global(.icon) {
    position: absolute;
    left: 12px;
    pointer-events: none;
  }
  .search input {
    padding-left: 36px;
  }
  /* The browser's own clear cross is blue; Escape clears the search. */
  .search input::-webkit-search-cancel-button {
    appearance: none;
  }
  .groups {
    display: grid;
    gap: var(--s-4);
  }
  .group {
    display: grid;
    gap: var(--s-1);
  }
  /* A category reads like the label printed above a row of keys. */
  .group-label {
    padding-bottom: var(--s-1);
    border-bottom: 1px solid var(--border);
    color: var(--muted);
    font-family: var(--mono);
    font-size: var(--fs-xs);
    font-weight: 500;
    letter-spacing: 0.02em;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
  }
  .item {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--s-3);
    width: 100%;
    min-height: 52px;
    padding: var(--s-2);
    background: none;
    color: var(--text);
    border-radius: var(--r-md);
    text-align: left;
    font-weight: 400;
  }
  .item:hover:not(:disabled) {
    background: var(--neutral-soft);
  }
  .item:hover .glyph,
  .item:focus-visible .glyph {
    border-color: var(--border-strong);
    color: var(--text);
  }
  .item:active:not(:disabled) {
    transform: none;
  }
  .glyph {
    display: grid;
    place-items: center;
    width: 34px;
    height: 34px;
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    background: var(--surface);
    color: var(--text-soft);
  }
  .text {
    display: grid;
    gap: 1px;
    min-width: 0;
  }
  .name {
    font-weight: 550;
  }
  .description {
    color: var(--muted);
    font-size: var(--fs-sm);
    line-height: 1.35;
  }
  .plus {
    color: var(--muted);
    font-family: var(--mono);
    padding-right: var(--s-1);
  }
  .item:hover .plus,
  .item:focus-visible .plus {
    color: var(--focus);
  }
  .empty {
    padding: var(--s-3) 0;
  }
</style>
