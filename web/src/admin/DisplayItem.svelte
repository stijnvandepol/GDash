<script lang="ts">
  import type { Dashboard, Display } from './api';
  import { describeCast } from './status';
  import Icon from './Icon.svelte';
  import DashboardThumb from './DashboardThumb.svelte';

  // One display in the list: what it is, what it shows, and what can be
  // done with it. Editing forms (rename, schedule, pairing) stay in
  // Displays.svelte, which swaps this item for them.
  let {
    display: d,
    dashboards,
    textSizes,
    grid,
    meta,
    ondashboard,
    ontextscale,
    onschedule,
    onpair,
    onunpair,
    ontakeover,
    onrename,
    onremove,
  }: {
    display: Display;
    dashboards: Dashboard[];
    /** Text sizes in percent, with their label. */
    textSizes: [number, string][];
    /** The TV grid, to draw the dashboard; null until it is known. */
    grid: { columns: number; rows: number } | null;
    /** One line about the device, paired screens and hours. */
    meta: string;
    ondashboard: (id: string) => void;
    ontextscale: (percent: number) => void;
    onschedule: () => void;
    onpair: () => void;
    onunpair: () => void;
    /** Ends a wait for the TV to be switched off; see describeCast. */
    ontakeover: () => void;
    onrename: () => void;
    onremove: () => void;
  } = $props();

  const shown = $derived(dashboards.find((b) => b.id === (d.dashboardId || 'default')));
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
    <div class="title">
      <h3>{d.name}</h3>
      {#if d.cast}
        <span
          class="badge"
          class:ok={d.cast.state === 'showing'}
          class:warn={d.cast.state === 'other_app' || d.cast.state === 'yielded'}
          class:bad={d.cast.state === 'unreachable'}
          title={d.cast.problem ?? ''}
        >
          {describeCast(d.cast)}
        </span>
      {/if}
    </div>
    <a class="open" href="/d/{d.id}" target="_blank" rel="noopener">
      Open screen <Icon name="external" size={16} />
    </a>
  </header>
  <p class="muted meta">{meta}</p>

  <div class="fields">
    <label>
      Dashboard
      <!-- The accessible name keeps the visible label and adds the display,
           so several displays' lists stay apart for a screen reader. -->
      <select
        aria-label="Dashboard for {d.name}"
        value={d.dashboardId || 'default'}
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

  <div class="actions">
    {#if d.cast?.state === 'yielded'}
      <button onclick={ontakeover} aria-label="Show dashboard now on {d.name}">Show dashboard now</button>
    {/if}
    <button class="secondary" onclick={onschedule}>Schedule</button>
    <button class="secondary" onclick={onpair}>Pair a screen</button>
    {#if (d.pairedScreens ?? 0) > 0}
      <button class="secondary" onclick={onunpair}>Unpair screens</button>
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
  .title {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--s-2) var(--s-4);
    min-width: 0;
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
  .meta {
    margin: calc(-1 * var(--s-2)) 0 0;
    font-size: var(--fs-sm);
  }
  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 200px), 1fr));
    gap: var(--s-3) var(--s-4);
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
