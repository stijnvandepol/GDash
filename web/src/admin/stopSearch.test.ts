import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SEARCH_DELAY_MS, typeAheadSearch } from './stopSearch';

function setup(search: (q: string) => Promise<string>) {
  const events: string[] = [];
  const handlers = {
    onIdle: () => events.push('idle'),
    onBusy: () => events.push('busy'),
    onResult: (r: string) => events.push(`result:${r}`),
    onError: (e: unknown) => events.push(`error:${(e as Error).message}`),
  };
  return { events, typeahead: typeAheadSearch(search, handlers) };
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe('typeAheadSearch', () => {
  it('does not search for fewer than two characters', async () => {
    const search = vi.fn(async (q: string) => q);
    const { events, typeahead } = setup(search);
    typeahead.input('u');
    typeahead.input('  u ');
    await vi.advanceTimersByTimeAsync(SEARCH_DELAY_MS * 2);
    expect(search).not.toHaveBeenCalled();
    expect(events).toEqual(['idle', 'idle']);
  });

  it('asks once, for the last text, after the typing pauses', async () => {
    const search = vi.fn(async (q: string) => q);
    const { events, typeahead } = setup(search);
    typeahead.input('ut');
    await vi.advanceTimersByTimeAsync(SEARCH_DELAY_MS - 1);
    typeahead.input('utr');
    typeahead.input(' utrecht ');
    await vi.advanceTimersByTimeAsync(SEARCH_DELAY_MS);
    expect(search).toHaveBeenCalledTimes(1);
    expect(search).toHaveBeenCalledWith('utrecht');
    expect(events).toEqual(['busy', 'busy', 'busy', 'result:utrecht']);
  });

  it('ignores a slow answer that arrives after a newer one', async () => {
    const answers: Record<string, (v: string) => void> = {};
    const search = (q: string) => new Promise<string>((resolve) => (answers[q] = resolve));
    const { events, typeahead } = setup(search);

    typeahead.input('ut');
    await vi.advanceTimersByTimeAsync(SEARCH_DELAY_MS);
    typeahead.input('utrecht');
    await vi.advanceTimersByTimeAsync(SEARCH_DELAY_MS);
    answers['utrecht']!('new');
    await vi.advanceTimersByTimeAsync(0);
    answers['ut']!('old');
    await vi.advanceTimersByTimeAsync(0);

    expect(events.filter((e) => e.startsWith('result'))).toEqual(['result:new']);
  });

  it('ignores an answer after the text was cleared', async () => {
    let resolve: (v: string) => void = () => {};
    const { events, typeahead } = setup(() => new Promise<string>((r) => (resolve = r)));
    typeahead.input('utrecht');
    await vi.advanceTimersByTimeAsync(SEARCH_DELAY_MS);
    typeahead.input('');
    resolve('late');
    await vi.advanceTimersByTimeAsync(0);
    expect(events).toEqual(['busy', 'idle']);
  });

  it('passes on the error of the last search only', async () => {
    const search = vi.fn(async (q: string) => {
      throw new Error(`down for ${q}`);
    });
    const { events, typeahead } = setup(search);
    typeahead.input('utrecht');
    await vi.advanceTimersByTimeAsync(SEARCH_DELAY_MS);
    expect(events).toEqual(['busy', 'error:down for utrecht']);
  });

  it('drops a waiting search on cancel', async () => {
    const search = vi.fn(async (q: string) => q);
    const { events, typeahead } = setup(search);
    typeahead.input('utrecht');
    typeahead.cancel();
    await vi.advanceTimersByTimeAsync(SEARCH_DELAY_MS * 2);
    expect(search).not.toHaveBeenCalled();
    expect(events).toEqual(['busy']);
  });
});
