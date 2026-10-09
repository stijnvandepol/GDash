/**
 * Short messages that fade away on their own ("Saved", "Removed").
 *
 * From any component:
 *
 *   import { toast } from './toast.svelte';
 *   toast.success('Saved');   // also toast.info('…') and toast.error('…')
 *
 * Admin.svelte renders them through <Toasts />.
 */

export type ToastKind = 'success' | 'info' | 'error';

export interface Toast {
  id: number;
  kind: ToastKind;
  message: string;
}

/** How long a message stays; an error stays longer so it can be read. */
const VISIBLE_MS: Record<ToastKind, number> = { success: 3500, info: 4500, error: 8000 };

/** More than this at once is noise; the oldest goes first. */
const MAX_VISIBLE = 4;

export const toasts = $state<{ items: Toast[] }>({ items: [] });

let nextId = 1;

function show(kind: ToastKind, message: string) {
  const id = nextId++;
  toasts.items = [...toasts.items, { id, kind, message }].slice(-MAX_VISIBLE);
  setTimeout(() => dismiss(id), VISIBLE_MS[kind]);
}

export function dismiss(id: number) {
  toasts.items = toasts.items.filter((t) => t.id !== id);
}

export const toast = {
  success: (message: string) => show('success', message),
  info: (message: string) => show('info', message),
  error: (message: string) => show('error', message),
};
