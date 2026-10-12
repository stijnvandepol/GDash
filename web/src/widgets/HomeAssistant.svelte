<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { clock } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import { entityStates, show, type Shown } from './hass';
  import Readings from './Readings.svelte';

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const states = $derived(entityStates(entry?.value));
  const ids = $derived(
    Array.isArray(settings?.entities) ? settings.entities.filter((id): id is string => typeof id === 'string') : [],
  );
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
  const rows = $derived(
    ids.map((id) => {
      const entity = states?.[id];
      const shown: Shown = entity ? show(id, entity) : { value: t('haMissing'), unit: '', dim: true };
      return { id, name: entity?.name ?? id, shown, missing: !entity };
    }),
  );
</script>

{#if states && entry}
  <Readings {rows} {stale} />
{:else}
  <span class="muted">{waitingText(entry, 'homeassistant')}</span>
{/if}

<style>
  .muted {
    color: var(--text-muted);
    font-size: 38px;
  }
</style>
