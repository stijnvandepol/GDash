<script lang="ts">
  import type { Entry } from '../protocol';
  import { fitItems } from './fit';
  import { clock } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import {
    DEFAULT_ITEMS,
    FUEL_TEXT,
    distanceText,
    fuelKey,
    fuelOf,
    fuelRows,
    fuelWaitingText,
    priceText,
    radiusOf,
    readPlace,
    savingText,
    stationList,
  } from './fuel';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const fuel = $derived(fuelOf(settings));
  const place = $derived(readPlace(settings?.place));
  const maxItems = $derived(typeof settings?.maxItems === 'number' ? settings.maxItems : DEFAULT_ITEMS);
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
  const list = $derived(stationList(entry?.value, fuelKey(settings)));
  const rows = $derived(list ? fuelRows(list, maxItems) : undefined);
  const saving = $derived(rows ? savingText(rows, fuel) : undefined);
</script>

<div class="fuel" use:fitItems>
  {#if rows}
    <h2 class="title" class:stale>
      <span class="kind">{t('fuelTitle')}</span>
      {#if place}<span class="place">{place.name}</span>{/if}
      <span class="type">{t(FUEL_TEXT[fuel]!)}</span>
    </h2>
    {#if rows.length > 0}
      <ol class="stations">
        {#each rows as row, i (i)}
          <li class:cheapest={row.cheapest}>
            <span class="price">{priceText(row.price)}</span>
            <span class="station">
              <span class="name">{row.name}</span>
              <span class="meta">{distanceText(row.distanceKm)}{#if row.cheapest && saving}<span class="saving">{' · '}{saving}</span>{/if}</span>
            </span>
          </li>
        {/each}
      </ol>
    {:else}
      <p class="muted">{t('fuelNone', { km: radiusOf(settings) })}</p>
    {/if}
  {:else}
    <span class="muted">{fuelWaitingText(entry, settings)}</span>
  {/if}
</div>

<style>
  .fuel {
    display: flex;
    flex-direction: column;
    width: 100%;
    min-width: 0;
  }
  .title {
    display: flex;
    gap: 0.4em;
    margin: 0 0 clamp(8px, 4cqh, 20px);
    font-family: var(--font-display);
    font-size: clamp(24px, 6.5cqw, 36px);
    font-weight: 500;
    color: var(--text-muted);
    white-space: nowrap;
  }
  .title.stale {
    color: var(--warning);
  }
  .kind,
  .type {
    flex: none;
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
  .place::before,
  .type::before {
    content: '· ';
  }
  .stations {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: clamp(8px, 3cqh, 16px);
  }
  li {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 0 min(20px, 4cqw);
    align-items: center;
    font-size: clamp(28px, 7cqw, 44px);
    line-height: 1.15;
    color: var(--text-primary);
  }
  .price {
    font-weight: var(--weight-figure);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .station {
    display: grid;
    min-width: 0;
  }
  .name,
  .meta {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .name {
    font-size: 0.66em;
  }
  .meta {
    font-size: 0.54em;
    color: var(--text-muted);
  }
  /* The cheapest row is set apart by colour and by the saving it names, so
     it is also found without telling the colours apart. */
  .cheapest .price,
  .cheapest .saving {
    color: var(--accent);
  }
  .muted {
    margin: 0;
    color: var(--text-muted);
    font-size: clamp(24px, 6.5cqw, 38px);
  }
</style>
