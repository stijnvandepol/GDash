<script lang="ts">
  import { shownList, shownValue, type Preset } from './presets';

  // What a device or service answered when it was tested: each of the
  // preset's values by name, or why it could not be asked.
  let {
    preset,
    answer,
  }: {
    preset: Preset;
    /** The answer, why asking failed, or null while asking. */
    answer: { data: unknown } | { problem: string } | null;
  } = $props();
</script>

<div class="answer" role="status">
  {#if answer === null}
    <p class="small muted">Asking {preset.name}…</p>
  {:else if 'problem' in answer}
    <p class="small warn">GDash could not reach it: {answer.problem}</p>
  {:else}
    <p class="small"><span class="badge ok">It works</span> {preset.name} answered. What it says now:</p>
    <ul class="list values">
      {#each preset.values as value (value.name)}
        <li><span class="grow">{value.name}</span><span class="now">{shownValue(answer.data, value)}</span></li>
      {/each}
      {#each preset.lists as list (list.name)}
        <li><span class="grow">{list.name}</span><span class="now">{shownList(answer.data, list)}</span></li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .answer {
    display: grid;
    gap: var(--s-2);
    min-width: 0;
  }
  .values {
    max-width: 40rem;
  }
  .now {
    font-family: var(--mono);
    font-size: var(--fs-sm);
    overflow-wrap: anywhere;
  }
  .warn {
    color: var(--warning);
  }
</style>
