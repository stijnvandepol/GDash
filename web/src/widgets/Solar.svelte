<script lang="ts">
  import type { Entry } from '../protocol';
  import { clock } from '../state.svelte';
  import { locale, t } from '../i18n.svelte';
  import { waitingText } from './waiting';
  import { formatKWh, readSolar, solarView } from './solar';

  let { entry }: { entry: Entry | undefined } = $props();

  const report = $derived(readSolar(entry?.value));
  const view = $derived(report ? solarView(report, clock.now.getTime()) : undefined);
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
</script>

<div class="solar">
  {#if view}
    <div class="today">
      <span class="figure">~{formatKWh(view.todayKWh, locale())}</span>
      <span class="unit" class:stale>kWh</span>
      <span class="label">{t('solarToday')}</span>
      {#if view.peak}<span class="peak">{t('solarPeak', { time: view.peak })}</span>{/if}
    </div>
    {#if view.chart}
      <div class="chart" aria-hidden="true">
        {#each view.chart.bars as bar, i (i)}
          <span class="slot" class:now={bar.now}>
            <span class="bar" class:current={bar.now} class:tomorrow={view.chart.tomorrow} style:height="{bar.height * 100}%"></span>
          </span>
        {/each}
      </div>
      <div class="hours">
        <span>{view.chart.from}</span>
        <span class="which">{t(view.chart.tomorrow ? 'solarTomorrowHours' : 'solarRest')}</span>
        <span>{view.chart.to}</span>
      </div>
    {/if}
    {#if view.tomorrowKWh !== null}
      <span class="tomorrow">{t('solarTomorrow', { amount: formatKWh(view.tomorrowKWh, locale()) })}</span>
    {/if}
  {:else}
    <span class="muted">{waitingText(entry, 'solar')}</span>
  {/if}
</div>

<style>
  .solar {
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 8px;
    width: 100%;
    min-width: 0;
  }
  .today {
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
  .unit,
  .label,
  .peak {
    font-size: 0.4em;
    color: var(--text-muted);
  }
  .unit.stale {
    color: var(--warning);
  }
  .peak {
    margin-left: auto;
    white-space: nowrap;
    color: var(--accent);
  }
  /* Bars are plain boxes in the theme's colours: the accent for the hours
     still to come today, muted for tomorrow's, the current hour in full
     text colour. */
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
    background: var(--accent);
    opacity: 0.85;
  }
  .bar.tomorrow {
    background: var(--text-muted);
    opacity: 0.45;
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
  .hours {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    font-size: 22px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .which {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .tomorrow {
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
  /* A short tile gives its height to the bars; a tall one has room for a
     heading. */
  @container (max-height: 200px) {
    .hours {
      display: none;
    }
  }
</style>
