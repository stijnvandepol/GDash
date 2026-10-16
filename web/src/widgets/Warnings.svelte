<script lang="ts">
  import type { Entry } from '../protocol';
  import { clock } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import { fitItems } from './fit';
  import {
    LEVEL_TEXT,
    activeWarnings,
    readArea,
    warningHeadline,
    warningName,
    warningWindow,
    warningsKey,
    warningsWaitingText,
  } from './warnings';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const area = $derived(readArea(entry?.value, warningsKey(settings)));
  const list = $derived(area ? activeWarnings(area, clock.now.getTime()) : []);
</script>

<div class="warnings" use:fitItems>
  {#if area}
    {#if list.length > 0}
      <ol class="list">
        {#each list as w (w.id)}
          <li class="banner {w.level}">
            <span class="head">
              <span class="level">
                {#if w.level === 'red'}
                  <!-- Red is also marked with a sign, so it does not rely on colour alone. -->
                  <svg class="mark" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M12 2 1 21h22L12 2zM11 9h2v6h-2zM11 17h2v2h-2z" /></svg>
                {/if}
                {t(LEVEL_TEXT[w.level])}
              </span>
              <span class="what">{warningName(w)}</span>
              <span class="when">{warningWindow(w, clock.now.getTime())}</span>
            </span>
            {#if warningHeadline(w)}<span class="headline">{warningHeadline(w)}</span>{/if}
          </li>
        {/each}
      </ol>
    {:else}
      <p class="calm">{t('warningsNone', { region: area.region })}</p>
    {/if}
    <!-- MeteoAlarm asks that its feed is credited along with the KNMI's warnings. -->
    <span class="source">KNMI · MeteoAlarm</span>
  {:else}
    <span class="muted">{warningsWaitingText(entry, settings)}</span>
  {/if}
</div>

<style>
  .warnings {
    display: flex;
    flex-direction: column;
    gap: clamp(6px, 3cqh, 14px);
    width: 100%;
    min-width: 0;
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: clamp(6px, 2.5cqh, 12px);
    min-height: 0;
  }
  /* A banner per warning, set apart by a bar in the theme's warning colour
     that grows with the level, and by the level's name. */
  .banner {
    /* A stacking context of its own, so the tint (::before) sits between
       the banner's background and its text. */
    position: relative;
    z-index: 0;
    display: grid;
    gap: 2px;
    padding: 0.3em 0.5em 0.3em 0.6em;
    border-left: 0.22em solid var(--warning);
    border-radius: 4px;
    font-size: clamp(22px, min(6cqw, 19cqh), 38px);
    line-height: 1.2;
    color: var(--text-primary);
  }
  .banner.orange,
  .banner.red {
    border-left-width: 0.45em;
  }
  /* The warning colour as a faint tint: with opacity, since color-mix only
     exists from Chrome 111. */
  .banner.orange::before,
  .banner.red::before {
    content: '';
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: -1;
    border-radius: inherit;
    background: var(--warning);
    opacity: 0.1;
  }
  .banner.red::before {
    opacity: 0.22;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    column-gap: 0.45em;
    min-width: 0;
  }
  .level {
    font-weight: var(--weight-figure);
    color: var(--warning);
    white-space: nowrap;
  }
  /* Red's tint lowers the contrast of the warning colour, so its name is
     in the text colour; the sign and the tint mark it. */
  .red .level {
    color: var(--text-primary);
  }
  .mark {
    display: inline-block;
    width: 0.9em;
    height: 0.9em;
    margin-right: 0.2em;
    vertical-align: -0.1em;
  }
  .mark path {
    fill: var(--warning);
  }
  .what {
    font-weight: var(--weight-figure);
  }
  .when {
    color: var(--text-muted);
    font-size: 0.8em;
    white-space: nowrap;
  }
  .red .when,
  .orange .when {
    color: var(--text-primary);
  }
  .headline {
    display: none;
    font-size: 0.7em;
    color: var(--text-muted);
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    overflow: hidden;
    overflow-wrap: break-word;
  }
  .red .headline,
  .orange .headline {
    color: var(--text-primary);
  }
  .calm {
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
  .muted {
    color: var(--text-muted);
    font-size: clamp(22px, 6cqw, 38px);
  }
  /* A taller tile has room for the KNMI's own headline. */
  @container (min-height: 260px) {
    .headline {
      display: -webkit-box;
    }
  }
</style>
