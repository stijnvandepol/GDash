<script lang="ts">
  import type { Dashboard, DimAtNight, Display, ScreenCare, ScreenCareChanges } from './api';
  import { castTone, describeAge, describeCast, describeScreensaver } from './status';
  import { describeBrowser, describeScreenSize, describeVisibility, screenWarnings } from './screenReports';
  import { DEFAULT_DASHBOARD_ID, describeHours, describeScreens } from './displays';
  import Icon from './Icon.svelte';
  import DashboardThumb from './DashboardThumb.svelte';
  import PairScreen from './PairScreen.svelte';

  // One display in the list: what its screen is doing, what it shows, and
  // what can be done with it. Renaming and the schedule stay in
  // Displays.svelte, which swaps this item for them.
  let {
    display: d,
    dashboards,
    textSizes,
    screensaverChoices,
    screenCareChoices,
    dimChoices,
    grid,
    tvName,
    ondashboard,
    ontextscale,
    onscreensaver,
    onscreencare,
    onschedule,
    onpair,
    onunpair,
    ontakeover,
    onrename,
    onremove,
    onerror,
  }: {
    display: Display;
    dashboards: Dashboard[];
    /** Text sizes in percent, with their label. */
    textSizes: [number, string][];
    /** Screensaver restart intervals in minutes, with their label. */
    screensaverChoices: [number, string][];
    /** Screen care settings and dimming modes, with their label. */
    screenCareChoices: [ScreenCare, string][];
    dimChoices: [DimAtNight, string][];
    /** The TV grid, to draw the dashboard; null until it is known. */
    grid: { columns: number; rows: number } | null;
    /** The name of the display's Cast device; empty when it was not found. */
    tvName: string;
    ondashboard: (id: string) => void;
    ontextscale: (percent: number) => void;
    onscreensaver: (minutes: number) => void;
    onscreencare: (changes: ScreenCareChanges) => void;
    onschedule: () => void;
    /** Approves the code a screen shows; rejects when it is wrong. */
    onpair: (code: string) => Promise<void>;
    onunpair: () => void;
    /** Ends a wait for the TV to be switched off; see describeCast. */
    ontakeover: () => void;
    onrename: () => void;
    onremove: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  const shown = $derived(dashboards.find((b) => b.id === (d.dashboardId || DEFAULT_DASHBOARD_ID)));
  const isCast = $derived(d.castDeviceId !== '');
  const screens = $derived(d.pairedScreens ?? 0);
  const tv = $derived(tvName || 'the TV');
  const address = $derived(`${location.origin}/d/${d.id}`);
  // A display without Cast and without a screen shows how to connect one
  // straight away; otherwise connecting another screen is a step away.
  const needsScreen = $derived(!isCast && screens === 0);
  let connecting = $state(false);
  const reports = $derived(d.screenReports ?? []);
  const screensaver = $derived(isCast ? describeScreensaver(d.cast) : []);

  function reportedAgo(receivedAt: string): string {
    return describeAge(Math.max(0, Math.round((Date.now() - Date.parse(receivedAt)) / 60_000)));
  }

  async function pair(code: string) {
    await onpair(code);
    connecting = false;
  }
</script>

<article class="display">
  <!-- The display drawn as what it is: a screen showing its dashboard. The
       light is on only when GDash knows the dashboard is on the TV. -->
  <div class="set" aria-hidden="true">
    <div class="bezel">
      {#if grid && shown}
        <DashboardThumb widgets={shown.widgets} theme={shown.theme ?? 'minimal-dark'} font={shown.font} columns={grid.columns} rows={grid.rows} labels={false} />
      {:else}
        <div class="blank"></div>
      {/if}
      <span class="led" class:on={d.cast?.state === 'showing'}></span>
    </div>
    <span class="stand"></span>
  </div>

  <div class="body">
    <header class="head">
      <h3>{d.name}</h3>
      <a class="open" href="/d/{d.id}" target="_blank" rel="noopener" aria-label="Preview {d.name} in a new tab">
        Preview <Icon name="external" size={16} />
      </a>
    </header>

    <!-- What the screen is doing, first and in plain words. -->
    {#if isCast}
      <p class="badge state {castTone(d.cast)}" title={d.cast?.problem ?? ''}>{describeCast(d.cast, tv, d.activeFrom)}</p>
    {:else if screens > 0}
      <p class="badge state">{describeScreens(d)}</p>
    {:else}
      <p class="badge state warn">No screen yet</p>
    {/if}
    <p class="muted meta">
      {describeHours(d)}{#if isCast && screens > 0}{' · '}{describeScreens(d)}{/if}
    </p>
    {#each screensaver as line (line)}<p class="muted meta">{line}</p>{/each}

    <div class="fields">
      <label class="shows">
        Shows
        <!-- The accessible name keeps the display's name, so several
             displays' lists stay apart for a screen reader. -->
        <select
          aria-label="Dashboard for {d.name}"
          value={d.dashboardId || DEFAULT_DASHBOARD_ID}
          onchange={(e) => ondashboard((e.target as HTMLSelectElement).value)}>
          {#each dashboards as b (b.id)}
            <option value={b.id}>{b.name}</option>
          {/each}
        </select>
      </label>
      <label>
        Text size
        <select
          aria-label="Text size for {d.name}"
          value={d.textScale || 100}
          onchange={(e) => ontextscale(Number((e.target as HTMLSelectElement).value))}>
          {#each textSizes as [value, label] (value)}
            <option {value}>{label}</option>
          {/each}
        </select>
      </label>
      {#if isCast}
        <!-- Google TV covers a running cast with its screensaver; restarting
             the receiver, a few seconds of reloading, keeps it away. -->
        <label>
          Keep in front of the screensaver
          <select
            aria-label="Keep {d.name} in front of the screensaver"
            value={d.screensaverMinutes}
            onchange={(e) => onscreensaver(Number((e.target as HTMLSelectElement).value))}>
            {#each screensaverChoices as [value, label] (value)}
              <option {value}>{label}</option>
            {/each}
          </select>
        </label>
      {/if}
      <!-- OLED care and dimming work on any theme, on this screen only. -->
      <label>
        Screen care
        <select
          aria-label="Screen care for {d.name}"
          aria-describedby="care-{d.id}"
          value={d.screenCare}
          onchange={(e) => onscreencare({ screenCare: (e.target as HTMLSelectElement).value as ScreenCare })}>
          {#each screenCareChoices as [value, label] (value)}
            <option {value}>{label}</option>
          {/each}
        </select>
      </label>
      <label>
        Dim at night
        <select
          aria-label="Dim {d.name} at night"
          aria-describedby="care-{d.id}"
          value={d.dimAtNight}
          onchange={(e) => onscreencare({ dimAtNight: (e.target as HTMLSelectElement).value as DimAtNight })}>
          {#each dimChoices as [value, label] (value)}
            <option {value}>{label}</option>
          {/each}
        </select>
      </label>
      {#if d.dimAtNight === 'hours'}
        <div class="dim-hours">
          <label>
            Dim from
            <input type="time" aria-label="Dim {d.name} from" value={d.dimFrom}
              onchange={(e) => onscreencare({ dimFrom: (e.target as HTMLInputElement).value })} />
          </label>
          <label>
            until
            <input type="time" aria-label="Dim {d.name} until" value={d.dimUntil}
              onchange={(e) => onscreencare({ dimUntil: (e.target as HTMLInputElement).value })} />
          </label>
        </div>
      {/if}
    </div>
    <p id="care-{d.id}" class="muted small care-hint">
      {#if d.screenCare === 'oled'}OLED care: the dashboard drifts slowly, borders and titles are dimmed and the screen is black for a second every hour.{/if}
      {#if d.dimAtNight === 'sunset'}Dims to 60 % at the same moment as a dashboard’s style after sunset; it needs a weather location.{:else if d.dimAtNight === 'hours'}Dims to 60 % between these hours.{/if}
      {#if d.screenCare === 'off' && d.dimAtNight === 'off'}For an OLED TV: care against burn-in, and less light at night.{/if}
    </p>

    {#if needsScreen || connecting}
      <PairScreen {address} onpair={pair} {onerror} oncancel={needsScreen ? undefined : () => (connecting = false)}>
        {#if needsScreen}
          No screen shows {d.name} yet. Connect the TV or tablet it is meant for:
        {:else if isCast}
          GDash keeps this display on {tv} by itself. To show it on a tablet or computer as well:
        {:else}
          To show {d.name} on one more screen:
        {/if}
      </PairScreen>
    {/if}

    <div class="actions">
      {#if d.cast?.state === 'yielded'}
        <button onclick={ontakeover} aria-label="Show dashboard now on {d.name}">Show dashboard now</button>
      {/if}
      <button class="secondary" onclick={onschedule}>Schedule</button>
      {#if !needsScreen && !connecting}
        <button class="link" onclick={() => (connecting = true)}>
          {isCast ? 'Open on another screen…' : 'Add another screen…'}
        </button>
      {/if}
      {#if screens > 0}
        <button class="link" onclick={onunpair}>{isCast ? 'Disconnect the other screens' : 'Disconnect screens'}</button>
      {/if}
      <button class="link" onclick={onrename}>Rename</button>
      <button class="link danger remove" onclick={onremove}>Remove</button>
    </div>

    <!-- What the screens found out about their own browsers: how to tell
         why the TV looks different from the preview. -->
    {#if reports.length > 0}
      <details class="reports">
        <summary>What the screen reports</summary>
        {#each reports as report, i (i)}
          {@const warnings = screenWarnings(report)}
          <section class="report" aria-label="Report of {report.screen === 'cast' ? tv : 'a paired screen'}">
            <p class="report-head">
              <strong>{describeBrowser(report.userAgent)}</strong>
              <span class="muted">on {report.screen === 'cast' ? tv : 'a paired screen'}, {reportedAgo(report.receivedAt)}</span>
            </p>
            <p class="small">{describeScreenSize(report)}</p>
            {#if report.visibility.length > 0}<p class="small">Visibility: {describeVisibility(report)}</p>{/if}
            {#if warnings.length > 0}
              <ul class="small warnings">
                {#each warnings as warning (warning)}<li>{warning}</li>{/each}
              </ul>
            {:else}
              <p class="small">Everything the dashboard uses works on this screen.</p>
            {/if}
            {#if report.errors.length > 0}
              <ul class="small errors">
                {#each report.errors as error, j (j)}<li><code>{error.message}</code>{#if error.source}{' '}<span class="muted">{error.source}</span>{/if}</li>{/each}
              </ul>
            {/if}
            <p class="small muted agent">{report.userAgent}</p>
          </section>
        {/each}
      </details>
    {/if}
  </div>
</article>

<style>
  .display {
    /* The list row is a flex line; the item takes all of it. */
    flex: 1 1 100%;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--s-4) var(--s-6);
    min-width: 0;
    padding: var(--s-3) 0;
  }
  @container page (min-width: 700px) {
    .display {
      grid-template-columns: 240px minmax(0, 1fr);
    }
  }
  .set {
    display: grid;
    justify-items: center;
    align-self: start;
    max-width: 280px;
  }
  .bezel {
    position: relative;
    width: 100%;
    padding: 6px 6px 14px;
    border-radius: var(--r-lg);
    background: var(--device);
    box-shadow: inset 0 0 0 1px var(--device-edge);
  }
  .bezel :global(.thumbnail) {
    border: 0;
    border-radius: 3px;
  }
  .blank {
    aspect-ratio: 16 / 9;
    border-radius: 3px;
    background: var(--device-screen);
  }
  .led {
    position: absolute;
    right: 12px;
    bottom: 5px;
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: var(--device-led-off);
  }
  .led.on {
    background: var(--spark);
    box-shadow: 0 0 6px 1px color-mix(in srgb, var(--spark) 60%, transparent);
  }
  .stand {
    width: 28%;
    height: 6px;
    margin-top: -1px;
    border-radius: 0 0 3px 3px;
    background: var(--device-stand);
  }
  .body {
    display: grid;
    gap: var(--s-3);
    align-content: start;
    min-width: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--s-2) var(--s-4);
  }
  h3 {
    font-size: var(--fs-xl);
    font-weight: 650;
    letter-spacing: -0.03em;
    overflow-wrap: anywhere;
  }
  .open {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-weight: 550;
    text-decoration-color: transparent;
    text-underline-offset: 4px;
  }
  .open:hover {
    text-decoration-color: currentColor;
  }
  .state {
    margin: calc(-1 * var(--s-2)) 0 0;
    font-size: var(--fs-md);
    width: auto;
    align-items: baseline;
  }
  /* The dot sits on the first line of a status that wraps. */
  .state::before {
    transform: translateY(-1px);
  }
  .meta {
    margin: calc(-1 * var(--s-2)) 0 0;
    font-size: var(--fs-sm);
  }
  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 200px), 1fr));
    gap: var(--s-3) var(--s-4);
  }
  /* What the display shows is the choice that matters most. */
  @container page (min-width: 700px) {
    .fields {
      grid-template-columns: minmax(0, 2fr) minmax(0, 1fr);
    }
  }
  .dim-hours {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-3);
  }
  .dim-hours input {
    width: auto;
  }
  .care-hint {
    margin: calc(-1 * var(--s-1)) 0 0;
  }
  .shows {
    font-weight: 650;
    color: var(--text);
  }
  .shows select {
    font-weight: 550;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2) var(--s-4);
  }
  .reports summary {
    cursor: pointer;
    color: var(--muted);
    font-size: var(--fs-sm);
    min-height: 32px;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  /* A flex summary loses the browser's triangle, so it draws its own. */
  .reports summary::before {
    content: '▸';
  }
  .reports[open] > summary::before {
    content: '▾';
  }
  .report {
    display: grid;
    gap: var(--s-1);
    padding: var(--s-2) 0;
  }
  .report + .report {
    border-top: 1px solid var(--border);
  }
  .report p {
    margin: 0;
  }
  .report-head {
    display: flex;
    flex-wrap: wrap;
    gap: 0 var(--s-2);
    align-items: baseline;
  }
  .warnings,
  .errors {
    margin: 0;
    padding-left: 1.2em;
    display: grid;
    gap: var(--s-1);
  }
  /* Plain bullets, not the rows of the display list around them. */
  .warnings li,
  .errors li {
    display: list-item;
    padding: 0;
    border-top: 0;
  }
  .errors code {
    overflow-wrap: anywhere;
  }
  .agent {
    font-family: var(--mono);
    overflow-wrap: anywhere;
  }
  /* Removing is rare and cannot be undone: apart from the rest. */
  .remove {
    margin-left: auto;
  }
</style>
