<script lang="ts">
  import type { Entry } from '../protocol';
  import { t } from '../i18n.svelte';
  import { waitingText } from './waiting';
  import { fitItems } from './fit';
  import { AQI_LEVELS, AQI_TEXT, POLLEN_LEVEL_TEXT, POLLEN_TEXT, aqiLevel, pollenLevel, readAirQuality } from './airquality';

  // From this level on, the air is a reason to take care.
  const FIRST_WARNING_LEVEL = AQI_LEVELS.indexOf('poor');

  let { entry }: { entry: Entry | undefined } = $props();

  const air = $derived(readAirQuality(entry?.value));
  const level = $derived(air ? aqiLevel(air.index) : undefined);
  const levelNumber = $derived(level ? AQI_LEVELS.indexOf(level) : 0);
</script>

<div class="air" use:fitItems>
  {#if air && level}
    <span class="title">{t('airQuality')}</span>
    <div class="index">
      <span class="number">{air.index}</span>
      <span class="level" class:warning={levelNumber >= FIRST_WARNING_LEVEL}>{t(AQI_TEXT[level])}</span>
    </div>
    <div class="meter" aria-hidden="true">
      {#each AQI_LEVELS as step, i (step)}
        <span class:filled={i <= levelNumber}></span>
      {/each}
    </div>
    {#if air.hasPollen}
      <ul class="pollen">
        {#each air.pollen as kind (kind.type)}
          <li>
            <span class="kind">{t(POLLEN_TEXT[kind.type])}</span>
            <span class="amount">{t(POLLEN_LEVEL_TEXT[pollenLevel(kind.grainsPerM3)])}</span>
          </li>
        {:else}
          <li class="none">{t('pollenNone')}</li>
        {/each}
      </ul>
    {/if}
  {:else}
    <span class="muted">{waitingText(entry, 'airquality')}</span>
  {/if}
</div>

<style>
  .air {
    display: flex;
    flex-direction: column;
    justify-content: center;
    width: 100%;
    min-width: 0;
  }
  .title {
    display: none;
    margin-bottom: 12px;
    font-size: 30px;
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .index {
    display: flex;
    align-items: baseline;
    gap: 0.3em;
    font-size: 100px;
    font-size: min(100px, 22cqw, 30cqh);
  }
  .number {
    font-size: 1.4em;
    font-weight: 300;
    line-height: 1;
    font-variant-numeric: tabular-nums;
    color: var(--text-primary);
  }
  .level {
    font-size: 0.55em;
    color: var(--text-primary);
  }
  .level.warning {
    color: var(--warning);
  }
  .meter {
    display: grid;
    grid-template-columns: repeat(6, 1fr);
    gap: 6px;
    margin-top: 12px;
  }
  .meter span {
    height: 10px;
    height: min(14px, 3cqh);
    border-radius: 7px;
    background: var(--text-muted);
    opacity: 0.35;
  }
  .meter span.filled {
    background: var(--text-primary);
    opacity: 1;
  }
  .pollen {
    list-style: none;
    margin: 18px 0 0;
    padding: 0;
    display: grid;
    gap: 6px;
    font-size: 34px;
    font-size: min(34px, 9cqw, 11cqh);
  }
  .pollen li {
    display: flex;
    justify-content: space-between;
    gap: 0.6em;
    color: var(--text-primary);
  }
  .pollen .amount,
  .pollen .none {
    color: var(--text-muted);
  }
  .muted {
    color: var(--text-muted);
    font-size: 38px;
  }
  /* A tall tile has room for a heading. */
  @container (min-height: 300px) {
    .title {
      display: block;
    }
  }
</style>
