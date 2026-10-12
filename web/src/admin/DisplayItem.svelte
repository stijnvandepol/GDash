<script lang="ts">
  import type { Dashboard, Display } from './api';
  import { castTone, describeCast } from './status';
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
    grid,
    tvName,
    ondashboard,
    ontextscale,
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
    /** The TV grid, to draw the dashboard; null until it is known. */
    grid: { columns: number; rows: number } | null;
    /** The name of the display's Cast device; empty when it was not found. */
    tvName: string;
    ondashboard: (id: string) => void;
    ontextscale: (percent: number) => void;
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
    </div>

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
  /* Removing is rare and cannot be undone: apart from the rest. */
  .remove {
    margin-left: auto;
  }
</style>
