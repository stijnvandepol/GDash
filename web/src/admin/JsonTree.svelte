<script lang="ts">
  import JsonTree from './JsonTree.svelte';
  import { formatPath } from './jsonPath';

  // Levels shown open at first; deeper ones open with a click, so a large
  // answer does not fill the page.
  const OPEN_DEPTH = 2;

  let {
    value,
    steps = [],
    onpick,
  }: {
    value: unknown;
    /** The keys and indexes from the top of the answer to value. */
    steps?: (string | number)[];
    /** Called with the path of a clicked value, or of a list chosen as a whole. */
    onpick: (path: string, value: unknown) => void;
  } = $props();

  function isContainer(v: unknown): v is object {
    return typeof v === 'object' && v !== null;
  }

  function entries(v: object): [string | number, unknown][] {
    return Array.isArray(v) ? v.map((item, i) => [i, item]) : Object.entries(v);
  }

  // Text in quotes, so "12" and 12 look different, as they are to a rule.
  function shown(v: unknown): string {
    return typeof v === 'string' ? JSON.stringify(v) : String(v);
  }

  function size(v: object): string {
    return Array.isArray(v) ? `list of ${v.length}` : `${Object.keys(v).length} keys`;
  }
</script>

{#if isContainer(value)}
  <ul class="tree" class:top={steps.length === 0}>
    {#each entries(value) as [key, child] (key)}
      {@const path = formatPath([...steps, key])}
      <li>
        {#if isContainer(child)}
          <details open={steps.length < OPEN_DEPTH}>
            <summary>
              <span class="key">{key}</span>
              <span class="muted">{size(child)}</span>
              {#if Array.isArray(child)}
                <button
                  type="button"
                  class="link use"
                  title={path}
                  onclick={(e) => {
                    e.preventDefault();
                    onpick(path, child);
                  }}>Use this list</button
                >
              {/if}
            </summary>
            <JsonTree value={child} steps={[...steps, key]} {onpick} />
          </details>
        {:else}
          <button type="button" class="leaf" title={path} onclick={() => onpick(path, child)}>
            <span class="key">{key}</span>
            <span class="value">{shown(child)}</span>
          </button>
        {/if}
      </li>
    {/each}
  </ul>
{:else}
  <button type="button" class="leaf" title="$" onclick={() => onpick('$', value)}>
    <span class="value">{shown(value)}</span>
  </button>
{/if}

<style>
  .tree {
    list-style: none;
    margin: 0;
    padding: 0 0 0 var(--s-4);
    font-family: var(--mono);
    font-size: var(--fs-sm);
  }
  /* A tree inside a list row (Settings) must not take that list's rules
     and padding. */
  .tree li {
    display: block;
    padding: 0;
    border: 0;
  }
  .tree.top {
    padding-left: 0;
    max-height: 24rem;
    overflow: auto;
  }
  /* A flex summary loses the browser's triangle, so it draws its own. */
  summary::before {
    content: '▸';
    color: var(--muted);
  }
  details[open] > summary::before {
    content: '▾';
  }
  summary {
    cursor: pointer;
    min-height: 28px;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    flex-wrap: wrap;
  }
  .leaf {
    display: flex;
    gap: var(--s-2);
    min-height: 28px;
    padding: 2px 6px;
    border: 0;
    border-radius: var(--r-sm);
    background: none;
    color: inherit;
    font: inherit;
    text-align: left;
    max-width: 100%;
  }
  .leaf:hover {
    background: var(--neutral-soft);
  }
  .key {
    color: var(--text-soft);
  }
  .key::after {
    content: ':';
  }
  .value {
    color: var(--text);
    overflow-wrap: anywhere;
  }
  .use {
    min-height: 24px;
  }
</style>
