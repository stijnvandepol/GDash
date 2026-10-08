<script lang="ts">
  import { clock } from '../state.svelte';

  const DAY_MS = 86_400_000;

  let { settings }: { settings: Record<string, unknown> | undefined } = $props();

  const label = $derived(typeof settings?.label === 'string' ? settings.label : '');
  const days = $derived(daysUntil(settings?.date, clock.now));

  /** Whole days from today until a YYYY-MM-DD date, in the screen's time zone. */
  function daysUntil(date: unknown, now: Date): number | null {
    if (typeof date !== 'string') return null;
    const [y, m, d] = date.split('-').map(Number);
    if (!y || !m || !d) return null;
    const target = new Date(y, m - 1, d);
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    // Rounding absorbs the hour that a daylight saving change adds or removes.
    return Math.round((target.getTime() - today.getTime()) / DAY_MS);
  }
</script>

<div class="countdown">
  <span class="label">{label}</span>
  {#if days === null}
    <span class="when">–</span>
  {:else if days > 1}
    <span class="number">{days}</span>
    <span class="when">days to go</span>
  {:else if days === 1}
    <span class="number">1</span>
    <span class="when">day to go</span>
  {:else if days === 0}
    <span class="number">Today</span>
  {:else}
    <span class="when">Has passed</span>
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
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .number {
    font-size: 110px;
    font-size: min(110px, 40cqw, 52cqh);
    font-weight: 300;
    line-height: 1.05;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }
  .when {
    font-size: 32px;
    color: var(--text-muted);
  }
</style>
