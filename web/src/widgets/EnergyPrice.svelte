<script lang="ts">
  import type { Entry } from '../protocol';
  import { clock } from '../state.svelte';
  import { locale, t } from '../i18n.svelte';
  import { waitingText } from './waiting';
  import { LEVEL_TEXT, formatPrice, priceView, readPrices } from './energyprice';

  let { entry }: { entry: Entry | undefined } = $props();

  const view = $derived(priceView(readPrices(entry?.value), clock.now.getTime()));
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
</script>

<div class="price">
  {#if view}
    <div class="now">
      <span class="figure">{formatPrice(view.price, locale())}</span>
      <span class="unit" class:stale>{t('energyPriceUnit')}</span>
      <span class="level {view.level}">{t(LEVEL_TEXT[view.level])}</span>
    </div>
    <div class="chart" aria-hidden="true">
      {#each view.bars as bar, i (i)}
        <span class="slot" class:now={bar.now}>
          <span class="bar" class:cheap={bar.cheap} class:current={bar.now} style:height="{bar.height * 100}%"></span>
        </span>
      {/each}
    </div>
    <div class="hours" aria-hidden="true">
      <span>00</span><span>06</span><span>12</span><span>18</span>
    </div>
    {#if view.cheapest}
      <span class="cheapest">
        {t(view.cheapest.tomorrow ? 'energyPriceCheapestTomorrow' : 'energyPriceCheapest', { from: view.cheapest.from, to: view.cheapest.to })}
      </span>
    {/if}
  {:else}
    <span class="muted">{waitingText(entry, 'energyprice')}</span>
  {/if}
</div>

<style>
  .price {
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 8px;
    width: 100%;
    min-width: 0;
  }
  .now {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    column-gap: 0.25em;
    font-size: 90px;
    font-size: min(90px, 17cqw, 27cqh);
    line-height: 1.05;
  }
  .figure {
    font-weight: var(--weight-figure);
    font-variant-numeric: tabular-nums;
    color: var(--text-primary);
  }
  .unit {
    font-size: 0.4em;
    color: var(--text-muted);
  }
  .unit.stale {
    color: var(--warning);
  }
  .level {
    margin-left: auto;
    font-size: 0.4em;
    white-space: nowrap;
    color: var(--text-muted);
  }
  .level.low {
    color: var(--accent);
  }
  .level.high {
    color: var(--warning);
  }
  /* Bars are plain boxes in the theme's colours: muted, the cheapest
     stretch in the accent, the current hour in full text colour. */
  .chart {
    display: flex;
    align-items: stretch;
    gap: 3px;
    flex: 1 1 0;
    min-height: 24px;
  }
  .slot {
    position: relative;
    display: flex;
    flex: 1 1 0;
    align-items: flex-end;
    min-width: 0;
    /* Room for the notch above the tallest bar. */
    padding-top: 10px;
  }
  .bar {
    width: 100%;
    border-radius: 3px 3px 0 0;
    background: var(--text-muted);
    opacity: 0.45;
  }
  .bar.cheap {
    background: var(--accent);
    opacity: 0.85;
  }
  .bar.current {
    background: var(--text-primary);
    opacity: 1;
  }
  /* A notch above the bar of the current hour, so it is found without
     relying on its colour alone. */
  .slot.now::before {
    content: '';
    position: absolute;
    top: 0;
    left: 50%;
    width: 10px;
    height: 7px;
    transform: translateX(-50%);
    background: var(--text-primary);
    clip-path: polygon(0 0, 100% 0, 50% 100%);
  }
  /* Four labels at a quarter of the day each, under the bars they name. */
  .hours {
    display: none;
    grid-template-columns: repeat(4, 1fr);
    font-size: 22px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .cheapest {
    flex-shrink: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    line-height: 1.2;
    font-size: 32px;
    font-size: min(32px, 6.4cqw, 12cqh);
    color: var(--text-primary);
  }
  .muted {
    color: var(--text-muted);
    font-size: 38px;
  }
  /* A tall tile has room for the hours under the bars. */
  @container (min-height: 300px) {
    .hours {
      display: grid;
    }
  }
</style>
