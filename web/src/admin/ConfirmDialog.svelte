<script lang="ts">
  import { confirmation } from './confirm.svelte';

  // The one confirmation dialog of the web interface, shown by ask(). A
  // native <dialog> opened as modal keeps focus inside and closes on
  // Escape; focus starts on Cancel, so Enter never removes something by
  // accident.
  let dialog: HTMLDialogElement;
  let cancelButton = $state<HTMLButtonElement>();

  $effect(() => {
    if (confirmation.pending && !dialog.open) {
      dialog.showModal();
      cancelButton?.focus();
    } else if (!confirmation.pending && dialog.open) {
      dialog.close();
    }
  });

  function answer(confirmed: boolean) {
    confirmation.pending?.answer(confirmed);
  }
</script>

<dialog
  bind:this={dialog}
  aria-labelledby="confirm-title"
  aria-describedby="confirm-message"
  oncancel={(e) => {
    e.preventDefault();
    answer(false);
  }}
>
  {#if confirmation.pending}
    <h2 id="confirm-title">{confirmation.pending.title}</h2>
    <p id="confirm-message">{confirmation.pending.message}</p>
    <div class="actions">
      <button bind:this={cancelButton} type="button" class="secondary" onclick={() => answer(false)}>Cancel</button>
      <button type="button" class:danger={confirmation.pending.danger} onclick={() => answer(true)}>
        {confirmation.pending.action}
      </button>
    </div>
  {/if}
</dialog>

<style>
  dialog {
    width: min(100% - 2 * var(--s-4), 440px);
    padding: var(--s-5);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    background: var(--raised);
    color: var(--text);
    box-shadow: var(--shadow-md);
  }
  dialog::backdrop {
    background: rgb(20 17 14 / 0.45);
  }
  p {
    margin: var(--s-3) 0 var(--s-5);
    color: var(--text-soft);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--s-2);
  }
</style>
