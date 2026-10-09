<script lang="ts">
  import Icon from './Icon.svelte';

  // A file field that looks like the other buttons. The browser's own field
  // shows its text in the browser's language ("Bestand kiezen") and a
  // "no file chosen" note that means nothing here: the file is used as
  // soon as it is chosen. The real input stays in the page, hidden but
  // focusable, so keyboard and screen readers work as with any file field.
  let {
    label,
    accept,
    multiple = false,
    disabled = false,
    onchange,
  }: {
    label: string;
    accept: string;
    multiple?: boolean;
    disabled?: boolean;
    onchange: (event: Event) => void;
  } = $props();
</script>

<label class="file-button" class:disabled>
  <input class="sr-only" type="file" {accept} {multiple} {disabled} {onchange} />
  <span class="face"><Icon name="upload" size={18} /> {label}</span>
</label>

<style>
  .file-button {
    display: inline-flex;
    justify-self: start;
    cursor: pointer;
  }
  .face {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    min-height: 40px;
    padding: 0 var(--s-4);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
    background: var(--surface);
    color: var(--text);
    font-weight: 600;
    transition: background var(--t-fast) var(--ease);
  }
  .file-button:hover .face {
    background: var(--neutral-soft);
  }
  .file-button:focus-within .face {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }
  .disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }
</style>
