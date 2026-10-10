<script lang="ts">
  let { size = 32 }: { size?: number } = $props();
</script>

<!-- A screen on a short foot, with its power light on. -->
<svg class="logo" width={size} height={size} viewBox="0 0 32 32" fill="none" aria-hidden="true" focusable="false">
  <rect class="frame" x="2.5" y="4.5" width="27" height="19" rx="4" stroke-width="3" />
  <circle class="led" cx="23.5" cy="18" r="2.5" />
  <rect class="foot" x="11" y="26.5" width="10" height="3" rx="1.5" />
</svg>

<style>
  .frame {
    stroke: var(--text);
  }
  .led {
    fill: var(--spark);
  }
  .foot {
    fill: var(--text);
  }
</style>
