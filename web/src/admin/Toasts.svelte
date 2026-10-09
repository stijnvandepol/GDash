<script lang="ts">
  import Icon, { type IconName } from './Icon.svelte';
  import { dismiss, toasts, type ToastKind } from './toast.svelte';

  const ICONS: Record<ToastKind, IconName> = { success: 'check', info: 'info', error: 'alert' };
</script>

<div class="toasts" role="region" aria-label="Notifications" aria-live="polite">
  {#each toasts.items as t (t.id)}
    <div class="toast {t.kind}" role={t.kind === 'error' ? 'alert' : 'status'}>
      <Icon name={ICONS[t.kind]} size={18} />
      <span class="message">{t.message}</span>
      <button class="close" onclick={() => dismiss(t.id)} aria-label="Dismiss"><Icon name="close" size={16} /></button>
    </div>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    z-index: 50;
    left: 50%;
    bottom: var(--s-4);
    transform: translateX(-50%);
    width: min(420px, calc(100vw - 2 * var(--s-3)));
    display: grid;
    gap: var(--s-2);
    pointer-events: none;
  }
  /* Keep clear of the tab bar on phones. */
  @media (max-width: 640px) {
    :global(.with-tabs) .toasts {
      bottom: calc(var(--tabbar-h) + var(--s-3) + env(safe-area-inset-bottom));
    }
  }

  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: var(--s-3);
    padding: var(--s-3) var(--s-2) var(--s-3) var(--s-4);
    background: var(--raised);
    color: var(--text);
    border: 1px solid var(--border);
    border-left-width: 4px;
    border-radius: var(--r-md);
    box-shadow: var(--shadow-md);
    animation: rise var(--t-base) var(--ease);
  }
  .toast.success {
    border-left-color: var(--success);
  }
  .toast.success :global(.icon) {
    color: var(--success);
  }
  .toast.info {
    border-left-color: var(--accent);
  }
  .toast.info :global(.icon) {
    color: var(--accent);
  }
  .toast.error {
    border-left-color: var(--danger);
  }
  .toast.error :global(.icon) {
    color: var(--danger);
  }
  .message {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .close {
    min-height: 32px;
    min-width: 32px;
    padding: 0;
    background: none;
    color: var(--muted);
  }
  .close:hover:not(:disabled) {
    background: var(--neutral-soft);
    color: var(--text);
  }

  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
  }
</style>
