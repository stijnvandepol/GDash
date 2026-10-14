<script lang="ts">
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import type { NewsCatalogue, NewsCatalogueEntry, NewsFeed } from './api';
  import { catalogueGroups, chosenFeed } from './newsCatalogue';
  import Icon from './Icon.svelte';

  // The built-in news sources, to tick and untick; a ticked one is one of
  // the feeds under Integrations, found by its address.

  let {
    catalogue,
    feeds,
    max,
    onadd,
    onremove,
    onerror,
  }: {
    catalogue: NewsCatalogue;
    feeds: NewsFeed[];
    max: number;
    onadd: (name: string, url: string) => Promise<void>;
    onremove: (id: number) => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let query = $state('');
  let busy = $state('');
  let error = $state('');

  const groups = $derived(catalogueGroups(catalogue.sources, catalogue.categories, query));
  const full = $derived(feeds.length >= max);

  async function toggle(entry: NewsCatalogueEntry, box: HTMLInputElement) {
    const chosen = chosenFeed(feeds, entry);
    error = '';
    if (chosen && !(await ask({ title: `Remove "${chosen.name}"?`, message: 'It is no longer fetched or shown on the TVs.', action: 'Remove' }))) {
      box.checked = true;
      return;
    }
    busy = entry.id;
    try {
      if (chosen) {
        await onremove(chosen.id);
        toast.success(`Removed ${chosen.name}.`);
      } else {
        await onadd(entry.name, entry.url);
        toast.success(`Added ${entry.name}.`);
      }
    } catch (err) {
      // The box shows what is stored, not what was clicked.
      box.checked = chosen !== undefined;
      error = onerror(err);
    } finally {
      busy = '';
    }
  }
</script>

<div class="catalogue">
  <h3>Add a news source</h3>
  <div class="search">
    <Icon name="search" size={16} />
    <input
      type="search"
      bind:value={query}
      placeholder="Search, such as NOS or sport"
      aria-label="Search news sources"
      autocomplete="off"
      spellcheck="false"
    />
  </div>
  <p class="small muted" role="status">
    {feeds.length} of {max} chosen{full ? '. Remove one to choose another.' : ''}
  </p>
  <div class="groups">
    {#each groups as g (g.country)}
      <section aria-label={g.label}>
        <h4>{g.label}</h4>
        {#each g.categories as c (c.category)}
          <fieldset>
            <legend>{c.label}</legend>
            <ul>
              {#each c.sources as entry (entry.id)}
                {@const checked = chosenFeed(feeds, entry) !== undefined}
                <li>
                  <label>
                    <input
                      type="checkbox"
                      {checked}
                      disabled={busy !== '' || (!checked && full)}
                      onchange={(e) => toggle(entry, e.currentTarget)}
                    />
                    {entry.name}
                  </label>
                </li>
              {/each}
            </ul>
          </fieldset>
        {/each}
      </section>
    {:else}
      <p class="muted">Nothing matches “{query.trim()}”. Add its RSS or Atom address below.</p>
    {/each}
  </div>
  {#if error}<p class="error">{error}</p>{/if}
</div>

<style>
  .catalogue {
    display: grid;
    gap: var(--s-3);
    min-width: 0;
  }
  .search {
    position: relative;
    display: flex;
    align-items: center;
    max-width: 40rem;
    color: var(--muted);
  }
  .search :global(.icon) {
    position: absolute;
    left: 12px;
    pointer-events: none;
  }
  .search input {
    width: 100%;
    padding-left: 36px;
  }
  .search input::-webkit-search-cancel-button {
    appearance: none;
  }
  .groups {
    display: grid;
    gap: var(--s-4);
    max-height: 26rem;
    overflow-y: auto;
    padding: var(--s-3) var(--s-4);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    background: var(--surface);
  }
  section {
    display: grid;
    gap: var(--s-2);
  }
  h4 {
    margin: 0;
    font-size: var(--fs-sm);
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--text-soft);
  }
  fieldset {
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  legend {
    padding: 0;
    margin-bottom: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--muted);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr));
    gap: var(--s-1) var(--s-4);
  }
  li label {
    display: flex;
    gap: var(--s-2);
    align-items: center;
    min-height: 32px;
    font-weight: 400;
    overflow-wrap: anywhere;
  }
</style>
