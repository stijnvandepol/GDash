<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { fitItems } from './fit';
  import { clock, display } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import { departureLists, upcoming, whenText } from './departures';

  const DEFAULT_ITEMS = 5;

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const stop = $derived(typeof settings?.stop === 'string' ? settings.stop : '');
  const title = $derived(typeof settings?.stopName === 'string' && settings.stopName !== '' ? settings.stopName : stop);
  const maxItems = $derived(typeof settings?.maxItems === 'number' ? settings.maxItems : DEFAULT_ITEMS);
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
  // The screen's clock, corrected when it is far off: a TV that runs five
  // minutes fast would show every bus five minutes too soon.
  const now = $derived(new Date(clock.now.getTime() + display.serverOffsetMs));
  const list = $derived(departureLists(entry?.value)?.[stop]);
  const rows = $derived(list ? upcoming(list, now, maxItems) : undefined);
</script>

<div class="board" use:fitItems>
  {#if rows}
    <h2 class="stop" class:stale>{title}</h2>
    {#if rows.length > 0}
      <ol class="departures">
        {#each rows as row, i (i)}
          <li class:cancelled={row.cancelled} class:late={row.delay > 0}>
            <span class="line">{row.line}</span>
            <span class="destination">{row.destination}</span>
            <span class="when">
              {row.cancelled ? row.clock : whenText(row.minutes)}
              {#if row.delay > 0 && !row.cancelled}<span class="delay">+{row.delay}</span>{/if}
            </span>
          </li>
        {/each}
      </ol>
    {:else}
      <p class="muted">{t('departuresNone')}</p>
    {/if}
  {:else}
    <span class="muted">{waitingText(entry, 'departures')}</span>
  {/if}
</div>

<style>
  .board {
    display: flex;
    flex-direction: column;
    width: 100%;
    min-width: 0;
  }
  .stop {
    margin: 0 0 clamp(8px, 4cqh, 20px);
    font-family: var(--font-display);
    font-size: clamp(24px, 6.5cqw, 36px);
    font-weight: 500;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .stop.stale {
    color: var(--warning);
  }
  .departures {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: clamp(8px, 3cqh, 16px);
  }
  li {
    display: grid;
    grid-template-columns: 2.7em minmax(0, 1fr) auto;
    gap: 0 min(20px, 4cqw);
    align-items: baseline;
    font-size: clamp(26px, 6.2cqw, 40px);
    line-height: 1.25;
    color: var(--text-primary);
  }
  /* The line number is outlined in the accent colour, which every theme
     keeps readable on its tiles; a filled badge would need a second
     colour that the themes do not promise. */
  .line {
    min-width: 1.9em;
    max-width: 100%;
    justify-self: start;
    padding: 0 0.3em;
    box-sizing: border-box;
    border: 0.08em solid currentColor;
    border-radius: 0.3em;
    text-align: center;
    font-size: 0.8em;
    font-weight: var(--weight-figure);
    font-variant-numeric: tabular-nums;
    color: var(--accent);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .destination {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .when {
    font-weight: var(--weight-figure);
    font-variant-numeric: tabular-nums;
    text-align: right;
    white-space: nowrap;
  }
  .late .when {
    color: var(--warning);
  }
  .delay {
    margin-left: 0.3em;
    font-size: 0.75em;
  }
  .cancelled,
  .cancelled .line {
    color: var(--text-muted);
  }
  .cancelled .destination,
  .cancelled .when {
    text-decoration: line-through;
  }
  .muted {
    margin: 0;
    color: var(--text-muted);
    font-size: clamp(24px, 6.5cqw, 38px);
  }
</style>
