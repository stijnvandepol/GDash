<script lang="ts">
  // Line icons on a 24x24 grid, drawn in the theme's text colour: the sun
  // on the horizon with an arrow up for the sunrise and down for the sunset.
  let { kind }: { kind: 'sunrise' | 'sunset' } = $props();
</script>

<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <path d="M2.5 19.5h19" />
  <path d="M6.5 19.5a5.5 5.5 0 0 1 11 0" />
  {#if kind === 'sunrise'}
    <path d="M12 3.5v6M9.5 6l2.5-2.5L14.5 6" />
  {:else}
    <path d="M12 3.5v6M9.5 7l2.5 2.5L14.5 7" />
  {/if}
</svg>

<style>
  svg {
    width: 1em;
    height: 1em;
    flex: none;
  }
</style>
