<script lang="ts">
  import type { Entry } from '../protocol';
  import { fitItems } from './fit';
  import { clock } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import {
    DEFAULT_ITEMS,
    camerasText,
    emptyText,
    showOf,
    showsIncidents,
    totalsText,
    trafficKey,
    trafficResult,
    trafficRows,
    trafficWaitingText,
  } from './traffic';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const show = $derived(showOf(settings));
  const maxItems = $derived(typeof settings?.maxItems === 'number' ? settings.maxItems : DEFAULT_ITEMS);
  const result = $derived(trafficResult(entry?.value, trafficKey(settings)));
  const rows = $derived(result ? trafficRows(result, show, maxItems, clock.now) : []);
  // ANWB data is wanted but traffic info is turned off: say so instead of
  // a calm "no jams" that is not true.
  const anwbOff = $derived(result !== undefined && !result.incidents && showsIncidents(show));
  const showCameras = $derived(show.includes('cameras'));
  const sources = $derived(
    [result?.incidents && showsIncidents(show) ? t('trafficSourceAnwb') : '', showCameras ? t('trafficSourceOsm') : '']
      .filter(Boolean)
      .join(' · '),
  );
</script>

<div class="traffic">
  {#if result}
    {#if show.includes('totals') && result.totals}
      <p class="totals">{totalsText(result.totals)}</p>
    {/if}
    {#if anwbOff}
      <p class="muted">{t('trafficAnwbOff')}</p>
    {/if}
    <div class="body" use:fitItems>
      {#if rows.length > 0}
        <ol class="rows">
          {#each rows as row, i (i)}
            <li class:severe={row.severe} class={row.kind}>
              <span class="road">{row.road || '—'}</span>
              <span class="place">
                <span class="where">{row.place}</span>
                {#if row.note}<span class="note">{row.note}</span>{/if}
              </span>
              {#if row.detail}<span class="detail">{row.detail}</span>{/if}
            </li>
          {/each}
        </ol>
      {:else if !anwbOff}
        <p class="calm">{emptyText(settings, show)}</p>
      {/if}
    </div>
    {#if showCameras}
      <p class="cameras">
        {result.cameras === null ? t('trafficCamerasLoading') : camerasText(settings, result.cameraCount)}
      </p>
    {/if}
    {#if sources}<p class="source">{sources}</p>{/if}
  {:else}
    <span class="muted">{trafficWaitingText(entry, settings)}</span>
  {/if}
</div>

<style>
  .traffic {
    display: flex;
    flex-direction: column;
    width: 100%;
    height: 100%;
    min-width: 0;
  }
  .totals,
  .cameras,
  .source {
    margin: 0;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .totals {
    margin-bottom: clamp(6px, 2.5cqh, 14px);
    font-size: clamp(20px, 4.6cqw, 28px);
  }
  .body {
    flex: 1 1 auto;
    min-height: 0;
    overflow: hidden;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: clamp(8px, 2.6cqh, 16px);
  }
  li {
    display: grid;
    grid-template-columns: 2.9em minmax(0, 1fr) auto;
    gap: 0 min(18px, 3.5cqw);
    align-items: baseline;
    font-size: clamp(24px, 5.6cqw, 38px);
    line-height: 1.2;
    color: var(--text-primary);
  }
  /* The road number is outlined like a line number on the departure
     board: the accent colour is readable on every theme's tiles. */
  .road {
    justify-self: start;
    min-width: 2.2em;
    padding: 0 0.25em;
    box-sizing: border-box;
    border: 0.08em solid currentColor;
    border-radius: 0.3em;
    text-align: center;
    font-size: 0.8em;
    font-weight: var(--weight-figure);
    font-variant-numeric: tabular-nums;
    color: var(--accent);
    white-space: nowrap;
  }
  .place {
    display: grid;
    min-width: 0;
  }
  .where,
  .note {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .note {
    font-size: 0.56em;
    color: var(--text-muted);
  }
  .detail {
    font-size: 0.8em;
    font-weight: var(--weight-figure);
    font-variant-numeric: tabular-nums;
    text-align: right;
    white-space: nowrap;
  }
  /* A closure or a long delay is set apart by colour and by its words
     ("Closed", "+12 min"), so it is also found without the colour. */
  .severe .detail {
    color: var(--warning);
  }
  .speedcheck .detail,
  .camera .detail {
    color: var(--text-muted);
  }
  .calm,
  .muted {
    margin: 0;
    color: var(--text-muted);
    font-size: clamp(24px, 6cqw, 36px);
  }
  .cameras {
    margin-top: clamp(6px, 2.5cqh, 14px);
    font-size: clamp(18px, 4.2cqw, 26px);
  }
  .source {
    margin-top: 4px;
    font-size: clamp(14px, 3cqw, 18px);
    opacity: 0.85;
  }
  @container (max-height: 300px) {
    .note,
    .cameras {
      display: none;
    }
  }
</style>
