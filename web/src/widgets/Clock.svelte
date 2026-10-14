<script lang="ts">
  import { clock } from '../state.svelte';
  import { locale } from '../i18n.svelte';

  const time = $derived(new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit' }));
  const date = $derived(new Intl.DateTimeFormat(locale(), { weekday: 'long', day: 'numeric', month: 'long' }));
  // The separators apart, so OLED care can keep the colon from being lit
  // in the same place all day (App.svelte).
  const parts = $derived(time.formatToParts(clock.now));
</script>

<div class="clock">
  <time class="time">{#each parts as part, i (i)}{#if part.type === 'literal'}<span class="sep">{part.value}</span>{:else}{part.value}{/if}{/each}</time>
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
  /* A box of the time's own line height, not the taller box of the face
     that a plain inline element has. */
  .sep {
    display: inline-block;
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
