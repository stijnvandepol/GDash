<script lang="ts">
  import { iconFor } from './wmo';

  // Line icons on a 24x24 grid, drawn in the theme's text colour.
  let { code, day = true }: { code: number; day?: boolean } = $props();

  const kind = $derived(iconFor(code, day));
</script>

<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  {#if kind === 'sun'}
    <circle cx="12" cy="12" r="4" />
    <path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4" />
  {:else if kind === 'moon'}
    <path d="M19 14.5A7.5 7.5 0 0 1 9.5 5a7.5 7.5 0 1 0 9.5 9.5z" />
  {:else if kind === 'partly'}
    <circle cx="8.5" cy="8.5" r="3" />
    <path d="M8.5 2.5v1.2M2.5 8.5h1.2M4.3 4.3l.9.9M12.7 4.3l-.9.9" />
    <path d="M9 19h8.5a3.5 3.5 0 0 0 .3-7 4.8 4.8 0 0 0-9.1 1.2A2.9 2.9 0 0 0 9 19z" />
  {:else}
    {#if kind === 'fog'}
      <path d="M4 9h16M6 13h12M4 17h16" />
    {:else}
      <path d="M7 15h10a4 4 0 0 0 .4-8 5.5 5.5 0 0 0-10.5 1.3A3.4 3.4 0 0 0 7 15z" />
    {/if}
    {#if kind === 'rain'}
      <path d="M8.5 17.5l-1 3M12.5 17.5l-1 3M16.5 17.5l-1 3" />
    {:else if kind === 'drizzle'}
      <path d="M8.5 18v.5M12.5 19v.5M16.5 18v.5" />
    {:else if kind === 'snow'}
      <path d="M8.5 18.5v.01M12 20v.01M15.5 18.5v.01M10 21.5v.01M14 21.5v.01" stroke-width="2.2" />
    {:else if kind === 'thunder'}
      <path d="M12.5 15.5l-2 3.5h3l-2 3.5" />
    {/if}
  {/if}
</svg>

<style>
  svg {
    width: 1em;
    height: 1em;
    flex: none;
  }
</style>
