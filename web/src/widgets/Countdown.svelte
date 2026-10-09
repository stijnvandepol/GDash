<script lang="ts">
  import { clock } from '../state.svelte';
  import { daysUntil } from './dates';
  import { t } from '../i18n.svelte';

  let { settings }: { settings: Record<string, unknown> | undefined } = $props();

  const label = $derived(typeof settings?.label === 'string' ? settings.label : '');
  const days = $derived(typeof settings?.date === 'string' ? daysUntil(settings.date, clock.now) : null);
</script>

<div class="countdown">
  <span class="label">{label}</span>
  {#if days === null}
    <span class="when">–</span>
  {:else if days > 1}
    <span class="number">{days}</span>
    <span class="when">{t('daysToGo')}</span>
  {:else if days === 1}
    <span class="number">1</span>
    <span class="when">{t('dayToGo')}</span>
  {:else if days === 0}
    <span class="number">{t('today')}</span>
  {:else}
    <span class="when">{t('hasPassed')}</span>
  {/if}
</div>

<style>
  .countdown {
    display: flex;
    flex-direction: column;
    justify-content: center;
    width: 100%;
    min-width: 0;
  }
  .label {
    font-size: 36px;
    font-size: min(36px, 15cqh);
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .number {
    font-size: 110px;
    font-size: min(110px, 40cqw, 44cqh);
    font-weight: var(--weight-figure);
    line-height: 1.05;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }
  .when {
    font-size: 32px;
    font-size: min(32px, 13cqh);
    color: var(--text-muted);
  }
</style>
