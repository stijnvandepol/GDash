<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { clock } from '../state.svelte';
  import { currentLanguage, locale, t } from '../i18n.svelte';
  import { describeWeather } from './wmo';
  import WeatherIcon from './WeatherIcon.svelte';
  import { humidityText, measuredText, readObservation, showsMeasured } from './knmi';

  interface WeatherReport {
    temperatureC: number;
    isDay?: boolean;
    weatherCode: number;
    windKmh: number;
    todayMaxC: number;
    todayMinC: number;
  }

  let {
    entry,
    settings,
    measured,
  }: {
    entry: Entry | undefined;
    settings?: Record<string, unknown> | undefined;
    /** What the nearest KNMI station measured, for a widget set to show it. */
    measured?: Entry | undefined;
  } = $props();

  const report = $derived(isReport(entry?.value) ? entry.value : undefined);
  // The server decides how old data may get; the display only compares.
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
  const updatedAt = $derived(new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit' }));
  // Measured values replace the model's current ones; until they arrive, or
  // when the KNMI fails without a last value, the model's are shown.
  const observation = $derived(showsMeasured(settings) ? readObservation(measured?.value) : undefined);
  const measuredStale = $derived(measured !== undefined && clock.now.getTime() > Date.parse(measured.staleAfter));
  const temperature = $derived(observation?.temperatureC ?? report?.temperatureC ?? 0);
  const windKmh = $derived(observation?.windKmh ?? report?.windKmh ?? 0);

  function isReport(v: unknown): v is WeatherReport {
    if (typeof v !== 'object' || v === null) return false;
    const r = v as Record<string, unknown>;
    return ['temperatureC', 'weatherCode', 'windKmh', 'todayMaxC', 'todayMinC'].every((k) => typeof r[k] === 'number');
  }
</script>

<div class="weather">
  <!-- Measured values show on their own too, while the model is out. -->
  {#if (report && entry) || observation}
    <div class="now">
      <span class="temperature">{Math.round(temperature)}°</span>
      {#if report}<span class="icon"><WeatherIcon code={report.weatherCode} day={report.isDay !== false} /></span>{/if}
    </div>
    {#if report}
      <span class="description">{describeWeather(report.weatherCode, currentLanguage()) ?? t('unknownWeather')}</span>
      <span class="range">{Math.round(report.todayMinC)}° / {Math.round(report.todayMaxC)}°</span>
    {/if}
    <span class="wind">
      {t('wind', { speed: Math.round(windKmh) })}{#if observation?.humidity != null}{' · '}{humidityText(observation.humidity)}{/if}
    </span>
    {#if observation}
      <span class="updated measured" class:stale={measuredStale}>{measuredText(observation)} · KNMI</span>
    {:else if entry && !Number.isNaN(Date.parse(entry.fetchedAt))}
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
  /* A small tile keeps the temperature, the sky and today's range, and
     where a measured temperature comes from: it says the number is not the
     model's. */
  @container (max-height: 420px) {
    .wind,
    .updated:not(.measured) {
      display: none;
    }
    .temperature {
      font-size: min(220px, 50cqw, 44cqh);
    }
    .icon {
      font-size: min(140px, 32cqw, 30cqh);
    }
    .description {
      margin-top: min(16px, 3cqh);
      font-size: min(56px, 12cqw, 13cqh);
    }
    .range {
      margin-top: min(12px, 3cqh);
      font-size: min(40px, 11cqh);
    }
    .measured {
      margin-top: min(12px, 3cqh);
      font-size: max(22px, min(28px, 8cqh));
    }
  }
  @container (max-height: 260px) {
    .range {
      display: none;
    }
  }
</style>
