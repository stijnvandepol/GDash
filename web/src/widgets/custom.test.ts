import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Entry, Snapshot } from '../protocol';
import { formatCell, shownFor, waitingFor } from './custom';

beforeEach(() => {
  vi.stubGlobal('navigator', { language: 'en-US' });
});

afterEach(() => {
  display.snapshot = null;
  vi.unstubAllGlobals();
});

const entry = (value: unknown): Entry => ({ value, fetchedAt: '', staleAfter: '', failing: false });

describe('shownFor', () => {
  it('finds the widget by dashboard and widget', () => {
    const e = entry({ 'hall/c': { state: 'ok', mode: 'value', value: 1 }, 'hall/d': { state: 'ok' } });
    expect(shownFor(e, 'hall/c')?.value).toBe(1);
    expect(shownFor(e, 'kitchen/c')).toBeUndefined();
    expect(shownFor(e, 'hall/d')).toBeUndefined();
    expect(shownFor(undefined, 'hall/c')).toBeUndefined();
    expect(shownFor(entry([1]), 'hall/c')).toBeUndefined();
  });
});

describe('formatCell', () => {
  it('formats numbers in the screen language', () => {
    expect(formatCell(1234.5678)).toBe('1,234.57');
    expect(formatCell(2, 2)).toBe('2.00');
    display.snapshot = { language: 'nl' } as Snapshot;
    expect(formatCell(21.5, 1)).toBe('21,5');
    expect(formatCell(true)).toBe('Ja');
  });

  it('shows text as it is and missing values as a dash', () => {
    expect(formatCell('<b>not markup</b>')).toBe('<b>not markup</b>');
    expect(formatCell(null)).toBe('–');
    expect(formatCell(undefined)).toBe('–');
    expect(formatCell(false)).toBe('No');
  });
});

describe('waitingFor', () => {
  it('says why there is nothing to show', () => {
    expect(waitingFor(undefined)).toBe('Loading…');
    expect(waitingFor({ state: 'missing', mode: 'value' })).toContain('not available');
    expect(waitingFor({ state: 'failing', mode: 'value' })).toContain('could not be fetched');
  });
});
