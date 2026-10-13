<script lang="ts">
  import type { Entry } from '../protocol';
  import { display } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import { fitItems, shrinkToFit } from './fit';
  import { followedDevices, formatTime, playingItems, positionAt, progressAt, shownItems } from './nowplaying';

  // A title shrinks to this before it ends in an ellipsis: still readable
  // from the sofa.
  const MIN_TITLE_PX = 34;
  const TICK_MS = 1000;

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const items = $derived(entry?.value === undefined ? undefined : playingItems(entry.value));
  const shown = $derived(items ? shownItems(items, followedDevices(settings)) : []);
  const featured = $derived(shown[0]);
  const others = $derived(shown.slice(1));

  // The position moves on every second while something plays; the server
  // only says where it was and when. Its clock, not the screen's.
  let nowMs = $state(Date.now());
  $effect(() => {
    if (featured?.state !== 'playing') return;
    nowMs = Date.now();
    const timer = setInterval(() => (nowMs = Date.now()), TICK_MS);
    return () => clearInterval(timer);
  });
  const serverNow = $derived(nowMs + display.serverOffsetMs);
  const progress = $derived(featured ? progressAt(featured, serverNow) : undefined);

  // A cover that does not load leaves the note in its place.
  let failedImage = $state('');
  const cover = $derived(featured && featured.image !== failedImage ? featured.image : '');

  const sourceText = $derived(
    featured ? (featured.app ? t('nowPlayingOn', { app: featured.app, device: featured.name }) : featured.name) : '',
  );
</script>

{#snippet note()}
  <svg class="note" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M9 18V5l11-2v13" />
    <circle cx="6" cy="18" r="3" />
    <circle cx="17" cy="16" r="3" />
  </svg>
{/snippet}

{#snippet mark(state: string)}
  <svg class="mark" viewBox="0 0 12 12" aria-hidden="true">
    {#if state === 'paused'}
      <rect x="2" y="1.5" width="2.8" height="9" rx="0.8" fill="currentColor" />
      <rect x="7.2" y="1.5" width="2.8" height="9" rx="0.8" fill="currentColor" />
    {:else}
      <path d="M3 1.5v9l7.5-4.5z" fill="currentColor" />
    {/if}
  </svg>
{/snippet}

<div class="now">
  {#if items === undefined}
    <p class="muted">{t('nowPlayingLoading')}</p>
  {:else if featured}
    <article class="card" class:paused={featured.state === 'paused'} class:alone={others.length === 0}>
      <div class="art">
        {#if cover}
          {#key cover}
            <img src={cover} alt="" onerror={() => (failedImage = cover)} />
          {/key}
        {:else}
          {@render note()}
        {/if}
      </div>
      <div class="info">
        <p class="source">
          {@render mark(featured.state)}
          <span class="source-text">{featured.state === 'paused' ? `${t('nowPlayingPaused')} · ${sourceText}` : sourceText}</span>
        </p>
        <p class="title" use:shrinkToFit={MIN_TITLE_PX}>{featured.title || featured.artist}</p>
        {#if featured.title && (featured.artist || featured.album)}
          <p class="artist">{[featured.artist, featured.album].filter(Boolean).join(' · ')}</p>
        {/if}
        {#if progress !== undefined}
          <div class="progress" role="presentation"><div class="bar" style:transform="scaleX({progress})"></div></div>
          <p class="times">
            <span>{formatTime(positionAt(featured, serverNow))}</span><span>{formatTime(featured.duration)}</span>
          </p>
        {:else}
          <p class="times"><span>{t('nowPlayingLive')}</span></p>
        {/if}
      </div>
    </article>
    {#if others.length > 0}
      <ul class="others" use:fitItems>
        {#each others as other (other.id)}
          <li class:paused={other.state === 'paused'}>
            {@render mark(other.state)}
            <span class="who">{other.name}</span>
            <span class="what">{[other.title, other.artist].filter(Boolean).join(' · ')}</span>
          </li>
        {/each}
      </ul>
    {/if}
  {:else}
    <div class="nothing">
      {@render note()}
      <p>{t('nowPlayingNothing')}</p>
    </div>
  {/if}
</div>

<style>
  .now {
    display: flex;
    flex-direction: column;
    gap: clamp(12px, 5cqh, 28px);
    width: 100%;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }
  .card {
    display: flex;
    gap: clamp(16px, 4cqw, 40px);
    align-items: center;
    flex: 0 1 62%;
    min-height: 0;
  }
  .card.alone {
    flex-basis: 100%;
  }
  /* Square, as high as the card but never more than 38% of the tile wide. */
  .art {
    flex: none;
    height: min(100%, 38cqw);
    aspect-ratio: 1;
    border-radius: calc(var(--radius-widget) / 2);
    position: relative;
    display: grid;
    place-items: center;
    overflow: hidden;
    color: var(--text-muted);
  }
  /* A faint tile behind the cover, in the theme's own text colour; layers
     with opacity instead of color-mix, which older receivers lack. */
  .art::before,
  .progress::before {
    content: '';
    position: absolute;
    inset: 0;
    background: var(--text-primary);
    opacity: 0.08;
  }
  .progress::before {
    background: var(--text-muted);
    opacity: 0.3;
  }
  .art img,
  .art .note,
  .bar {
    position: relative;
  }
  .art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .art .note {
    width: 42%;
    height: 42%;
  }
  .paused .art {
    opacity: 0.6;
  }
  .info {
    flex: 1 1 auto;
    min-width: 0;
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: clamp(4px, 2cqh, 12px);
  }
  p {
    margin: 0;
  }
  .source {
    display: flex;
    align-items: center;
    gap: 0.4em;
    font-size: clamp(18px, 4.2cqw, 28px);
    color: var(--text-muted);
    min-width: 0;
  }
  .source-text,
  .artist {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .mark {
    flex: none;
    width: 0.8em;
    height: 0.8em;
    color: var(--accent);
  }
  .paused .mark {
    color: var(--text-muted);
  }
  /* One line in a small tile, ending in an ellipsis. A tall tile has room
     for two lines: a longer title first shrinks (shrinkToFit), then ends
     in an ellipsis. */
  .title {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    font-family: var(--font-display);
    font-size: clamp(30px, min(8cqw, 16cqh), 72px);
    font-weight: 600;
    line-height: 1.15;
    color: var(--text-primary);
  }
  @container (min-height: 380px) {
    .title {
      display: -webkit-box;
      -webkit-box-orient: vertical;
      -webkit-line-clamp: 2;
      line-clamp: 2;
      white-space: normal;
      overflow-wrap: anywhere;
    }
  }
  .artist {
    font-size: clamp(18px, min(4.2cqw, 7.5cqh), 32px);
    color: var(--text-muted);
  }
  .progress {
    margin-top: clamp(4px, 2cqh, 14px);
    height: 4px;
    border-radius: 2px;
    position: relative;
    overflow: hidden;
  }
  .bar {
    height: 100%;
    background: var(--accent);
    transform-origin: left center;
  }
  .paused .bar {
    background: var(--text-muted);
  }
  .times {
    display: flex;
    justify-content: space-between;
    font-size: clamp(16px, 3.4cqw, 24px);
    font-variant-numeric: tabular-nums;
    color: var(--text-muted);
  }
  .others {
    list-style: none;
    margin: 0;
    padding: 0;
    flex: 1 1 0;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    gap: clamp(6px, 2.5cqh, 14px);
  }
  li {
    display: grid;
    grid-template-columns: auto minmax(0, max-content) minmax(0, 1fr);
    gap: 0 0.6em;
    align-items: center;
    font-size: clamp(20px, min(4.6cqw, 8cqh), 34px);
    line-height: 1.25;
  }
  .who {
    color: var(--text-primary);
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 14em;
  }
  .what {
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .nothing {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: clamp(8px, 4cqh, 20px);
    color: var(--text-muted);
    font-size: clamp(22px, min(6cqw, 11cqh), 40px);
  }
  .nothing .note {
    width: clamp(36px, 18cqh, 88px);
    height: clamp(36px, 18cqh, 88px);
    opacity: 0.55;
  }
  .muted {
    color: var(--text-muted);
    font-size: clamp(24px, 6.5cqw, 38px);
  }
</style>
