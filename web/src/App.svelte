<script lang="ts">
  import './theme.css';
  import { display } from './state.svelte';
  import Clock from './widgets/Clock.svelte';
  import Weather from './widgets/Weather.svelte';
  import News from './widgets/News.svelte';

  // Dashboards are designed on a fixed 1920x1080 stage and scaled to the
  // screen, so a layout looks the same on a 720p Chromecast and a 4K TV.
  const STAGE_WIDTH = 1920;
  const STAGE_HEIGHT = 1080;

  let innerWidth = $state(STAGE_WIDTH);
  let innerHeight = $state(STAGE_HEIGHT);
  const scale = $derived(Math.min(innerWidth / STAGE_WIDTH, innerHeight / STAGE_HEIGHT));

  const dashboard = $derived(display.snapshot?.dashboard);
</script>

<svelte:window bind:innerWidth bind:innerHeight />

<main class="screen">
  {#if display.problem}
    <p class="message">{display.problem}</p>
  {:else if display.pairingCode}
    <div class="pairing">
      <p class="pairing-title">Pair this screen</p>
      <p class="pairing-code">{display.pairingCode.code}</p>
      <p class="pairing-help">
        Open Daycast on your phone or computer, choose this display and enter the code. It is valid for 10 minutes.
      </p>
    </div>
  {:else if dashboard}
    <div
      class="stage"
      style:width="{STAGE_WIDTH}px"
      style:height="{STAGE_HEIGHT}px"
      style:transform="translate(-50%, -50%) scale({scale})"
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
          {:else if widget.type === 'news'}
            <News entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else}
            <p class="muted">Unknown widget: {widget.type}</p>
          {/if}
        </section>
      {/each}
    </div>
  {:else}
    <p class="message">Connecting to Daycast…</p>
  {/if}

  {#if display.connection === 'offline'}
    <div class="offline" title="No connection to the Daycast server"></div>
  {/if}
</main>

<style>
  .screen {
    position: fixed;
    inset: 0;
    overflow: hidden;
    background: var(--surface-0);
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
    border-radius: var(--radius-widget);
    padding: var(--space-widget);
    overflow: hidden;
    display: flex;
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
