<script lang="ts">
  import type { Entry } from '../protocol';
  import { t } from '../i18n.svelte';
  import { display } from '../state.svelte';
  import { LOAD_TIMEOUT_MS, frameScale, pageFrame } from './webpage';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const frame = $derived(display.framesAllowed ? pageFrame(entry, settings, location.origin) : null);
  // Primitives, so a new snapshot with the same page does not reload it.
  const src = $derived(frame?.src ?? '');
  const sandbox = $derived(frame?.sandbox ?? '');
  const reloadMs = $derived(frame?.reloadMs ?? 0);
  const scale = $derived(frameScale(frame?.zoom ?? 100));

  let reloads = $state(0);
  let loaded = $state(false);
  let timedOut = $state(false);

  $effect(() => {
    if (reloadMs <= 0) return;
    const timer = setInterval(() => reloads++, reloadMs);
    return () => clearInterval(timer);
  });

  // A frame tells when it loaded, not whether the page in it works: a page
  // that never loads shows the placeholder, one that forbids frames cannot
  // be told apart (Integrations has a Check for that).
  $effect(() => {
    if (!src) return;
    void reloads;
    loaded = false;
    timedOut = false;
    const timer = setTimeout(() => (timedOut = !loaded), LOAD_TIMEOUT_MS);
    return () => clearTimeout(timer);
  });

  function onload() {
    loaded = true;
    timedOut = false;
  }
</script>

<div class="webpage">
  {#if src}
    {#key `${src} ${sandbox} ${reloads}`}
      <iframe
        title={frame?.name ?? ''}
        {src}
        {sandbox}
        allow=""
        referrerpolicy="no-referrer"
        loading="eager"
        tabindex="-1"
        class:hidden={timedOut}
        style:width={scale.size}
        style:height={scale.size}
        style:transform="scale({scale.scale})"
        {onload}
      ></iframe>
    {/key}
  {/if}
  {#if !src || timedOut}
    <p class="unavailable">{t('webPageUnavailable')}</p>
  {/if}
</div>

<style>
  /* The page fills the whole tile, up to its rounded edge. */
  .webpage {
    position: relative;
    flex: 1 1 auto;
    margin: calc(-1 * var(--space-widget));
    overflow: hidden;
  }
  /* Under a title the page starts below it, and fills the rest. */
  :global(.titled) .webpage {
    margin-top: 0;
  }
  iframe {
    position: absolute;
    top: 0;
    left: 0;
    border: 0;
    transform-origin: 0 0;
    /* Most pages assume a white page behind them, as in a browser tab. */
    background: #fff;
    /* The TV shows the page; nobody clicks it. */
    pointer-events: none;
  }
  .hidden {
    visibility: hidden;
  }
  .unavailable {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    margin: 0;
    padding: var(--space-widget);
    text-align: center;
    color: var(--text-muted);
    font-size: 38px;
    font-size: min(38px, 12cqh);
  }
</style>
