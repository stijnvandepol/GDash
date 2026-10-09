<script lang="ts">
  import type { Entry } from '../protocol';
  import { t } from '../i18n.svelte';
  import { msToNextPage } from '../pages';

  const DEFAULT_SECONDS = 30;
  // The names the server gives photos; anything else in a snapshot is
  // ignored rather than put in a URL.
  const PHOTO_NAME = /^[0-9a-f]{32}\.(jpg|png|webp)$/;

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const names = $derived(
    Array.isArray(entry?.value) ? entry.value.filter((n): n is string => typeof n === 'string' && PHOTO_NAME.test(n)) : [],
  );
  const seconds = $derived(typeof settings?.seconds === 'number' && settings.seconds > 0 ? settings.seconds : DEFAULT_SECONDS);

  // Photos change on the wall clock, like pages, so every screen of a
  // display shows the same photo; one timer to the next change.
  let now = $state(Date.now());
  $effect(() => {
    if (names.length < 2) return;
    const every = seconds;
    now = Date.now();
    let timer = setTimeout(function next() {
      now = Date.now();
      timer = setTimeout(next, msToNextPage(now, every));
    }, msToNextPage(now, every));
    return () => clearTimeout(timer);
  });
  const current = $derived(names.length > 0 ? names[Math.floor(now / 1000 / seconds) % names.length] : undefined);
</script>

{#if current}
  {#key current}
    <img src="/api/photos/{current}" alt="" />
  {/key}
{:else}
  <span class="muted">{t('photosNone')}</span>
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
