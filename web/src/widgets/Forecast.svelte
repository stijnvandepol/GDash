<script lang="ts">
  import type { Entry } from '../protocol';
  import { LOCALE } from '../format';
  import { waitingText } from './waiting';
  import { describeWeather } from './wmo';

  interface Day {
    date: string;
    weatherCode: number;
    maxC: number;
    minC: number;
    rainChance: number;
  }

  const DEFAULT_DAYS = 5;

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const wanted = $derived(typeof settings?.days === 'number' ? settings.days : DEFAULT_DAYS);
  const days = $derived(forecastDays(entry?.value).slice(0, wanted));
  const weekday = new Intl.DateTimeFormat(LOCALE, { weekday: 'short' });

  function forecastDays(v: unknown): Day[] {
    if (typeof v !== 'object' || v === null) return [];
    const list = (v as { days?: unknown }).days;
    if (!Array.isArray(list)) return [];
    return list.filter(
      (d): d is Day =>
        typeof d === 'object' &&
        d !== null &&
        typeof d.date === 'string' &&
        ['weatherCode', 'maxC', 'minC', 'rainChance'].every((k) => typeof d[k] === 'number'),
    );
  }

  /** "Today" for the first day, else the short weekday of the local date. */
  function dayName(date: string, index: number): string {
    if (index === 0) return 'Today';
    const [y, m, d] = date.split('-').map(Number);
    // A date without time is the forecast's local day, not UTC midnight.
    return weekday.format(new Date(y!, m! - 1, d!));
  }
</script>

{#if days.length > 0}
  <ol class="forecast" style:grid-template-columns="repeat({days.length}, 1fr)" style:--days={days.length}>
    {#each days as day, i (day.date)}
      <li title={describeWeather(day.weatherCode)}>
        <span class="day">{dayName(day.date, i)}</span>
        <span class="max">{Math.round(day.maxC)}°</span>
        <span class="min">{Math.round(day.minC)}°</span>
        <span class="rain">{day.rainChance}%</span>
      </li>
    {/each}
  </ol>
{:else}
  <span class="muted">{waitingText(entry, 'forecast', 'No location set. Choose one under Settings.')}</span>
{/if}

<style>
  .forecast {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 12px;
    width: 100%;
    align-self: center;
  }
  li {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    min-width: 0;
  }
  .day {
    font-size: 30px;
    font-size: min(30px, calc(100cqw / var(--days) * 0.3), 14cqh);
    color: var(--text-muted);
    white-space: nowrap;
  }
  .max {
    font-size: 48px;
    font-size: min(48px, calc(100cqw / var(--days) * 0.42), 22cqh);
    color: var(--text-primary);
  }
  .min {
    font-size: 32px;
    font-size: min(32px, calc(100cqw / var(--days) * 0.32), 15cqh);
    color: var(--text-muted);
  }
  .rain {
    font-size: 26px;
    font-size: min(26px, calc(100cqw / var(--days) * 0.28), 12cqh);
    color: var(--text-muted);
  }
  .muted {
    color: var(--text-muted);
    font-size: 38px;
  }
</style>
