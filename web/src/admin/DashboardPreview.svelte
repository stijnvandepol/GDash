<script lang="ts">
  import { onMount } from 'svelte';
  import { PREVIEW_MESSAGE, type PreviewDraft, type PreviewMessage } from '../previewApi';
  import { SHAPES, TEXT_SIZES, fitFrame } from './previewShapes';

  let {
    dashboardId,
    draft,
    onClose,
  }: {
    dashboardId: string;
    /** The layout in the editor when it is not saved; null to show the saved dashboard. */
    draft: PreviewDraft | null;
    onClose: () => void;
  } = $props();

  // Room the dialog needs besides the preview: its header, controls and padding.
  const DIALOG_CHROME = 200;

  let dialog: HTMLDialogElement;
  let frame = $state<HTMLIFrameElement>();
  let loaded = $state(false);
  let shapeId = $state(SHAPES[0]!.id);
  let textScale = $state(100);
  let availableWidth = $state(0);
  let windowHeight = $state(0);

  const shape = $derived(SHAPES.find((s) => s.id === shapeId) ?? SHAPES[0]!);
  const fit = $derived(fitFrame(shape, availableWidth, windowHeight - DIALOG_CHROME));

  onMount(() => dialog.showModal());

  // The page in the frame draws the dashboard; it is told what to show once
  // it has loaded, and again whenever the text size changes.
  $effect(() => {
    if (!loaded || !frame?.contentWindow) return;
    const message: PreviewMessage = { type: PREVIEW_MESSAGE, textScale, draft: $state.snapshot(draft) };
    frame.contentWindow.postMessage(message, location.origin);
  });

  /** A click on the backdrop, outside the dialog's box, closes it. */
  function closeOnBackdrop(event: MouseEvent) {
    if (event.target === dialog) dialog.close();
  }
</script>

<svelte:window bind:innerHeight={windowHeight} />

<!-- The dialog closes on Escape by itself; the backdrop click is a convenience. -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<dialog bind:this={dialog} onclose={onClose} onclick={closeOnBackdrop} aria-label="Dashboard preview">
  <div class="head">
    <strong>Preview</strong>
    <label class="option">
      Screen
      <select bind:value={shapeId}>
        {#each SHAPES as s (s.id)}<option value={s.id}>{s.label}</option>{/each}
      </select>
    </label>
    <label class="option">
      Text size
      <select bind:value={textScale}>
        {#each TEXT_SIZES as [size, label] (size)}<option value={size}>{label}</option>{/each}
      </select>
    </label>
    <span class="spacer"></span>
    <button class="link" onclick={() => dialog.close()}>Close</button>
  </div>

  <div class="room" bind:clientWidth={availableWidth}>
    <div class="frame" style:width="{fit.width}px" style:height="{fit.height}px">
      <iframe
        bind:this={frame}
        title="Dashboard preview"
        src="/preview/{encodeURIComponent(dashboardId)}"
        onload={() => (loaded = true)}
        style:width="{shape.width}px"
        style:height="{shape.height}px"
        style:transform="scale({fit.scale})"
      ></iframe>
    </div>
  </div>

  <p class="muted small">
    {draft
      ? 'Showing your unsaved changes. Nothing is saved and no screen changes.'
      : 'Showing the saved dashboard, with the data screens get now.'}
  </p>
</dialog>

<style>
  dialog {
    width: min(1100px, 94vw);
    box-sizing: border-box;
    padding: 16px 20px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--card);
    color: var(--text);
  }
  dialog[open] {
    display: grid;
    /* A column may shrink below its widest control, or the dialog scrolls sideways on a phone. */
    grid-template-columns: minmax(0, 1fr);
    gap: 12px;
  }
  dialog::backdrop {
    background: var(--scrim);
  }
  .head {
    display: flex;
    gap: 14px;
    align-items: center;
    flex-wrap: wrap;
  }
  .option {
    min-width: 0;
    max-width: 100%;
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 400;
  }
  .option select {
    flex: 1;
  }
  .room {
    min-width: 0;
  }
  .frame {
    margin: 0 auto;
    overflow: hidden;
    border-radius: var(--r-md);
    background: var(--device-screen);
  }
  iframe {
    display: block;
    border: 0;
    transform-origin: 0 0;
  }
</style>
