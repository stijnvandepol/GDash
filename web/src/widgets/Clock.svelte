<script lang="ts">
  import { clock } from '../state.svelte';
  import { LOCALE } from '../format';

  const time = new Intl.DateTimeFormat(LOCALE, { hour: '2-digit', minute: '2-digit' });
  const date = new Intl.DateTimeFormat(LOCALE, { weekday: 'long', day: 'numeric', month: 'long' });
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
    font-weight: 300;
    line-height: 1;
    letter-spacing: -0.02em;
    font-variant-numeric: tabular-nums;
    color: var(--text-primary);
  }
  .date {
    margin-top: 12px;
    font-size: 48px;
    color: var(--text-muted);
  }
</style>
