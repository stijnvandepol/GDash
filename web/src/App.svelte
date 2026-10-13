<script lang="ts">
  import './themes.css';
  import './theme.css';
  import { clock, display } from './state.svelte';
  import { t } from './i18n.svelte';
  import { msToNextPage, pageAt } from './pages';
  import { stageFor } from './stage';
  import { msUntilExpiry, visibleAnnouncement } from './announcement';
  import AnnouncementBanner from './AnnouncementBanner.svelte';

  // Spacing of theme.css in design pixels. A larger text size draws the
  // stage smaller and these with it, so only the text grows.
  const SPACE_GAP = 24;
  const SPACE_WIDGET = 48;
  const RADIUS_WIDGET = 28;
  import Clock from './widgets/Clock.svelte';
  import Weather from './widgets/Weather.svelte';
  import News from './widgets/News.svelte';
  import Forecast from './widgets/Forecast.svelte';
  import Sun from './widgets/Sun.svelte';
  import Moon from './widgets/Moon.svelte';
  import AirQuality from './widgets/AirQuality.svelte';
  import Waste from './widgets/Waste.svelte';
  import EnergyPrice from './widgets/EnergyPrice.svelte';
  import Solar from './widgets/Solar.svelte';
  import Note from './widgets/Note.svelte';
  import Countdown from './widgets/Countdown.svelte';
  import Holidays from './widgets/Holidays.svelte';
  import Photos from './widgets/Photos.svelte';
  import HomeAssistant from './widgets/HomeAssistant.svelte';
  import Homey from './widgets/Homey.svelte';
  import HomeyPresence from './widgets/HomeyPresence.svelte';
  import Departures from './widgets/Departures.svelte';
  import Fuel from './widgets/Fuel.svelte';
  import Rain from './widgets/Rain.svelte';
  import Warnings from './widgets/Warnings.svelte';
  import Custom from './widgets/Custom.svelte';
  import Calendar from './widgets/Calendar.svelte';
  import Camera from './widgets/Camera.svelte';
  import NowPlaying from './widgets/NowPlaying.svelte';

  // Dashboards are designed on a fixed 1920x1080 stage and scaled to the
  // screen, so a layout looks the same on a 720p Chromecast and a 4K TV.
  // The stage follows the screen's shape and the display's text size; see
  // stage.ts.
  let innerWidth = $state(1920);
  let innerHeight = $state(1080);
  const stage = $derived(stageFor(innerWidth, innerHeight, display.snapshot?.textScale));

  // A rotating display turns its page at every boundary of pageSeconds;
  // one timer to the next boundary, not a tick every second.
  let now = $state(Date.now());
  $effect(() => {
    const seconds = display.snapshot?.pageSeconds ?? 0;
    if (seconds <= 0 || (display.snapshot?.pages?.length ?? 0) < 2) return;
    now = Date.now();
    let timer = setTimeout(function turn() {
      now = Date.now();
      timer = setTimeout(turn, msToNextPage(now, seconds));
    }, msToNextPage(Date.now(), seconds));
    return () => clearTimeout(timer);
  });
  const dashboard = $derived(pageAt(display.snapshot, now));
  const asleep = $derived(display.snapshot?.active === false);

  // An announcement leaves the screen at its expiry on the server's clock,
  // by one timer; the server's next snapshot does not have to arrive first.
  // The margin makes sure the timer fires after the expiry, not just before.
  const EXPIRY_MARGIN_MS = 100;
  let announcementNow = $state(Date.now());
  $effect(() => {
    const nowMs = Date.now();
    announcementNow = nowMs;
    const remaining = msUntilExpiry(display.snapshot?.announcement, display.serverOffsetMs, nowMs);
    if (remaining === null || remaining <= 0) return;
    const timer = setTimeout(() => (announcementNow = Date.now()), remaining + EXPIRY_MARGIN_MS);
    return () => clearTimeout(timer);
  });
  const announcement = $derived(visibleAnnouncement(display.snapshot?.announcement, display.serverOffsetMs, announcementNow));

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

  /** The display this page shows, from its address /d/<id>. */
  function pairingDisplayId(): string {
    return location.pathname.split('/').filter(Boolean).pop() ?? '';
  }
</script>

<svelte:window bind:innerWidth bind:innerHeight />

<main class="screen" data-theme={dashboard?.theme ?? 'minimal-dark'} data-font={dashboard?.font || undefined}>
  {#if asleep}
    <!-- Outside active hours: nothing lit, so the TV can idle and OLED pixels rest. -->
  {:else if display.problem}
    <p class="message">{display.problem}</p>
  {:else if display.pairingCode}
    <div class="pairing">
      <p class="pairing-brand">
        <!-- The logo: a screen with its power light on. -->
        <svg viewBox="0 0 32 32" fill="none" aria-hidden="true">
          <rect x="2.5" y="4.5" width="27" height="19" rx="4" stroke="currentColor" stroke-width="3" />
          <circle cx="23.5" cy="18" r="2.5" fill="var(--signal)" />
          <rect x="11" y="26.5" width="10" height="3" rx="1.5" fill="currentColor" />
        </svg>
        GDash
      </p>
      <p class="pairing-title">{t('pairTitle')}</p>
      <p class="pairing-code">{display.pairingCode.code}</p>
      <!-- Where to go and what to pick: this page knows its own address and
           display, which is exactly what the person with the phone needs. -->
      <p class="pairing-help">{t('pairHelp', { address: location.host, id: pairingDisplayId() })}</p>
    </div>
  {:else if dashboard}
    <div
      class="stage"
      style:width="{stage.width}px"
      style:height="{stage.height}px"
      style:transform="translate(-50%, -50%) scale({stage.scale}) translate({shift[0]}px, {shift[1]}px)"
      style:--space-gap="{SPACE_GAP / stage.textScale}px"
      style:--space-widget="{SPACE_WIDGET / stage.textScale}px"
      style:--radius-widget="calc({RADIUS_WIDGET / stage.textScale}px * var(--radius-scale, 1))"
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
          {:else if widget.type === 'sun'}
            <Sun entry={widget.source ? display.snapshot?.data[widget.source] : undefined} />
          {:else if widget.type === 'moon'}
            <Moon />
          {:else if widget.type === 'airquality'}
            <AirQuality entry={widget.source ? display.snapshot?.data[widget.source] : undefined} />
          {:else if widget.type === 'waste'}
            <Waste entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'energyprice'}
            <EnergyPrice entry={widget.source ? display.snapshot?.data[widget.source] : undefined} />
          {:else if widget.type === 'solar'}
            <Solar entry={widget.source ? display.snapshot?.data[widget.source] : undefined} />
          {:else if widget.type === 'text'}
            <Note settings={widget.settings} />
          {:else if widget.type === 'countdown'}
            <Countdown settings={widget.settings} />
          {:else if widget.type === 'holidays'}
            <Holidays entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'photos'}
            <Photos entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'homeassistant'}
            <HomeAssistant entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'homey'}
            <Homey entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'homeypresence'}
            <HomeyPresence entry={widget.source ? display.snapshot?.data[widget.source] : undefined} key="{dashboard.id}/{widget.id}" />
          {:else if widget.type === 'departures'}
            <Departures entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'fuel'}
            <Fuel entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'rain'}
            <Rain entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'warnings'}
            <Warnings entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else if widget.type === 'camera'}
            <Camera settings={widget.settings} />
          {:else if widget.type === 'custom'}
            <Custom entry={widget.source ? display.snapshot?.data[widget.source] : undefined} key="{dashboard.id}/{widget.id}" />
          {:else if widget.type === 'nowplaying'}
            <NowPlaying entry={widget.source ? display.snapshot?.data[widget.source] : undefined} settings={widget.settings} />
          {:else}
            <p class="muted">{t('unknownWidget', { type: widget.type })}</p>
          {/if}
        </section>
      {/each}
      {#if announcement}
        <AnnouncementBanner title={announcement.title} message={announcement.message} />
      {/if}
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
    font-family: var(--font-ui);
    font-feature-settings: var(--numerals);
    font-weight: var(--weight-text);
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
    background: var(--tile-image), var(--surface-1);
    border: var(--widget-border-width) solid var(--widget-border);
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
  .pairing-brand {
    position: absolute;
    top: 6vmin;
    margin: 0;
    display: flex;
    align-items: center;
    gap: 1.2vmin;
    font-family: var(--font-display);
    font-size: 3.4vmin;
    font-weight: 700;
    letter-spacing: -0.05em;
    text-transform: lowercase;
    color: var(--text-primary);
  }
  .pairing-brand svg {
    width: 1.3em;
    height: 1.3em;
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
