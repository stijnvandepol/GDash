<script lang="ts">
  let { size = 32 }: { size?: number } = $props();
</script>

<!-- A rounded screen with a small sun on it. -->
<svg class="logo" width={size} height={size} viewBox="0 0 32 32" fill="none" aria-hidden="true" focusable="false">
  <rect class="frame" x="1.5" y="3" width="29" height="21" rx="6" />
  <circle class="sun" cx="16" cy="13.5" r="3.6" />
  <g class="rays" stroke-width="1.8" stroke-linecap="round">
    <path d="M16 6.8v1.4M16 18.8v1.4M9.3 13.5h1.4M21.3 13.5h1.4M11.3 8.8l1 1M19.7 17.2l1 1M20.7 8.8l-1 1M12.3 17.2l-1 1" />
  </g>
  <path class="stand" d="M11 28.5h10M16 24v4.5" stroke-width="2.2" stroke-linecap="round" />
</svg>

<style>
  .frame {
    fill: var(--primary);
  }
  .sun {
    fill: var(--spark);
  }
  .rays {
    stroke: var(--spark);
  }
  .stand {
    stroke: var(--primary);
  }
</style>
