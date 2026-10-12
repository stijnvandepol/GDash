<script lang="ts">
  import type { Snippet } from 'svelte';

  // Connects a browser screen to a display: where to open the display's
  // page, and the field for the code that page then shows.
  let {
    address,
    onpair,
    onerror,
    oncancel,
    children,
  }: {
    /** The display's page, in full, as the person types it on the screen. */
    address: string;
    /** Approves the code; rejects when the server refuses it. */
    onpair: (code: string) => Promise<void>;
    onerror: (err: unknown) => string;
    /** Shown as a Cancel button when the panel can be closed. */
    oncancel?: () => void;
    /** The sentence before the steps, which differs per display. */
    children: Snippet;
  } = $props();

  let code = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    busy = true;
    try {
      await onpair(code.trim());
      code = '';
    } catch (err) {
      error = onerror(err);
    } finally {
      busy = false;
    }
  }
</script>

<div class="connect">
  <p class="lead">{@render children()}</p>
  <!-- Steps as paragraphs: this panel sits in a list row, whose styles
       would turn list items into rows of their own. -->
  <p class="step"><span class="number" aria-hidden="true">1</span><span>On the TV, tablet or computer, open <code class="address">{address}</code></span></p>
  <p class="step"><span class="number" aria-hidden="true">2</span><span>Enter the code that appears there:</span></p>
  <form class="inline" onsubmit={submit}>
    <input
      bind:value={code}
      aria-label="Code shown on the screen"
      placeholder="ABCD-EFGH"
      autocomplete="off"
      autocapitalize="characters"
      spellcheck="false"
      required
    />
    <button type="submit" disabled={busy}>{busy ? 'Connecting…' : 'Connect'}</button>
    {#if oncancel}<button type="button" class="link" onclick={oncancel}>Cancel</button>{/if}
  </form>
  {#if error}<p class="error">{error}</p>{/if}
</div>

<style>
  .connect {
    display: grid;
    gap: var(--s-2);
    padding: var(--s-4);
    border-radius: var(--r-lg);
    background: var(--sunken);
  }
  .lead {
    margin: 0;
    font-weight: 550;
  }
  .step {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    margin: 0;
    color: var(--text-soft);
  }
  .number {
    flex: none;
    font-family: var(--mono);
    font-size: var(--fs-xs);
    color: var(--muted);
  }
  .address {
    font-family: var(--mono);
    font-size: var(--fs-sm);
    color: var(--text);
    overflow-wrap: anywhere;
    user-select: all;
  }
  form {
    margin-top: var(--s-1);
  }
  input {
    font-family: var(--mono);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    max-width: 14ch;
  }
  .error {
    margin: 0;
  }
</style>
