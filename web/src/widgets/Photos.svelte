<script lang="ts">
  import type { Entry } from '../protocol';
  import { t } from '../i18n.svelte';
  import { msToNextPage } from '../pages';
  import { photoSource, photoUrls } from './photos';

  const DEFAULT_SECONDS = 30;

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const source = $derived(photoSource(settings));
  const urls = $derived(photoUrls(entry?.value, source));
  const seconds = $derived(typeof settings?.seconds === 'number' && settings.seconds > 0 ? settings.seconds : DEFAULT_SECONDS);

  // Photos change on the wall clock, like pages, so every screen of a
  // display shows the same photo; one timer to the next change.
  let now = $state(Date.now());
  $effect(() => {
    if (urls.length < 2) return;
    const every = seconds;
    now = Date.now();
    let timer = setTimeout(function next() {
      now = Date.now();
      timer = setTimeout(next, msToNextPage(now, every));
    }, msToNextPage(now, every));
    return () => clearTimeout(timer);
  });
  const current = $derived(urls.length > 0 ? urls[Math.floor(now / 1000 / seconds) % urls.length] : undefined);

  // An album that is not set up has no entry; one being read has no value yet.
  const waiting = $derived.by(() => {
    if (source === 'uploaded') return t('photosNone');
    if (entry === undefined) return t('albumNotSetUp');
    if (entry.value === undefined) return t(entry.failing ? 'albumFailed' : 'albumLoading');
    return t('albumNone');
  });
</script>

{#if current}
  {#key current}
    <img src={current} alt="" />
  {/key}
{:else}
  <span class="muted">{waiting}</span>
{/if}

<style>
  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: calc(var(--radius-widget) / 2);
  }
  .muted {
    color: var(--text-muted);
    font-size: 38px;
    font-size: min(38px, 12cqh);
  }
</style>
