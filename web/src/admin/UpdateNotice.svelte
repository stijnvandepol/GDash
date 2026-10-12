<script lang="ts">
  import type { Release } from './api';
  import Icon from './Icon.svelte';

  // A newer release, said once and calmly: what it is, what changed and how
  // to install it. The steps differ per installation, so they stay in the
  // documentation.
  let { release }: { release: Release } = $props();

  const HOW_TO_UPDATE = 'https://github.com/stijnvandepol/gdash/blob/main/docs/install.md#updating';
</script>

<p class="update-notice" role="status">
  <span class="badge info">GDash {release.version} is available</span>
  <span class="links">
    <a href={release.url} target="_blank" rel="noopener">What's new <Icon name="external" size={14} /></a>
    <a href={HOW_TO_UPDATE} target="_blank" rel="noopener">How to update <Icon name="external" size={14} /></a>
  </span>
</p>

<style>
  .update-notice {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2) var(--s-5);
    margin: 0;
    padding: var(--s-3) var(--s-4);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    background: var(--surface);
  }
  .badge {
    color: var(--text);
  }
  .links {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2) var(--s-4);
  }
  a {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: var(--fs-sm);
    font-weight: 550;
    text-underline-offset: 4px;
  }
</style>
