<script lang="ts">
  import type { Entry } from '../protocol';
  import { currentLanguage, locale, t } from '../i18n.svelte';
  import { clock } from '../state.svelte';
  import { daysUntil, parseLocalDate } from './dates';
  import { waitingText } from './waiting';
  import { describeWeather } from './wmo';
  import WeatherIcon from './WeatherIcon.svelte';

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
  // The forecast starts today, but the last good value may be from
  // yesterday when fetching fails: days that have passed are left out.
  const days = $derived(
    forecastDays(entry?.value)
      .map((day) => ({ ...day, offset: daysUntil(day.date, clock.now) }))
      .filter((day) => day.offset !== null && day.offset >= 0)
      .slice(0, wanted),
  );
  const weekday = $derived(new Intl.DateTimeFormat(locale(), { weekday: 'short' }));

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

  /** "Today" for today, else the short weekday of the local date. */
  function dayName(date: string, offset: number | null): string {
    if (offset === 0) return t('today');
    // A date without time is the forecast's local day, not UTC midnight.
    const local = parseLocalDate(date);
    return local ? weekday.format(local) : '';
  }
</script>

{#if days.length > 0}
  <ol class="forecast" style:grid-template-columns="repeat({days.length}, 1fr)" style:--days={days.length}>
    {#each days as day, i (i)}
      <li title={describeWeather(day.weatherCode, currentLanguage()) ?? t('unknownWeather')}>
        <span class="day">{dayName(day.date, day.offset)}</span>
        <span class="icon"><WeatherIcon code={day.weatherCode} /></span>
        <span class="max">{Math.round(day.maxC)}°</span>
        <span class="min">{Math.round(day.minC)}°</span>
        <span class="rain">{day.rainChance}%</span>
      </li>
    {/each}
  </ol>
{:else}
  <span class="muted">{waitingText(entry, 'forecast')}</span>
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
    font-variant-numeric: tabular-nums;
  }
  .day {
    font-size: 30px;
    font-size: min(30px, calc(100cqw / var(--days) * 0.3), 14cqh);
    color: var(--text-muted);
    white-space: nowrap;
  }
  .icon {
    display: flex;
    font-size: 44px;
    font-size: min(44px, calc(100cqw / var(--days) * 0.4), 18cqh);
    color: var(--text-primary);
  }
  .max {
    font-size: 48px;
    font-size: min(48px, calc(100cqw / var(--days) * 0.42), 22cqh);
    font-weight: var(--weight-figure);
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
