import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { dismiss, toast, toasts } from './toast.svelte';

describe('toast', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.runAllTimers();
    vi.useRealTimers();
  });

  it('shows a message with its kind and removes it after a while', () => {
    toast.success('Saved');
    expect(toasts.items.map((t) => [t.kind, t.message])).toEqual([['success', 'Saved']]);

    vi.advanceTimersByTime(10_000);
    expect(toasts.items).toEqual([]);
  });

  it('keeps an error on screen longer than a confirmation', () => {
    toast.success('Saved');
    toast.error('Saving failed');

    vi.advanceTimersByTime(5000);
    expect(toasts.items.map((t) => t.message)).toEqual(['Saving failed']);
  });

  it('dismisses one message by id', () => {
    toast.info('One');
    toast.info('Two');
    const [first] = toasts.items;
    dismiss(first!.id);
    expect(toasts.items.map((t) => t.message)).toEqual(['Two']);
  });

  it('drops the oldest message when too many pile up', () => {
    for (let i = 1; i <= 6; i++) toast.info(`Message ${i}`);
    expect(toasts.items.map((t) => t.message)).toEqual(['Message 3', 'Message 4', 'Message 5', 'Message 6']);
  });
});
