import { describe, expect, it } from 'vitest';
import { newestOnly } from './newest';

describe('newestOnly', () => {
  it('drops an older answer that arrives after a newer one', async () => {
    const answers: Array<(v: string) => void> = [];
    const load = newestOnly(() => new Promise<string>((resolve) => answers.push(resolve)));

    const poll = load(); // started first, for example by the timer
    const afterAdd = load(); // started later, after adding a calendar
    answers[1]!('with the new calendar');
    answers[0]!('without it');

    expect(await afterAdd).toEqual({ value: 'with the new calendar' });
    expect(await poll).toBeNull();
  });

  it('passes errors on', async () => {
    const load = newestOnly(() => Promise.reject(new Error('offline')));
    await expect(load()).rejects.toThrow('offline');
  });
});
