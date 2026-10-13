<script lang="ts">
  import type { Entry } from '../protocol';
  import { clock } from '../state.svelte';
  import { locale, t } from '../i18n.svelte';
  import { waitingText } from './waiting';
  import { fitItems } from './fit';
  import { holidayName, holidaySettings, holidayWhen, isNow, readHolidays, upcomingHolidays } from './holidays';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const chosen = $derived(holidaySettings(settings));
  const holidays = $derived(readHolidays(entry?.value, chosen.region, chosen.show));
  const upcoming = $derived(holidays ? upcomingHolidays(holidays, clock.now, chosen.maxItems) : []);
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
</script>

<div class="holidays" use:fitItems>
  {#if holidays}
    <span class="title">{t('holidaysTitle')}</span>
    {#if upcoming.length === 0}
      <p class="empty" class:stale>{t('holidaysNone')}</p>
    {:else}
      <ul>
        {#each upcoming as h (`${h.type}:${h.kind}:${h.start}`)}
          <li class:now={isNow(h)}>
            <span class="name">{holidayName(h)}</span>
            <span class="when" class:stale>{holidayWhen(h, locale())}</span>
          </li>
        {/each}
      </ul>
    {/if}
  {:else}
    <span class="empty">{waitingText(entry, 'holidays')}</span>
  {/if}
</div>

<style>
  .holidays {
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
    gap: 0.45em;
    font-size: 34px;
    font-size: min(34px, 7cqw);
  }
  /* A narrow tile puts when under the name; a wide one beside it. */
  li {
    display: flex;
    flex-direction: column;
    min-width: 0;
    line-height: 1.2;
  }
  .name,
  .when {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .name {
    color: var(--text-primary);
  }
  .when {
    font-size: 0.8em;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  li.now .when {
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
  @container (min-width: 640px) {
    ul {
      gap: 0.3em;
      font-size: min(34px, 4.5cqw);
    }
    li {
      flex-direction: row;
      align-items: baseline;
      gap: 0.5em;
    }
    .name {
      flex: 1;
    }
    .when {
      flex: none;
      max-width: 60%;
      font-size: 0.85em;
    }
  }
  /* A tall tile has room for a heading. */
  @container (min-height: 300px) {
    .title {
      display: block;
    }
  }
</style>
