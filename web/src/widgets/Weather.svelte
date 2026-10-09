<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { clock } from '../state.svelte';
  import { currentLanguage, locale, t } from '../i18n.svelte';
  import { describeWeather } from './wmo';
  import WeatherIcon from './WeatherIcon.svelte';

  interface WeatherReport {
    temperatureC: number;
    isDay?: boolean;
    weatherCode: number;
    windKmh: number;
    todayMaxC: number;
    todayMinC: number;
  }

  let { entry }: { entry: Entry | undefined } = $props();

  const report = $derived(isReport(entry?.value) ? entry.value : undefined);
  // The server decides how old data may get; the display only compares.
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
  const updatedAt = $derived(new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit' }));

  function isReport(v: unknown): v is WeatherReport {
    if (typeof v !== 'object' || v === null) return false;
    const r = v as Record<string, unknown>;
    return ['temperatureC', 'weatherCode', 'windKmh', 'todayMaxC', 'todayMinC'].every((k) => typeof r[k] === 'number');
  }
</script>

<div class="weather">
  {#if report && entry}
    <div class="now">
      <span class="temperature">{Math.round(report.temperatureC)}°</span>
      <span class="icon"><WeatherIcon code={report.weatherCode} day={report.isDay !== false} /></span>
    </div>
    <span class="description">{describeWeather(report.weatherCode, currentLanguage()) ?? t('unknownWeather')}</span>
    <span class="range">{Math.round(report.todayMinC)}° / {Math.round(report.todayMaxC)}°</span>
    <span class="wind">{t('wind', { speed: Math.round(report.windKmh) })}</span>
    {#if !Number.isNaN(Date.parse(entry.fetchedAt))}
      <span class="updated" class:stale>
        {t('updated', { time: updatedAt.format(new Date(entry.fetchedAt)) })}
      </span>
    {/if}
  {:else}
    <span class="description muted">{waitingText(entry, 'weather')}</span>
  {/if}
</div>

<style>
  .weather {
    display: flex;
    flex-direction: column;
    justify-content: center;
    width: 100%;
  }
  .now {
    display: flex;
    align-items: center;
    gap: 0.25em;
  }
  .icon {
    display: flex;
    font-size: 140px;
    font-size: min(140px, 32cqw, 32cqh);
    color: var(--text-primary);
  }
  .temperature {
    font-size: 220px;
    font-size: min(220px, 50cqw, 48cqh);
    font-weight: var(--weight-figure);
    line-height: 1;
    font-variant-numeric: tabular-nums;
    color: var(--text-primary);
  }
  .description {
    margin-top: 16px;
    font-size: 56px;
    font-size: min(56px, 12cqw, 14cqh);
    color: var(--text-primary);
  }
  .range,
  .wind {
    margin-top: 12px;
    font-size: 40px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .updated {
    margin-top: auto;
    font-size: 28px;
    color: var(--text-muted);
  }
  .updated.stale {
    color: var(--warning);
  }
  .muted {
    color: var(--text-muted);
  }
  /* A small tile keeps the temperature, the sky and today's range. */
  @container (max-height: 420px) {
    .wind,
    .updated {
      display: none;
    }
  }
  @container (max-height: 260px) {
    .range {
      display: none;
    }
  }
</style>
