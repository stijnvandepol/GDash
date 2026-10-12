<script lang="ts">
  import { fitItems } from './fit';
  import type { Reading } from './hass';

  // The unit is drawn at this fraction of the value's size; see .unit.
  const UNIT_SCALE = 0.8;

  // One reading shows large, more as a list.
  let { rows, stale }: { rows: Reading[]; stale: boolean } = $props();
</script>

{#if rows.length === 1}
  {@const row = rows[0]!}
  <div class="single" class:stale>
    <div
      class="big"
      class:dim={row.shown.dim}
      class:missing={row.missing}
      style:--length={row.shown.value.length + row.shown.unit.length * UNIT_SCALE}
    >
      {row.shown.value}{#if row.shown.unit}<span class="unit">{row.shown.unit}</span>{/if}
    </div>
    <div class="name">{row.name}</div>
  </div>
{:else}
  <ul class="list" class:stale use:fitItems>
    {#each rows as row (row.id)}
      <li>
        <span class="name">{row.name}</span>
        <span class="value" class:dim={row.shown.dim} class:missing={row.missing}>
          {row.shown.value}{#if row.shown.unit}<span class="unit">{row.shown.unit}</span>{/if}
        </span>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .single {
    display: flex;
    flex-direction: column;
    justify-content: center;
    width: 100%;
    min-width: 0;
  }
  .big {
    /* As large as the tile allows: a digit is about 0.55em wide, so the
       value fills most of the width whether it is "21" or "Geen beweging". */
    font-size: min(180px, calc(140cqw / max(var(--length), 3)), 44cqh);
    font-weight: var(--weight-figure);
    line-height: 1;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .single .name {
    margin-top: min(16px, 4cqh);
    font-size: min(40px, 9cqw, 18cqh);
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 18px;
    width: 100%;
    min-width: 0;
  }
  /* In a narrow tile the value drops below the name instead of squeezing it. */
  li {
    display: flex;
    flex-wrap: wrap;
    gap: 0 24px;
    align-items: baseline;
    justify-content: space-between;
    min-width: 0;
  }
  .name {
    font-size: 32px;
    font-size: min(32px, 10cqw);
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  li .name {
    flex: 1 1 4em;
  }
  .value {
    flex: none;
    margin-left: auto;
    font-size: 44px;
    font-size: min(44px, 14cqw);
    font-weight: var(--weight-figure);
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }
  /* Not much smaller than the value: a capital "W" at 0.6em is as tall as a
     lowercase letter next to the digits and reads as "w" from across a room. */
  .unit {
    margin-left: 0.15em;
    font-size: 0.8em;
    font-weight: var(--weight-text, 400);
    color: var(--text-muted);
  }
  .dim {
    color: var(--text-muted);
  }
  .missing {
    font-size: 28px;
  }
  .stale .value,
  .stale .big {
    color: var(--warning);
  }
</style>
