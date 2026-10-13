import { describe, expect, it } from 'vitest';
import {
  NOT_SET_UP,
  builtinState,
  groupByCategory,
  hashFor,
  isAvailable,
  matches,
  searchFromHash,
  sourceState,
  type CatalogueEntry,
  type IntegrationState,
} from './integrations';

const NOW = Date.parse('2026-10-13T12:00:00Z');

function entry(change: Partial<CatalogueEntry> = {}): CatalogueEntry {
  return {
    id: 'p1',
    name: 'HomeWizard P1 meter',
    description: 'Power now from the smart meter',
    category: 'energy',
    icon: 'bolt',
    tags: ['local', 'no-key', 'nl'],
    kind: 'preset',
    keywords: 'electricity',
    ...change,
  };
}

const connected: IntegrationState = { kind: 'connected', detail: 'Updated just now.' };

describe('isAvailable', () => {
  it('shows an entry only when the server reports its widget type or setting', () => {
    expect(isAvailable(entry(), [], {})).toBe(true);
    expect(isAvailable(entry({ requires: { widget: 'rain' } }), ['weather'], {})).toBe(false);
    expect(isAvailable(entry({ requires: { widget: 'rain' } }), ['weather', 'rain'], {})).toBe(true);
    expect(isAvailable(entry({ requires: { setting: 'immich' } }), [], { location: null })).toBe(false);
    expect(isAvailable(entry({ requires: { setting: 'immich' } }), [], { immich: null })).toBe(true);
  });
});

describe('matches', () => {
  it('searches the name, description, category, keywords and badges, every word', () => {
    expect(matches(entry(), NOT_SET_UP, '', 'all')).toBe(true);
    expect(matches(entry(), NOT_SET_UP, 'homewizard', 'all')).toBe(true);
    expect(matches(entry(), NOT_SET_UP, 'smart meter', 'all')).toBe(true);
    expect(matches(entry(), NOT_SET_UP, 'energy', 'all')).toBe(true);
    expect(matches(entry(), NOT_SET_UP, 'electricity', 'all')).toBe(true);
    expect(matches(entry(), NOT_SET_UP, 'no key', 'all')).toBe(true);
    expect(matches(entry(), NOT_SET_UP, 'P1 train', 'all')).toBe(false);
  });

  it('filters on what is connected and on badges', () => {
    expect(matches(entry(), NOT_SET_UP, '', 'connected')).toBe(false);
    expect(matches(entry(), connected, '', 'connected')).toBe(true);
    expect(matches(entry(), { kind: 'problem', detail: 'Failing' }, '', 'connected')).toBe(true);
    expect(matches(entry(), NOT_SET_UP, '', 'nl')).toBe(true);
    expect(matches(entry({ tags: ['free-key'] }), NOT_SET_UP, '', 'no-key')).toBe(false);
    expect(matches(entry({ tags: ['free-key'] }), NOT_SET_UP, '', 'local')).toBe(false);
  });
});

describe('groupByCategory', () => {
  it('keeps the order of the categories and leaves empty ones out', () => {
    const groups = groupByCategory([entry({ id: 'a', category: 'own' }), entry({ id: 'b', category: 'weather' }), entry({ id: 'c', category: 'own' })]);
    expect(groups.map((g) => [g.label, g.items.map((i) => i.id)])).toEqual([
      ['Weather & nature', ['b']],
      ['Your own data', ['a', 'c']],
    ]);
  });
});

describe('sourceState', () => {
  it('says when a source was updated, or that its last update failed', () => {
    const at = new Date(NOW).toISOString();
    expect(sourceState(undefined, 'Not fetched.')).toEqual({ kind: 'connected', detail: 'Not fetched.' });
    expect(sourceState({ updatedAt: at, failing: false }, '').kind).toBe('connected');
    expect(sourceState({ updatedAt: at, failing: true }, '')).toMatchObject({ kind: 'problem', detail: expect.stringMatching(/^The last update failed/) });
  });

  it('marks a built-in source without status as built in', () => {
    expect(builtinState(undefined)).toEqual({ kind: 'idle', detail: 'Built in · no setup' });
    expect(builtinState({ failing: true }).kind).toBe('problem');
  });
});

describe('the search in the address', () => {
  it('reads and writes the search and the filter', () => {
    expect(hashFor('', 'all')).toBe('#integrations');
    expect(hashFor(' solar panels ', 'nl')).toBe('#integrations?q=solar+panels&filter=nl');
    expect(searchFromHash('#integrations?q=solar+panels&filter=nl')).toEqual({ query: 'solar panels', filter: 'nl' });
    expect(searchFromHash('#integrations')).toEqual({ query: '', filter: 'all' });
  });

  it('ignores another hash and an unknown filter', () => {
    expect(searchFromHash('')).toBeNull();
    expect(searchFromHash('#integrationsx')).toBeNull();
    expect(searchFromHash('#integrations?filter=everything')).toEqual({ query: '', filter: 'all' });
  });
});
