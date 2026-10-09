<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from './api';
  import Icon from './Icon.svelte';

  let { onopen }: { onopen: (tab: 'displays' | 'dashboards' | 'settings') => void } = $props();

  // How often the steps are checked again while the card shows, so it
  // disappears soon after the last step is done on another tab.
  const RECHECK_MS = 10_000;
  const DEFAULT_DASHBOARD_ID = 'default';

  let hasLocation = $state(true);
  let hasSources = $state(true);
  let hasDashboard = $state(true);
  let hasDisplay = $state(true);

  const done = $derived(hasLocation && hasDisplay);
  const finished = $derived([hasLocation, hasSources, hasDisplay].filter(Boolean).length);

  onMount(() => {
    void check();
    const timer = setInterval(() => {
      if (!done) void check();
    }, RECHECK_MS);
    return () => clearInterval(timer);
  });

  // A failed check leaves the card hidden: it is a hint, and the page
  // itself reports errors when it loads.
  async function check() {
    try {
      const [{ settings }, displays, dashboards] = await Promise.all([api.settings(), api.displays(), api.dashboards()]);
      hasLocation = settings.location !== null;
      hasSources = settings.newsFeeds.length > 0 || settings.calendars.length > 0;
      // The default dashboard always exists, so only another one counts.
      hasDashboard = dashboards.some((d) => d.id !== DEFAULT_DASHBOARD_ID);
      hasDisplay = displays.length > 0;
    } catch {
      hasLocation = hasDisplay = hasDashboard = true;
    }
  }
</script>

{#if !done}
  <section class="card getting-started" aria-labelledby="getting-started-title">
    <div class="card-head">
      <h2 id="getting-started-title">Getting started</h2>
      <span class="badge info">{finished} of 3 done</span>
    </div>
    <ol>
      <li class:done={hasLocation}>
        <span class="marker" aria-hidden="true">{#if hasLocation}<Icon name="check" size={14} />{:else}1{/if}</span>
        <span class="step">Choose the location for the weather.</span>
        {#if !hasLocation}<button class="link" onclick={() => onopen('settings')}>Open Settings</button>{/if}
      </li>
      <li class:done={hasSources}>
        <span class="marker" aria-hidden="true">{#if hasSources}<Icon name="check" size={14} />{:else}2{/if}</span>
        <span class="step">Add news feeds or calendars, if you want them.</span>
        {#if !hasSources}<button class="link" onclick={() => onopen('settings')}>Open Settings</button>{/if}
      </li>
      <li class:done={hasDashboard}>
        Start a dashboard from a template, such as Kitchen or Hallway, or keep the default.
        {#if !hasDashboard}<button class="link" onclick={() => onopen('dashboards')}>Open Dashboards</button>{/if}
      </li>
      <li class:done={hasDisplay}>
        <span class="marker" aria-hidden="true">{#if hasDisplay}<Icon name="check" size={14} />{:else}3{/if}</span>
        <span class="step">Add a display and pick your TV, or pair a browser with the code it shows.</span>
        {#if !hasDisplay}<button class="link" onclick={() => onopen('displays')}>Open Displays</button>{/if}
      </li>
    </ol>
  </section>
{/if}

<style>
  .getting-started {
    background: var(--accent-soft);
    border-color: transparent;
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: var(--s-3);
  }
  li {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-1) var(--s-3);
  }
  .marker {
    display: grid;
    place-items: center;
    flex: none;
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: var(--surface);
    color: var(--accent);
    font-size: var(--fs-xs);
    font-weight: 700;
  }
  .step {
    flex: 1 1 14em;
    min-width: 0;
  }
  li.done .marker {
    background: var(--success);
    color: var(--surface);
  }
  li.done .step {
    color: var(--muted);
  }
</style>
