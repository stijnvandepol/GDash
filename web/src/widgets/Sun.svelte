<script lang="ts">
  import type { Entry } from '../protocol';
  import { clock } from '../state.svelte';
  import { locale, t } from '../i18n.svelte';
  import { waitingText } from './waiting';
  import { daylightChange, daylightMinutes, formatClock, pickDays, splitMinutes, sunDays } from './sun';
  import SunIcon from './SunIcon.svelte';

  let { entry }: { entry: Entry | undefined } = $props();

  const picked = $derived(pickDays(sunDays(entry?.value), clock.now));
  const sunrise = $derived(picked.today ? formatClock(picked.today.sunrise, locale()) : null);
  const sunset = $derived(picked.today ? formatClock(picked.today.sunset, locale()) : null);
  const length = $derived(daylightMinutes(picked.today));
  const change = $derived(daylightChange(picked.today, picked.yesterday));

  function duration(minutes: number): string {
    return t('sunDuration', splitMinutes(minutes));
  }
</script>

<div class="sun">
  {#if sunrise && sunset}
    <div class="events">
      <div class="event">
        <span class="icon"><SunIcon kind="sunrise" /></span>
        <span class="text">
          <span class="label">{t('sunrise')}</span>
          <time class="time">{sunrise}</time>
        </span>
      </div>
      <div class="event">
        <span class="icon"><SunIcon kind="sunset" /></span>
        <span class="text">
          <span class="label">{t('sunset')}</span>
          <time class="time">{sunset}</time>
        </span>
      </div>
    </div>
    {#if length !== null}
      <span class="length">{t('sunDaylight', { duration: duration(length) })}</span>
    {/if}
    {#if change}
      <span class="change">
        {#if change.kind === 'same'}
          {t('sunSame')}
        {:else}
          {t(change.kind === 'longer' ? 'sunLonger' : 'sunShorter', { minutes: change.minutes })}
        {/if}
      </span>
    {/if}
  {:else if picked.today}
    <span class="muted">{t('sunNone')}</span>
  {:else}
    <span class="muted">{waitingText(entry, 'sun')}</span>
  {/if}
</div>

<style>
  .sun {
    display: flex;
    flex-direction: column;
    justify-content: center;
    width: 100%;
  }
  .events {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .event {
    display: flex;
    align-items: center;
    gap: 0.25em;
    font-size: 100px;
    font-size: min(100px, 22cqw, 28cqh);
  }
  .icon {
    display: flex;
    color: var(--text-primary);
  }
  .text {
    display: flex;
    flex-direction: column;
  }
  .label {
    display: none;
    font-size: 0.34em;
    color: var(--text-muted);
  }
  .time {
    font-size: 0.8em;
    font-weight: var(--weight-figure);
    line-height: 1.05;
    font-variant-numeric: tabular-nums;
    color: var(--text-primary);
  }
  /* One line each, whatever the typeface: a wrapped line would push the
     times out of the top of the tile. */
  .length,
  .change {
    max-width: 100%;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .length {
    margin-top: 16px;
    font-size: 44px;
    font-size: min(44px, 10cqw, 13cqh);
    color: var(--text-primary);
  }
  .change {
    margin-top: 6px;
    font-size: 34px;
    font-size: min(34px, 7.5cqw, 10cqh);
    color: var(--text-muted);
  }
  .muted {
    color: var(--text-muted);
    font-size: 38px;
  }
  /* A wide tile puts sunrise and sunset side by side and uses the height. */
  @container (min-width: 480px) {
    .events {
      flex-direction: row;
      gap: 1.2em;
    }
    .event {
      font-size: min(120px, 11cqw, 36cqh);
    }
  }
  /* A tall tile has room to say which is which. */
  @container (min-height: 300px) {
    .label {
      display: block;
    }
  }
</style>
