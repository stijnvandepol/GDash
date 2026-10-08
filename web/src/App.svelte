<script lang="ts">
  import './theme.css';
  import { clock, display } from './state.svelte';
  import { t } from './i18n.svelte';
  import Clock from './widgets/Clock.svelte';
  import Weather from './widgets/Weather.svelte';
  import News from './widgets/News.svelte';
  import Forecast from './widgets/Forecast.svelte';
  import Note from './widgets/Note.svelte';
  import Countdown from './widgets/Countdown.svelte';
  import Calendar from './widgets/Calendar.svelte';

  // Dashboards are designed on a fixed 1920x1080 stage and scaled to the
  // screen, so a layout looks the same on a 720p Chromecast and a 4K TV.
  const STAGE_WIDTH = 1920;
  const STAGE_HEIGHT = 1080;

  let innerWidth = $state(STAGE_WIDTH);
  let innerHeight = $state(STAGE_HEIGHT);
  const scale = $derived(Math.min(innerWidth / STAGE_WIDTH, innerHeight / STAGE_HEIGHT));

  const dashboard = $derived(display.snapshot?.dashboard);
  const asleep = $derived(display.snapshot?.active === false);

  // Pixel shift against burn-in: every few minutes the whole stage moves a
  // few pixels, so no edge stays lit on the same OLED pixels all day.
  const SHIFT_EVERY_MINUTES = 3;
  const SHIFTS: [number, number][] = [
    [0, 0],
    [6, 3],
    [-4, 6],
    [-6, -3],
    [4, -6],
  ];
  const shift = $derived(SHIFTS[Math.floor(clock.now.getTime() / 60000 / SHIFT_EVERY_MINUTES) % SHIFTS.length] ?? [0, 0]);
</script>

<svelte:window bind:innerWidth bind:innerHeight />

<main class="screen" data-theme={dashboard?.theme ?? 'minimal-dark'}>
  {#if asleep}
    <!-- Outside active hours: nothing lit, so the TV can idle and OLED pixels rest. -->
  {:else if display.problem}
    <p class="message">{display.problem}</p>
  {:else if display.pairingCode}
    <div class="pairing">
      <p class="pairing-title">{t('pairTitle')}</p>
      <p class="pairing-code">{display.pairingCode.code}</p>
      <p class="pairing-help">{t('pairHelp')}</p>
    </div>
  {:else if dashboard}
    <div
      class="stage"
      style:width="{STAGE_WIDTH}px"
      style:height="{STAGE_HEIGHT}px"
      style:transform="translate(-50%, -50%) scale({scale}) translate({shift[0]}px, {shift[1]}px)"
      style:grid-template-columns="repeat({dashboard.columns}, 1fr)"
      style:grid-template-rows="repeat({dashboard.rows}, 1fr)"
    >
      {#each dashboard.widgets as widget (widget.id)}
        <section
          class="widget"
          style:grid-column="{widget.x + 1} / span {widget.w}"
          style:grid-row="{widget.y + 1} / span {widget.h}"
        >
          {#if widget.type === 'clock'}
            <Clock />
          {:else if widget.type === 'weather'}
            <Weather entry={widget.source ? display.snapshot?.data[widget.source] : undefined} />
          {:else if widget.type === 'calendar'}
            <Calendar entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'news'}
            <News entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'forecast'}
            <Forecast entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'text'}
            <Note settings={widget.settings} />
          {:else if widget.type === 'countdown'}
            <Countdown settings={widget.settings} />
          {:else}
            <p class="muted">{t('unknownWidget', { type: widget.type })}</p>
          {/if}
        </section>
      {/each}
    </div>
  {:else}
    <p class="message">{t('connecting')}</p>
  {/if}

  {#if display.connection === 'offline' && !asleep}
    <div class="offline" title={t('offline')}></div>
  {/if}
</main>

<style>
  .screen {
    position: fixed;
    inset: 0;
    overflow: hidden;
    background: var(--backdrop), var(--surface-0);
    color: var(--text-primary);
  }
  .stage {
    position: absolute;
    top: 50%;
    left: 50%;
    transform-origin: center;
    display: grid;
    gap: var(--space-gap);
    padding: var(--space-gap);
    box-sizing: border-box;
  }
  .widget {
    background: var(--surface-1);
    border: 1px solid var(--widget-border);
    border-radius: var(--radius-widget);
    padding: var(--space-widget);
    overflow: hidden;
    display: flex;
    /* Widgets size their type to the tile with cqw and cqh units. */
    container-type: size;
  }
  .message {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    margin: 0;
    padding: 0 10vw;
    text-align: center;
    color: var(--text-muted);
    font-size: 3vmin;
  }
  .pairing {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 3vmin;
    text-align: center;
  }
  .pairing-title {
    margin: 0;
    font-size: 4vmin;
    color: var(--text-muted);
  }
  .pairing-code {
    margin: 0;
    font-size: 14vmin;
    font-weight: 600;
    letter-spacing: 0.08em;
    font-variant-numeric: tabular-nums;
    color: var(--text-primary);
  }
  .pairing-help {
    margin: 0;
    max-width: 60vw;
    font-size: 2.6vmin;
    line-height: 1.4;
    color: var(--text-muted);
  }
  .muted {
    color: var(--text-muted);
  }
  .offline {
    position: absolute;
    right: 1.5vmin;
    bottom: 1.5vmin;
    width: 1.2vmin;
    height: 1.2vmin;
    border-radius: 50%;
    background: var(--warning);
  }
</style>
