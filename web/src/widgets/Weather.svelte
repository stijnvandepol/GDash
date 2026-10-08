<script lang="ts">
  import type { Entry } from '../protocol';
  import { clock } from '../state.svelte';
  import { LOCALE } from '../format';
  import { describeWeather } from './wmo';

  interface WeatherReport {
    temperatureC: number;
    weatherCode: number;
    windKmh: number;
    todayMaxC: number;
    todayMinC: number;
  }

  let { entry }: { entry: Entry | undefined } = $props();

  const report = $derived(isReport(entry?.value) ? entry.value : undefined);
  // The server decides how old data may get; the display only compares.
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
  const updatedAt = new Intl.DateTimeFormat(LOCALE, { hour: '2-digit', minute: '2-digit' });

  function isReport(v: unknown): v is WeatherReport {
    if (typeof v !== 'object' || v === null) return false;
    const r = v as Record<string, unknown>;
    return ['temperatureC', 'weatherCode', 'windKmh', 'todayMaxC', 'todayMinC'].every((k) => typeof r[k] === 'number');
  }
</script>

<div class="weather">
  {#if report && entry}
    <span class="temperature">{Math.round(report.temperatureC)}°</span>
    <span class="description">{describeWeather(report.weatherCode)}</span>
    <span class="range">{Math.round(report.todayMinC)}° / {Math.round(report.todayMaxC)}°</span>
    <span class="wind">Wind {Math.round(report.windKmh)} km/u</span>
    <span class="updated" class:stale>
      Bijgewerkt {updatedAt.format(new Date(entry.fetchedAt))}
    </span>
  {:else}
    <span class="description muted">Weer wordt opgehaald…</span>
  {/if}
</div>

<style>
  .weather {
    display: flex;
    flex-direction: column;
    justify-content: center;
    width: 100%;
  }
  .temperature {
    font-size: 220px;
    font-weight: 300;
    line-height: 1;
    color: var(--text-primary);
  }
  .description {
    margin-top: 16px;
    font-size: 56px;
    color: var(--text-primary);
  }
  .range,
  .wind {
    margin-top: 12px;
    font-size: 40px;
    color: var(--text-muted);
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
</style>
