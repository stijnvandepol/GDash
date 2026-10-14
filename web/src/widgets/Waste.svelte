<script lang="ts">
  import type { Entry } from '../protocol';
  import { clock } from '../state.svelte';
  import { locale, t } from '../i18n.svelte';
  import { calendarColor } from '../colors';
  import { waitingText } from './waiting';
  import { fitItems } from './fit';
  import { WASTE_COLOR, WASTE_TEXT, nextPickups, pickupDay, readWaste, type NextPickup } from './waste';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const pickups = $derived(readWaste(entry?.value, settings?.calendar));
  const next = $derived(Array.isArray(pickups) ? nextPickups(pickups, clock.now) : []);
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));

  function label(p: NextPickup): string {
    return p.kind === 'other' ? p.title : t(WASTE_TEXT[p.kind]);
  }
</script>

<div class="waste" use:fitItems>
  {#if Array.isArray(pickups)}
    <span class="title">{t('wasteTitle')}</span>
    {#if next.length === 0}
      <p class="empty" class:stale>{t('wasteNone')}</p>
    {:else}
      <ul>
        {#each next as p (p.kind === 'other' ? `other:${p.title}` : p.kind)}
          <li>
            <span class="dot" style:--dot={calendarColor(WASTE_COLOR[p.kind]) ?? 'var(--text-muted)'}></span>
            <span class="kind">{label(p)}</span>
            <span class="when" class:soon={p.days <= 1} class:stale>{pickupDay(p, { today: t('today'), tomorrow: t('tomorrow') }, locale())}</span>
          </li>
        {/each}
      </ul>
    {/if}
  {:else if pickups === 'missing'}
    <span class="empty">{t('wasteMissing')}</span>
  {:else if pickups === 'choose'}
    <span class="empty">{t('wasteChoose')}</span>
  {:else}
    <span class="empty">{waitingText(entry, 'waste')}</span>
  {/if}
</div>

<style>
  .waste {
    display: flex;
    flex-direction: column;
    width: 100%;
    min-width: 0;
    overflow: hidden;
  }
  .title {
    display: none;
    margin-bottom: 12px;
    font-family: var(--font-display);
    font-size: 34px;
    font-weight: 500;
    color: var(--text-muted);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.3em;
    /* As large as three lines allow in the tile's height (22% each, with
       the gaps) and as the width allows for a kind and its day. */
    font-size: 34px;
    font-size: min(52px, 11cqw, 22cqh);
  }
  li {
    display: flex;
    align-items: baseline;
    gap: 0.5em;
    min-width: 0;
    line-height: 1.2;
  }
  .dot {
    flex: none;
    align-self: center;
    width: 0.55em;
    height: 0.55em;
    border-radius: 50%;
    /* A theme can recolour every dot, as Night does to stay free of blue. */
    background: var(--dot-tint, var(--dot));
  }
  .kind {
    flex: 1;
    min-width: 0;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .when {
    flex: none;
    font-size: 0.85em;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .when.soon {
    color: var(--accent);
  }
  .when.stale,
  .empty.stale {
    color: var(--warning);
  }
  .empty {
    margin: 0;
    font-size: 38px;
    color: var(--text-muted);
  }
  /* A tall tile has room for a heading. */
  @container (min-height: 300px) {
    .title {
      display: block;
    }
  }
</style>
