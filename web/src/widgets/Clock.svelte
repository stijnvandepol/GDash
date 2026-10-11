<script lang="ts">
  import { clock } from '../state.svelte';
  import { locale } from '../i18n.svelte';

  const time = $derived(new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit' }));
  const date = $derived(new Intl.DateTimeFormat(locale(), { weekday: 'long', day: 'numeric', month: 'long' }));
</script>

<div class="clock">
  <time class="time">{time.format(clock.now)}</time>
  <span class="date">{date.format(clock.now)}</span>
</div>

<style>
  .clock {
    display: flex;
    flex-direction: column;
    justify-content: center;
    width: 100%;
  }
  .time {
    font-size: 172px;
    font-size: min(172px, 34cqw, 64cqh);
    font-weight: var(--weight-figure);
    line-height: 1;
    letter-spacing: -0.015em;
    font-variant-numeric: tabular-nums;
    color: var(--text-primary);
  }
  .date {
    margin-top: 12px;
    font-family: var(--font-display);
    font-size: 48px;
    font-size: min(48px, 10cqw, 18cqh);
    font-weight: var(--weight-text, 400);
    color: var(--text-muted);
  }
</style>
