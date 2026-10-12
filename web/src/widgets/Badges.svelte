<script lang="ts">
  import type { Tone } from './custom';

  // Everything shown is text, never markup: labels come from settings and
  // APIs the household chose.
  let { labels, tone, empty }: { labels: string[]; tone?: Tone; empty: string } = $props();
</script>

<ul class="badges">
  {#each labels as label, i (i)}
    <li class="badge" class:warn={tone === 'warn' || tone === 'bad'}>{label}</li>
  {:else}
    <li class="none">{empty}</li>
  {/each}
</ul>

<style>
  /* Badges wrap onto more lines when there are many; a short label such as
     initials makes a circle, a longer one a pill. */
  .badges {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    align-content: center;
    gap: min(16px, 3cqw);
    flex: 1 1 0;
    min-height: 0;
    overflow: hidden;
  }
  .badge {
    --size: clamp(56px, min(26cqw, 48cqh), 140px);
    display: grid;
    place-items: center;
    min-width: var(--size);
    height: var(--size);
    padding: 0 0.35em;
    box-sizing: border-box;
    border: max(3px, calc(var(--size) * 0.05)) solid var(--accent);
    border-radius: 999px;
    /* Accent on the tile is a pair every theme keeps at 4.5:1 or more
       (themes.test.ts); a filled badge would need a colour for its text
       that the themes do not define. */
    color: var(--accent);
    font-size: calc(var(--size) * 0.38);
    font-weight: 700;
    line-height: 1;
    letter-spacing: 0.02em;
    white-space: nowrap;
  }
  /* The warning colour, which every theme keeps readable on its tiles. */
  .badge.warn {
    border-color: var(--warning);
    color: var(--warning);
  }
  .none {
    color: var(--text-muted);
    font-size: clamp(24px, 6cqw, 40px);
    line-height: 1.25;
  }
</style>
