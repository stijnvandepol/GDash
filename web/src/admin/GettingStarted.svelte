<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from './api';
  import Icon from './Icon.svelte';

  let { onopen }: { onopen: (tab: 'displays' | 'dashboards' | 'integrations') => void } = $props();

  // How often the steps are checked again while the card shows, so it
  // disappears soon after the last step is done on another tab.
  const RECHECK_MS = 10_000;
  const DEFAULT_DASHBOARD_ID = 'default';

  let hasLocation = $state(true);
  let hasSources = $state(true);
  let hasDashboard = $state(true);
  let hasDisplay = $state(true);

  const done = $derived(hasLocation && hasDisplay);
  const steps = $derived([hasLocation, hasSources, hasDashboard, hasDisplay]);
  const finished = $derived(steps.filter(Boolean).length);

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
  <section class="card wide object getting-started" aria-labelledby="getting-started-title">
    <div class="card-head">
      <h2 id="getting-started-title">Getting started</h2>
      <span class="progress num">{finished} of {steps.length} done</span>
    </div>
    <ol>
      <li class:done={hasLocation}>
        <span class="marker" aria-hidden="true">{#if hasLocation}<Icon name="check" size={14} />{:else}1{/if}</span>
        <span class="step">Choose the location for the weather.</span>
        {#if !hasLocation}<button class="link" onclick={() => onopen('integrations')}>Open Integrations</button>{/if}
      </li>
      <li class:done={hasSources}>
        <span class="marker" aria-hidden="true">{#if hasSources}<Icon name="check" size={14} />{:else}2{/if}</span>
        <span class="step">Add news feeds or calendars, if you want them.</span>
        {#if !hasSources}<button class="link" onclick={() => onopen('integrations')}>Open Integrations</button>{/if}
      </li>
      <li class:done={hasDashboard}>
        <span class="marker" aria-hidden="true">{#if hasDashboard}<Icon name="check" size={14} />{:else}3{/if}</span>
        <span class="step">Start a dashboard from a template, such as Kitchen or Hallway, or keep the default.</span>
        {#if !hasDashboard}<button class="link" onclick={() => onopen('dashboards')}>Open Dashboards</button>{/if}
      </li>
      <li class:done={hasDisplay}>
        <span class="marker" aria-hidden="true">{#if hasDisplay}<Icon name="check" size={14} />{:else}4{/if}</span>
        <span class="step">Add a display and pick your TV, or connect a browser with the code it shows.</span>
        {#if !hasDisplay}<button class="link" onclick={() => onopen('displays')}>Open Displays</button>{/if}
      </li>
    </ol>
  </section>
{/if}

<style>
  /* A strip above the page until the four steps are done; each step has a
     light that comes on when it is. */
  .getting-started {
    margin-bottom: var(--s-6);
    padding: var(--s-5);
    border-top: 1px solid var(--border);
  }
  .progress {
    color: var(--muted);
    font-size: var(--fs-sm);
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr));
    gap: var(--s-4) var(--s-5);
    counter-reset: step;
  }
  li {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-content: start;
    gap: var(--s-1) var(--s-3);
  }
  li > :global(button) {
    grid-column: 2;
    justify-self: start;
  }
  .marker {
    display: grid;
    place-items: center;
    flex: none;
    width: 22px;
    height: 22px;
    border: 1.5px solid var(--border-strong);
    border-radius: 50%;
    color: var(--muted);
    font-family: var(--mono);
    font-size: var(--fs-xs);
    font-weight: 600;
  }
  .step {
    font-size: var(--fs-sm);
    padding-top: 1px;
  }
  li.done .marker {
    border-color: var(--spark);
    background: var(--spark);
    color: var(--device-screen);
  }
  li.done .step {
    color: var(--muted);
  }
</style>
