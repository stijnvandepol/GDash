<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { fitItems } from './fit';
  import { locale, t } from '../i18n.svelte';
  import { SOURCES_REMOVED, newsView, type Headline } from './news';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const view = $derived(newsView(entry?.value, settings));
  const feed = $derived(view === SOURCES_REMOVED ? null : view);
  const time = $derived(new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit' }));

  function publishedAt(item: Headline): string {
    if (!item.published) return '';
    const date = new Date(item.published);
    return Number.isNaN(date.getTime()) ? '' : time.format(date);
  }
</script>

<div class="news" use:fitItems>
  {#if feed && entry}
    <ol class="headlines">
      {#each feed.items as item, i (i)}
        <li>
          <span class="when">{publishedAt(item)}</span>
          <span class="title">
            {#if item.source}<span class="feed">{item.source}</span>{/if}<span class="text">{item.title}</span>
          </span>
        </li>
      {/each}
    </ol>
  {:else if view === SOURCES_REMOVED}
    <span class="muted">{t('newsSourcesRemoved')}</span>
  {:else}
    <span class="muted">{waitingText(entry, 'news')}</span>
  {/if}
</div>

<style>
  .news {
    display: flex;
    flex-direction: column;
    width: 100%;
    min-width: 0;
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
    display: flex;
    gap: 0.5em;
    align-items: baseline;
    font-size: 38px;
    line-height: 1.25;
    color: var(--text-primary);
    min-width: 0;
  }
  /* The source's name stays on one line, beside or above the headline. */
  .feed {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  /* A headline gets two lines before it is cut short, so most read whole;
     fitItems hides the ones that no longer fit the tile at all. */
  .text {
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    overflow: hidden;
    overflow-wrap: break-word;
  }
  /* A long source name never takes the headline's place. */
  .feed {
    flex: none;
    max-width: 40%;
    color: var(--text-muted);
  }
  .text {
    flex: 1;
    min-width: 0;
  }
  .muted {
    color: var(--text-muted);
    font-size: 38px;
  }
  /* A narrow tile keeps the headline and drops the time; the source goes
     above the headline, smaller, so both stay readable. */
  @container (max-width: 440px) {
    .when {
      display: none;
    }
    .title {
      flex-direction: column;
      gap: 0;
    }
    .feed {
      max-width: 100%;
      font-size: 28px;
    }
    .text {
      align-self: stretch;
    }
  }
</style>
