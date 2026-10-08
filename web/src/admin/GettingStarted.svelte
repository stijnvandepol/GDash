<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from './api';

  let { onopen }: { onopen: (tab: 'displays' | 'settings') => void } = $props();

  // How often the steps are checked again while the card shows, so it
  // disappears soon after the last step is done on another tab.
  const RECHECK_MS = 10_000;

  let hasLocation = $state(true);
  let hasSources = $state(true);
  let hasDisplay = $state(true);

  const done = $derived(hasLocation && hasDisplay);

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
      const [{ settings }, displays] = await Promise.all([api.settings(), api.displays()]);
      hasLocation = settings.location !== null;
      hasSources = settings.newsFeeds.length > 0 || settings.calendars.length > 0;
      hasDisplay = displays.length > 0;
    } catch {
      hasLocation = hasDisplay = true;
    }
  }
</script>

{#if !done}
  <section class="card getting-started">
    <h2>Getting started</h2>
    <ol>
      <li class:done={hasLocation}>
        Choose the location for the weather.
        {#if !hasLocation}<button class="link" onclick={() => onopen('settings')}>Open Settings</button>{/if}
      </li>
      <li class:done={hasSources}>
        Add news feeds or calendars, if you want them.
        {#if !hasSources}<button class="link" onclick={() => onopen('settings')}>Open Settings</button>{/if}
      </li>
      <li class:done={hasDisplay}>
        Add a display and pick your TV, or pair a browser with the code it shows.
        {#if !hasDisplay}<button class="link" onclick={() => onopen('displays')}>Open Displays</button>{/if}
      </li>
    </ol>
  </section>
{/if}

<style>
  ol {
    margin: 0;
    padding-left: 1.4em;
    display: grid;
    gap: 6px;
  }
  li.done {
    color: var(--muted);
    text-decoration: line-through;
  }
</style>
