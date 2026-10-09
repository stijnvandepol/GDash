<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { fitItems } from './fit';
  import { clock } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import { entityStates, show, type Shown } from './hass';

  // The unit is drawn at 0.6em; see .unit.
  const UNIT_SCALE = 0.6;

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const states = $derived(entityStates(entry?.value));
  const ids = $derived(
    Array.isArray(settings?.entities) ? settings.entities.filter((id): id is string => typeof id === 'string') : [],
  );
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
  const rows = $derived(
    ids.map((id) => {
      const entity = states?.[id];
      const shown: Shown = entity ? show(id, entity) : { value: t('haMissing'), unit: '', dim: true };
      return { id, name: entity?.name ?? id, shown, missing: !entity };
    }),
  );
</script>

{#if states && entry}
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
{:else}
  <span class="muted">{waitingText(entry, 'homeassistant')}</span>
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
    font-size: min(180px, calc(150cqw / max(var(--length), 3)), 48cqh);
    font-weight: 300;
    line-height: 1;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .single .name {
    margin-top: 16px;
    font-size: min(40px, 9cqw, 12cqh);
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
  li {
    display: flex;
    gap: 24px;
    align-items: baseline;
    justify-content: space-between;
    min-width: 0;
  }
  .name {
    font-size: 32px;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .value {
    flex: none;
    font-size: 44px;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }
  .unit {
    margin-left: 0.15em;
    font-size: 0.6em;
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
  .muted {
    color: var(--text-muted);
    font-size: 38px;
  }
</style>
