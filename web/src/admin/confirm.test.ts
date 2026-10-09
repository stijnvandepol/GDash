import { describe, expect, it } from 'vitest';
import { ask, confirmation } from './confirm.svelte';

describe('ask', () => {
  it('resolves with the answer and clears the question', async () => {
    const answer = ask({ title: 'Remove?', message: 'Gone for good.', action: 'Remove' });
    expect(confirmation.pending?.title).toBe('Remove?');
    expect(confirmation.pending?.danger).toBe(true);
    confirmation.pending?.answer(true);
    await expect(answer).resolves.toBe(true);
    expect(confirmation.pending).toBeNull();
  });

  it('cancels a question that a newer one replaces', async () => {
    const first = ask({ title: 'First?', message: '', action: 'Yes' });
    const second = ask({ title: 'Second?', message: '', action: 'Yes', danger: false });
    await expect(first).resolves.toBe(false);
    expect(confirmation.pending?.title).toBe('Second?');
    expect(confirmation.pending?.danger).toBe(false);
    confirmation.pending?.answer(false);
    await expect(second).resolves.toBe(false);
  });
});
