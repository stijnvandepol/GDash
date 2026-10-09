<script lang="ts">
  import { t } from './i18n.svelte';
  import { shrinkToFit } from './widgets/fit';

  // Smallest text that stays readable across a room.
  const MIN_FONT_PX = 36;

  let { title, message }: { title?: string; message: string } = $props();
</script>

<!--
  Sits on the stage, so it follows the screen's shape and the display's text
  size. Both texts are rendered as text: a message is never parsed as HTML.
-->
<div class="scrim">
  <div class="card" role="alert" aria-label={t('announcement')}>
    {#if title}<p class="title">{title}</p>{/if}
    <p class="message" use:shrinkToFit={MIN_FONT_PX}>{message}</p>
  </div>
</div>

<style>
  .scrim {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    background: rgba(0, 0, 0, 0.78);
    animation: arrive 400ms ease-out;
  }
  /* surface-0 is a solid colour in every theme, so the text stays readable
     over a translucent theme such as Glass. */
  .card {
    box-sizing: border-box;
    width: min(1500px, 82%);
    /* Relative to the stage, so on a small or magnified screen the message
       shrinks to fit instead of running off it. */
    max-height: 80%;
    display: flex;
    flex-direction: column;
    gap: 24px;
    padding: 56px 72px 64px;
    background: var(--backdrop), var(--surface-0);
    border: var(--widget-border-width) solid var(--widget-border);
    border-top: 14px solid var(--warning);
    border-radius: 28px;
    color: var(--text-primary);
    text-align: center;
  }
  .title {
    margin: 0;
    font-size: 40px;
    font-weight: 600;
    letter-spacing: 0.02em;
    color: var(--text-muted);
    overflow-wrap: anywhere;
  }
  .message {
    margin: 0;
    flex: 0 1 auto;
    min-height: 0;
    overflow: hidden;
    font-size: 76px;
    font-weight: 600;
    line-height: 1.25;
    overflow-wrap: anywhere;
  }
  @keyframes arrive {
    from {
      opacity: 0;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .scrim {
      animation: none;
    }
  }
</style>
