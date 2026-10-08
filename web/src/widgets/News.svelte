<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { clock } from '../state.svelte';
  import { LOCALE } from '../format';

  interface Headline {
    title: string;
    published?: string;
    /** The feed's name, when headlines of several feeds are merged. */
    source?: string;
  }
  interface Feed {
    title: string;
    items: Headline[];
  }

  const DEFAULT_ITEMS = 5;

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const feed = $derived(isFeed(entry?.value) ? entry.value : undefined);
  const maxItems = $derived(typeof settings?.maxItems === 'number' ? settings.maxItems : DEFAULT_ITEMS);
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
  const time = new Intl.DateTimeFormat(LOCALE, { hour: '2-digit', minute: '2-digit' });

  function isFeed(v: unknown): v is Feed {
    if (typeof v !== 'object' || v === null) return false;
    const f = v as Record<string, unknown>;
    return (
      typeof f.title === 'string' &&
      Array.isArray(f.items) &&
      f.items.every((i) => typeof i === 'object' && i !== null && typeof (i as Headline).title === 'string')
    );
  }

  function publishedAt(item: Headline): string {
    if (!item.published) return '';
    const date = new Date(item.published);
    return Number.isNaN(date.getTime()) ? '' : time.format(date);
  }
</script>

<div class="news">
  {#if feed && entry}
    <h2 class="source" class:stale>{feed.title}</h2>
    <ol class="headlines">
      {#each feed.items.slice(0, maxItems) as item, i (i)}
        <li>
          <span class="when">{publishedAt(item)}</span>
          <span class="title">
            {#if item.source}<span class="feed">{item.source}</span>{/if}{item.title}
          </span>
        </li>
      {/each}
    </ol>
  {:else}
    <span class="muted">{waitingText(entry, 'news', 'No news feed. Add one under Settings.')}</span>
  {/if}
</div>

<style>
  .news {
    display: flex;
    flex-direction: column;
    width: 100%;
    min-width: 0;
  }
  .source {
    margin: 0 0 20px;
    font-size: 30px;
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .source.stale {
    color: var(--warning);
  }
  .headlines {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 18px;
  }
  li {
    display: flex;
    gap: 24px;
    align-items: baseline;
    min-width: 0;
  }
  .when {
    flex: none;
    width: 3.2em;
    font-size: 30px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .title {
    font-size: 38px;
    line-height: 1.25;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .feed {
    margin-right: 0.5em;
    color: var(--text-muted);
  }
  .muted {
    color: var(--text-muted);
    font-size: 38px;
  }
</style>
