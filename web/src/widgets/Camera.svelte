<script lang="ts">
  import { locale, t } from '../i18n.svelte';
  import { cameraId, readShot, refreshSeconds, shotTime } from './camera';

  let { settings }: { settings: Record<string, unknown> | undefined } = $props();

  const id = $derived(cameraId(settings));
  const seconds = $derived(refreshSeconds(settings));
  const fit = $derived(settings?.fit === 'cover' ? 'cover' : 'contain');

  // The image is fetched rather than put in an img element, so the widget
  // can read when it arrived and whether the camera failed since.
  let src = $state<string | undefined>();
  let takenAt = $state<Date | null>(null);
  let stale = $state(false);
  let problem = $state<'missing' | 'failed' | undefined>();
  // The image's width over its height, so a whole image gets a frame of its
  // own shape and the stale time sits on its corner, not beside it.
  let ratio = $state(16 / 9);

  $effect(() => {
    const camera = id;
    const every = seconds * 1000;
    if (camera === undefined) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let shown: string | undefined;

    function show(url: string | undefined) {
      if (shown) URL.revokeObjectURL(shown);
      shown = url;
      src = url;
    }

    async function load() {
      try {
        const response = await fetch(`/api/cameras/${camera}/image`, { cache: 'no-store', credentials: 'same-origin' });
        if (stopped) return;
        if (response.status === 404) {
          show(undefined);
          problem = 'missing';
        } else if (!response.ok) {
          // The server has no image yet; a kept one stays, marked stale.
          stale = shown !== undefined;
          problem = shown ? undefined : 'failed';
        } else {
          const blob = await response.blob();
          if (stopped) return;
          ({ takenAt, stale } = readShot(response.headers));
          show(URL.createObjectURL(blob));
          problem = undefined;
        }
      } catch {
        // No connection to the server: keep the last image, marked stale.
        if (!stopped) {
          stale = shown !== undefined;
          problem = shown ? undefined : 'failed';
        }
      } finally {
        if (!stopped) timer = setTimeout(load, every);
      }
    }

    void load();
    return () => {
      stopped = true;
      clearTimeout(timer);
      show(undefined);
      problem = undefined;
      stale = false;
    };
  });
</script>

{#if id === undefined || problem === 'missing'}
  <span class="muted">{t('cameraMissing')}</span>
{:else if src}
  <div class="frame" class:whole={fit === 'contain'} style:--ratio={ratio}>
    <img
      {src}
      alt=""
      onload={(e) => {
        const img = e.currentTarget as HTMLImageElement;
        if (img.naturalHeight > 0) ratio = img.naturalWidth / img.naturalHeight;
      }}
    />
    {#if stale && takenAt}
      <span class="stale">{shotTime(takenAt, locale())}</span>
    {/if}
  </div>
{:else}
  <span class="muted">{t(problem === 'failed' ? 'cameraFailed' : 'cameraLoading')}</span>
{/if}

<style>
  .frame {
    position: relative;
    width: 100%;
    height: 100%;
  }
  /* The whole image: the largest frame of its shape that fits the tile,
     centred, measured in the tile's container units. */
  .frame.whole {
    width: min(100cqw, 100cqh * var(--ratio));
    height: min(100cqh, 100cqw / var(--ratio));
    margin: auto;
  }
  img {
    width: 100%;
    height: 100%;
    display: block;
    object-fit: cover;
    border-radius: calc(var(--radius-widget) / 2);
  }
  /* The time of the last image that arrived, in the warning colour, on a
     plate of the tile's own colour: every theme keeps its warning readable
     there, whatever the picture behind it. */
  .stale {
    position: absolute;
    right: 3cqmin;
    bottom: 3cqmin;
    padding: 0.15em 0.5em;
    border-radius: 0.4em;
    /* The tile's colour as it shows on the backdrop, also for a theme with
       translucent tiles. */
    background: linear-gradient(var(--surface-1), var(--surface-1)), var(--surface-0);
    color: var(--warning);
    font-size: min(34px, 9cqh);
    font-weight: 650;
    font-variant-numeric: tabular-nums;
  }
  .muted {
    color: var(--text-muted);
    font-size: 38px;
    font-size: min(38px, 12cqh);
  }
</style>
