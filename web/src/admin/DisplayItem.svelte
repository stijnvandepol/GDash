<script lang="ts">
  import type { Dashboard, Display } from './api';
  import { describeCast } from './status';
  import Icon from './Icon.svelte';

  // One display in the list: what it is, what it shows, and what can be
  // done with it. Editing forms (rename, schedule, pairing) stay in
  // Displays.svelte, which swaps this item for them.
  let {
    display: d,
    dashboards,
    textSizes,
    meta,
    ondashboard,
    ontextscale,
    onschedule,
    onpair,
    onunpair,
    onrename,
    onremove,
  }: {
    display: Display;
    dashboards: Dashboard[];
    /** Text sizes in percent, with their label. */
    textSizes: [number, string][];
    /** One line about the device, paired screens and hours. */
    meta: string;
    ondashboard: (id: string) => void;
    ontextscale: (percent: number) => void;
    onschedule: () => void;
    onpair: () => void;
    onunpair: () => void;
    onrename: () => void;
    onremove: () => void;
  } = $props();
</script>

<article class="display">
  <header class="head">
    <div class="title">
      <h3>{d.name}</h3>
      {#if d.cast}
        <span
          class="badge"
          class:ok={d.cast.state === 'showing'}
          class:warn={d.cast.state === 'other_app'}
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
      <select value={d.dashboardId || 'default'} onchange={(e) => ondashboard((e.target as HTMLSelectElement).value)}>
        {#each dashboards as b (b.id)}
          <option value={b.id}>{b.name}</option>
        {/each}
      </select>
    </label>
    <label>
      Text size
      <select value={d.textScale || 100} onchange={(e) => ontextscale(Number((e.target as HTMLSelectElement).value))}>
        {#each textSizes as [value, label] (value)}
          <option {value}>{label}</option>
        {/each}
      </select>
    </label>
  </div>

  <div class="actions">
    <button class="secondary" onclick={onschedule}>Schedule</button>
    <button class="secondary" onclick={onpair}>Pair a screen</button>
    {#if (d.pairedScreens ?? 0) > 0}
      <button class="secondary" onclick={onunpair}>Unpair screens</button>
    {/if}
    <button class="link" onclick={onrename}>Rename</button>
    <button class="link danger remove" onclick={onremove}>Remove</button>
  </div>
</article>

<style>
  .display {
    /* The list row is a flex line; the item takes all of it. */
    flex: 1 1 100%;
    display: grid;
    gap: var(--s-3);
    min-width: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2) var(--s-4);
  }
  .title {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2) var(--s-3);
    min-width: 0;
  }
  h3 {
    font-size: var(--fs-lg);
    overflow-wrap: anywhere;
  }
  .open {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-weight: 600;
    text-decoration: none;
  }
  .open:hover {
    text-decoration: underline;
  }
  .meta {
    margin: calc(-1 * var(--s-2)) 0 0;
  }
  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr));
    gap: var(--s-3) var(--s-4);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
  }
  /* Removing is rare and cannot be undone: apart from the rest. */
  .remove {
    margin-left: auto;
  }
</style>
