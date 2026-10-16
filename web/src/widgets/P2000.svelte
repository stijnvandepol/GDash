<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { fitItems } from './fit';
  import { clock, display } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import { DISCIPLINE_TEXT, agoText, coveredHours, readValue, rowsFor } from './p2000';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  // The screen's clock, corrected when it is far off, so "12 min" is right.
  const now = $derived(new Date(clock.now.getTime() + display.serverOffsetMs));
  const value = $derived(readValue(entry?.value));
  const rows = $derived(value ? rowsFor(value, settings, now) : undefined);
</script>

<div class="p2000">
  {#if rows === 'skipped'}
    <p class="muted">{t('p2000Skipped')}</p>
  {:else if rows}
    {#if rows.length > 0}
      <ol class="alerts" use:fitItems>
        {#each rows as row (row.id)}
          <li>
            <span class="mark {row.discipline}" aria-hidden="true"></span>
            <span class="visually-hidden">{DISCIPLINE_TEXT[row.discipline] ? t(DISCIPLINE_TEXT[row.discipline]!) : ''}</span>
            <span class="text">{row.text}</span>
            <span class="ago">{agoText(row.minutes, t)}</span>
          </li>
        {/each}
      </ol>
    {:else}
      <p class="muted">{t('p2000None', { hours: value ? coveredHours(value, settings, now) : 0 })}</p>
    {/if}
  {:else}
    <span class="muted">{waitingText(entry, 'p2000')}</span>
  {/if}
  <!-- alarmeringen.nl publishes its feeds under CC BY-NC-ND, which asks for credit. -->
  {#if rows}<span class="source">Alarmeringen.nl</span>{/if}
</div>

<style>
  .p2000 {
    display: flex;
    flex-direction: column;
    gap: clamp(6px, 3cqh, 14px);
    width: 100%;
    min-width: 0;
  }
  .alerts {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: clamp(6px, 2.5cqh, 12px);
    align-content: start;
    /* Bounded by the tile, so fitItems hides what does not fit above the
       credit. */
    flex: 1 1 0;
    min-height: 0;
    overflow: hidden;
  }
  li {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    gap: 0 min(16px, 3cqw);
    align-items: baseline;
    font-size: clamp(22px, min(5.4cqw, 17cqh), 36px);
    line-height: 1.25;
    color: var(--text-primary);
  }
  /* A dot in the colour each service is known by. The colour is a mark,
     never text, so it needs no contrast with every theme's tiles; the
     service is named for screen readers. */
  .mark {
    width: 0.55em;
    height: 0.55em;
    border-radius: 50%;
    background: var(--text-muted);
  }
  .mark.brandweer {
    background: #e53935;
  }
  .mark.politie {
    background: #1e88e5;
  }
  .mark.ambulance {
    background: #43a047;
  }
  .mark.knrm,
  .mark.trauma {
    background: #fb8c00;
  }
  .text {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .ago {
    color: var(--text-muted);
    font-size: 0.8em;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .muted {
    margin: 0;
    color: var(--text-muted);
    font-size: clamp(22px, min(6cqw, 19cqh), 38px);
  }
  .source {
    flex: none;
    margin-top: auto;
    align-self: flex-end;
    font-size: clamp(13px, 2.8cqw, 18px);
    line-height: 1;
    color: var(--text-muted);
  }
  .visually-hidden {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }
</style>
