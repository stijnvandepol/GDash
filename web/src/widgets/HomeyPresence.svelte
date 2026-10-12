<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { clock } from '../state.svelte';
  import { t } from '../i18n.svelte';
  import { presentLabels } from './homey';
  import Badges from './Badges.svelte';

  // The server sends the labels of the people who are home and nothing
  // about the others, so this widget cannot show who is away.
  let { entry, key }: { entry: Entry | undefined; key: string } = $props();

  const labels = $derived(entry ? presentLabels(entry.value, key) : undefined);
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
</script>

<div class="presence">
  {#if labels && entry}
    <Badges {labels} tone={stale || entry.failing ? 'warn' : undefined} empty={t('homeyNobodyHome')} />
  {:else}
    <span class="muted">{waitingText(entry, 'homeypresence')}</span>
  {/if}
</div>

<style>
  .presence {
    display: flex;
    flex-direction: column;
    justify-content: center;
    width: 100%;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }
  .muted {
    color: var(--text-muted);
    font-size: clamp(24px, 6.5cqw, 38px);
  }
</style>
