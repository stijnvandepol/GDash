<script lang="ts">
  import { clock } from '../state.svelte';
  import { locale, t } from '../i18n.svelte';
  import { PHASE_TEXT, litPath, moonAt } from './moon';

  // The drawing on a 100 by 100 grid, with room for the outline.
  const CENTER = 50;
  const RADIUS = 46;

  // The phase changes slowly: once a minute, with the clock, is plenty.
  const moon = $derived(moonAt(clock.now));
  const percent = $derived(Math.round(moon.illumination * 100));
  const next = $derived(
    [
      { label: t('moonFull'), at: moon.nextFull },
      { label: t('moonNew'), at: moon.nextNew },
    ].sort((a, b) => a.at.getTime() - b.at.getTime()),
  );

  function day(date: Date): string {
    return new Intl.DateTimeFormat(locale(), { weekday: 'short', day: 'numeric', month: 'short' }).format(date);
  }
</script>

<div class="moon">
  <svg class="disc" viewBox="0 0 100 100" aria-hidden="true">
    <!-- The dark side is the lit part of the other half of the month. -->
    <path class="dark" d={litPath(1 - moon.illumination, !moon.waxing, CENTER, CENTER, RADIUS)} />
    <path class="lit" d={litPath(moon.illumination, moon.waxing, CENTER, CENTER, RADIUS)} />
    <circle class="edge" cx={CENTER} cy={CENTER} r={RADIUS} />
  </svg>
  <div class="text">
    <span class="phase">{t(PHASE_TEXT[moon.phase])}</span>
    <span class="percent">{t('moonLit', { percent })}</span>
    <dl class="next">
      {#each next as event (event.label)}
        <div>
          <dt>{event.label}</dt>
          <dd>{day(event.at)}</dd>
        </div>
      {/each}
    </dl>
  </div>
</div>

<style>
  .moon {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    justify-content: center;
    gap: 0.4em;
    width: 100%;
    min-width: 0;
    font-size: 34px;
  }
  .disc {
    flex: none;
    width: min(55cqh, 75cqw);
    height: min(55cqh, 75cqw);
  }
  /* On a dark theme the lit part is bright, in the text colour, and the
     dark side faint. A light theme turns that around as print does (a full
     moon an open circle, a new moon a dark disc) with --moon-lit,
     --moon-shadow and --moon-shadow-opacity; text-coloured light would read
     as a dark moon there. The edge keeps a new moon a moon. */
  .lit {
    fill: var(--moon-lit, var(--text-primary));
  }
  .dark {
    fill: var(--moon-shadow, var(--text-muted));
    fill-opacity: var(--moon-shadow-opacity, 0.12);
  }
  .edge {
    fill: none;
    stroke: var(--text-muted);
    stroke-opacity: 0.5;
    stroke-width: 1;
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
    max-width: 100%;
  }
  .phase,
  .percent,
  dt,
  dd {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .phase {
    font-size: min(40px, 12.5cqw, 13.5cqh);
    line-height: 1.2;
    color: var(--text-primary);
  }
  .percent {
    font-size: min(30px, 10cqw, 11cqh);
    line-height: 1.2;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .next {
    display: none;
    margin: 0.5em 0 0;
    font-size: min(28px, 8cqw, 9cqh);
    line-height: 1.25;
  }
  .next div {
    display: flex;
    gap: 0.5em;
  }
  dt {
    color: var(--text-muted);
  }
  dd {
    margin: 0;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }
  /* A wide tile puts the moon beside the words and has room for the next
     full and new moon. */
  @container (min-width: 420px) {
    .moon {
      flex-direction: row;
      align-items: center;
      gap: 0.8em;
    }
    .disc {
      width: min(80cqh, 40cqw);
      height: min(80cqh, 40cqw);
    }
    .phase {
      font-size: min(44px, 7cqw, 16cqh);
    }
    .percent {
      font-size: min(32px, 5cqw, 11cqh);
    }
    .next {
      font-size: min(28px, 4.5cqw, 10cqh);
    }
  }
  @container (min-width: 420px) and (min-height: 170px) {
    .next {
      display: block;
    }
  }
  /* A tall, narrow tile has room under the moon. */
  @container (max-width: 419px) and (min-height: 330px) {
    .next {
      display: block;
    }
  }
</style>
