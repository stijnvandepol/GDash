<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type NewsCatalogue, type NewsFeed, type SourceStatus } from './api';
  import { feedDetail } from './newsCatalogue';
  import NewsCataloguePicker from './NewsCataloguePicker.svelte';
  import SourceList from './SourceList.svelte';

  // The server allows this many; internal/settings holds the same.
  const MAX_NEWS_FEEDS = 10;

  let {
    feeds,
    status,
    onchanged,
    onerror,
  }: {
    feeds: NewsFeed[];
    status: SourceStatus | undefined;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let catalogue = $state<NewsCatalogue | null>(null);
  let loadError = $state('');

  onMount(async () => {
    try {
      catalogue = await api.newsCatalogue();
    } catch (err) {
      loadError = onerror(err);
    }
  });

  const items = $derived(feeds.map((f) => ({ id: f.id, name: f.name, detail: feedDetail(f, catalogue?.sources ?? []) })));

  async function add(name: string, url: string) {
    await api.addNewsFeed(name, url);
    await onchanged();
  }

  async function remove(id: number) {
    await api.deleteNewsFeed(id);
    await onchanged();
  }
</script>

<SourceList
  title="News"
  intro="Choose news sources, or add an RSS or Atom address of your own. A News widget shows the headlines of all of them together, newest first, or of the ones you tick in its settings."
  {items}
  {status}
  max={MAX_NEWS_FEEDS}
  namePlaceholder="My local paper"
  urlPlaceholder="https://example.org/rss.xml"
  formTitle="Your own RSS or Atom address"
  onadd={add}
  onremove={remove}
  {onerror}
>
  {#if loadError}
    <p class="error">{loadError}</p>
  {:else if catalogue}
    <NewsCataloguePicker {catalogue} {feeds} max={MAX_NEWS_FEEDS} onadd={add} onremove={remove} {onerror} />
  {:else}
    <p class="loading muted" role="status">Loading the news sources…</p>
  {/if}
</SourceList>
