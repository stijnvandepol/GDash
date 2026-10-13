<script lang="ts">
  // A block of text to copy, such as a script or a command. GDash often runs
  // on plain HTTP in the home, where browsers offer no clipboard to a page;
  // then the button selects the text for Ctrl+C instead.
  let { text, label }: { text: string; label: string } = $props();

  let block = $state<HTMLPreElement | null>(null);
  let note = $state('');

  async function copy() {
    if (navigator.clipboard && window.isSecureContext) {
      try {
        await navigator.clipboard.writeText(text);
        note = 'Copied.';
        return;
      } catch {
        // Refused, such as without permission: select it instead.
      }
    }
    if (!block) return;
    const range = document.createRange();
    range.selectNodeContents(block);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);
    note = 'Selected; press Ctrl+C (or ⌘C) to copy.';
  }
</script>

<div class="copy">
  <div class="copy-head">
    <span class="small">{label}</span>
    <button type="button" class="link" onclick={copy}>Copy</button>
  </div>
  <pre bind:this={block} aria-label={label}><code>{text}</code></pre>
  {#if note}<span class="small muted" role="status">{note}</span>{/if}
</div>

<style>
  .copy {
    display: grid;
    gap: var(--s-1);
    min-width: 0;
  }
  .copy-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
  }
  .copy-head span {
    font-weight: 550;
    color: var(--text-soft);
  }
  pre {
    margin: 0;
    padding: var(--s-3);
    max-height: 20rem;
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    background: var(--sunken);
    font-size: var(--fs-xs);
    line-height: 1.5;
  }
  code {
    font-family: var(--mono);
    white-space: pre;
  }
</style>
