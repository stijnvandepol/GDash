<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { clock } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import { capabilityName, capabilityStates, showCapability } from './homey';
  import Readings from './Readings.svelte';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const states = $derived(capabilityStates(entry?.value));
  const keys = $derived(
    Array.isArray(settings?.capabilities) ? settings.capabilities.filter((k): k is string => typeof k === 'string') : [],
  );
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
  const rows = $derived(
    keys.map((key) => {
      const state = states?.[key];
      // A device that was removed: its capability ID says more than its device ID.
      if (!state) {
        const name = key.slice(key.indexOf('/') + 1);
        return { id: key, name, shown: { value: t('homeyMissing'), unit: '', dim: true }, missing: true };
      }
      return { id: key, name: capabilityName(state), shown: showCapability(state), missing: false };
    }),
  );
</script>

{#if states && entry}
  <Readings {rows} {stale} />
{:else}
  <span class="muted">{waitingText(entry, 'homey')}</span>
{/if}

<style>
  .muted {
    color: var(--text-muted);
    font-size: 38px;
  }
</style>
