<script lang="ts">
  import type { Entry } from '../protocol';
  import { clock } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import { readPlace } from './fuel';
  import { rainKey, rainSentence, rainView, rainWaitingText, readForecast } from './rain';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const place = $derived(readPlace(settings?.place));
  const forecast = $derived(readForecast(entry?.value, rainKey(settings)));
  const view = $derived(forecast ? rainView(forecast, clock.now.getTime()) : undefined);
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
</script>

<div class="rain">
  {#if view}
    <h2 class="title" class:stale>
      <span>{t('rainTitle')}</span>
      {#if place}<span class="place">{place.name}</span>{/if}
    </h2>
    <p class="sentence" class:wet={view.summary.kind !== 'dry'}>{rainSentence(view.summary)}</p>
    <div class="chart" aria-hidden="true">
      {#each view.bars as bar, i (i)}
        <span class="slot">
          <span class="bar {bar.intensity ?? 'dry'}" style:height={bar.intensity ? `${bar.height * 100}%` : undefined}></span>
        </span>
      {/each}
    </div>
    <div class="times" aria-hidden="true"><span>{t('rainNow')}</span><span>{view.middle}</span><span>{view.last}</span></div>
    <!-- Buienradar's terms ask for this credit wherever its data is shown. -->
    <span class="source" class:stale>{view.source}</span>
  {:else}
    <span class="muted">{forecast ? t('rainLoading') : rainWaitingText(entry, settings)}</span>
  {/if}
</div>

<style>
  .rain {
    display: flex;
    flex-direction: column;
    gap: clamp(4px, 3cqh, 14px);
    width: 100%;
    min-width: 0;
  }
  .title {
    display: none;
    gap: 0.4em;
    margin: 0;
    font-family: var(--font-display);
    font-size: clamp(24px, 6cqw, 34px);
    font-weight: 500;
    color: var(--text-muted);
    white-space: nowrap;
  }
  .title.stale {
    color: var(--warning);
  }
  .place {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-primary);
  }
  .title.stale .place {
    color: inherit;
  }
  .place::before {
    content: '· ';
  }
  .sentence {
    flex: none;
    margin: 0;
    /* One line on most tiles, so the graph keeps its room. */
    font-size: clamp(20px, min(7cqw, 20cqh), 46px);
    line-height: 1.15;
    color: var(--text-muted);
    overflow-wrap: anywhere;
  }
  .sentence.wet {
    color: var(--text-primary);
    font-weight: var(--weight-figure);
  }
  /* One bar per step; dry steps are a thin line on the ground, so the
     graph shows where the rain is without counting on its colour. */
  .chart {
    display: flex;
    align-items: stretch;
    gap: 2px;
    flex: 1 1 0;
    min-height: 16px;
    border-bottom: 2px solid color-mix(in srgb, var(--text-muted) 45%, transparent);
  }
  .slot {
    display: flex;
    flex: 1 1 0;
    align-items: flex-end;
    min-width: 0;
  }
  .bar {
    width: 100%;
    border-radius: 3px 3px 0 0;
    background: var(--accent);
  }
  .bar.light {
    opacity: 0.6;
  }
  .bar.moderate {
    opacity: 0.85;
  }
  .bar.dry {
    height: 0;
  }
  .times {
    display: none;
    grid-template-columns: repeat(3, 1fr);
    flex: none;
    margin-top: calc(-1 * clamp(2px, 2cqh, 10px));
    font-size: clamp(16px, 3.6cqw, 24px);
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .times > :nth-child(2) {
    text-align: center;
  }
  .times > :nth-child(3) {
    text-align: right;
  }
  .source {
    flex: none;
    align-self: flex-end;
    margin-top: -2px;
    font-size: clamp(13px, 2.8cqw, 18px);
    line-height: 1;
    color: var(--text-muted);
  }
  .source.stale {
    color: var(--warning);
  }
  .muted {
    color: var(--text-muted);
    font-size: clamp(22px, 6cqw, 38px);
  }
  /* A taller tile has room for a heading and the times under the bars. */
  @container (min-height: 180px) {
    .times {
      display: grid;
    }
  }
  @container (min-height: 280px) {
    .title {
      display: flex;
    }
  }
</style>
