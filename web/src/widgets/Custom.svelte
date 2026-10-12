<script lang="ts">
  import type { Entry } from '../protocol';
  import { fitItems } from './fit';
  import { formatCell, shownFor, waitingFor, type Tone } from './custom';

  // The unit is drawn at this fraction of the value's size; see .unit.
  const UNIT_SCALE = 0.6;

  // Everything shown comes from an API the household chose, so it is only
  // ever put on the screen as text, never as markup.
  let { entry, key }: { entry: Entry | undefined; key: string } = $props();

  const shown = $derived(shownFor(entry, key));
  const value = $derived(shown ? formatCell(shown.value, shown.decimals) : '');
  const unit = $derived(shown?.unit ?? '');

  function toneClass(tone: Tone | undefined): string {
    return tone ? `tone-${tone}` : '';
  }
</script>

<div class="custom" class:stale={shown?.stale || shown?.failing}>
  {#if shown?.title}<h2 class="title">{shown.title}</h2>{/if}
  {#if shown && shown.state === 'ok'}
    {#if shown.message}
      <p class="message {toneClass(shown.tone)}">
        {#if shown.tone === 'bad'}<span class="mark">!</span>{/if}{shown.message}
      </p>
    {/if}
    {#if !shown.hidden}
      {#if shown.mode === 'value'}
        <div class="single">
          <div class="big {toneClass(shown.tone)}" style:--length={value.length + unit.length * UNIT_SCALE}>
            {value}{#if unit}<span class="unit">{unit}</span>{/if}
          </div>
          {#if shown.label}<div class="label">{shown.label}</div>{/if}
        </div>
      {:else if shown.mode === 'list'}
        <ul class="rows" use:fitItems>
          {#each shown.rows ?? [] as row, i (i)}
            <li class={toneClass(row.tone ?? shown.tone)}>
              {#if row.tone === 'bad' || (!row.tone && shown.tone === 'bad')}<span class="mark">!</span>{/if}
              {#each row.cells as cell, j (j)}
                <span class={j === 0 ? 'name' : 'cell'}>{formatCell(cell)}</span>
              {/each}
            </li>
          {:else}
            <li class="none">–</li>
          {/each}
        </ul>
      {:else}
        <ul class="rows" use:fitItems>
          {#each shown.pairs ?? [] as pair, i (i)}
            <li class={toneClass(shown.tone)}>
              <span class="name">{pair.label}</span>
              <span class="cell">{formatCell(pair.value)}</span>
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  {:else}
    <span class="muted">{waitingFor(shown)}</span>
  {/if}
</div>

<style>
  .custom {
    display: flex;
    flex-direction: column;
    justify-content: center;
    width: 100%;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }
  .title {
    margin: 0 0 clamp(6px, 3cqh, 16px);
    font-family: var(--font-display);
    font-size: clamp(22px, 6cqw, 34px);
    font-weight: 500;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: none;
  }
  .stale .title {
    color: var(--warning);
  }
  .message {
    margin: 0 0 clamp(6px, 3cqh, 16px);
    font-size: clamp(22px, 6cqw, 36px);
    line-height: 1.2;
    color: var(--text-primary);
    flex: none;
    overflow-wrap: anywhere;
  }
  .single {
    display: flex;
    flex-direction: column;
    justify-content: center;
    min-width: 0;
  }
  .big {
    /* As large as the tile allows, as in the Home Assistant widget. */
    font-size: min(180px, calc(140cqw / max(var(--length), 3)), 44cqh);
    font-weight: var(--weight-figure);
    line-height: 1;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .unit {
    margin-left: 0.15em;
    font-size: 0.6em;
    font-weight: var(--weight-text, 400);
    color: var(--text-muted);
  }
  .label {
    margin-top: min(16px, 4cqh);
    font-size: min(40px, 9cqw, 18cqh);
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    align-content: start;
    gap: clamp(6px, 2.5cqh, 16px);
    flex: 1 1 0;
    min-height: 0;
    overflow: hidden;
  }
  /* In a narrow tile the values drop below the name instead of squeezing it. */
  li {
    display: flex;
    flex-wrap: wrap;
    gap: 0 min(24px, 4cqw);
    align-items: baseline;
    font-size: clamp(24px, 6cqw, 40px);
    line-height: 1.25;
    color: var(--text-primary);
    min-width: 0;
  }
  .name {
    flex: 1 1 4em;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .cell {
    flex: none;
    margin-left: auto;
    font-weight: var(--weight-figure);
    font-variant-numeric: tabular-nums;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .none {
    color: var(--text-muted);
  }
  /* Tones use colours every theme defines and keeps readable on its tiles:
     the accent for good, the warning colour for a warning, and the warning
     colour with a mark and heavier type for bad, so the two differ without
     colour too. */
  .tone-ok,
  .tone-ok .cell {
    color: var(--accent);
  }
  .tone-warn,
  .tone-warn .cell,
  .tone-bad,
  .tone-bad .cell {
    color: var(--warning);
  }
  .tone-bad {
    font-weight: 700;
  }
  .mark {
    margin-right: 0.3em;
    font-weight: 800;
  }
  .muted {
    color: var(--text-muted);
    font-size: clamp(24px, 6.5cqw, 38px);
  }
</style>
