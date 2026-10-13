import type { CustomSource, Settings, SourceStatus } from './api';
import type { IconName } from './Icon.svelte';
import { describeStatus } from './status';

// The catalogue of integrations: every service GDash can show data from,
// whether it needs setting up, ships built in or starts from a preset. The
// cards themselves are listed in integrationRegistry.ts; this file holds
// what they are made of and how the page finds them.

export const CATEGORIES = [
  { id: 'weather', label: 'Weather & nature' },
  { id: 'travel', label: 'Travel' },
  { id: 'energy', label: 'Energy' },
  { id: 'home', label: 'Home' },
  { id: 'lists', label: 'Calendar & lists' },
  { id: 'media', label: 'Photos & media' },
  { id: 'news', label: 'News' },
  { id: 'own', label: 'Your own data' },
] as const;

export type CategoryId = (typeof CATEGORIES)[number]['id'];

/** The small labels on a card, which the filters also use. */
export const TAGS = {
  'no-key': 'No key',
  'free-key': 'Free key',
  local: 'Local',
  nl: 'NL',
  push: 'Push',
} as const;

export type Tag = keyof typeof TAGS;

export const FILTERS = [
  { id: 'all', label: 'All' },
  { id: 'connected', label: 'Connected' },
  { id: 'nl', label: 'Netherlands' },
  { id: 'local', label: 'Local' },
  { id: 'no-key', label: 'No key' },
] as const;

export type FilterId = (typeof FILTERS)[number]['id'];

/**
 * service: set up here, with an address, a key or a list.
 * builtin: ships with GDash; a widget is all it takes.
 * preset: a ready-made custom API.
 */
export type IntegrationKind = 'service' | 'builtin' | 'preset';

/** What a card's status light says. */
export type IntegrationState =
  | { kind: 'connected'; detail: string }
  | { kind: 'problem'; detail: string }
  | { kind: 'idle'; detail: string };

/** What the cards read their state from; the page loads it. */
export interface IntegrationContext {
  settings: Settings;
  status: Record<string, SourceStatus>;
  sources: CustomSource[];
  photoCount: number;
  tokenCount: number;
  /** The widget types this server reports. */
  widgetTypes: string[];
}

export interface CatalogueEntry {
  id: string;
  name: string;
  /** One line: what it shows. */
  description: string;
  category: CategoryId;
  icon: IconName;
  tags: Tag[];
  kind: IntegrationKind;
  /** Other words search finds it by, such as the provider. */
  keywords?: string;
  /**
   * What the server must have for the card to show: a widget type, or a
   * key in the settings. A card for something that is still being built
   * appears by itself once the server reports it.
   */
  requires?: { widget?: string; setting?: string };
}

/** Whether this server has what the entry needs. */
export function isAvailable(entry: CatalogueEntry, widgetTypes: string[], settings: object): boolean {
  if (entry.requires?.widget && !widgetTypes.includes(entry.requires.widget)) return false;
  if (entry.requires?.setting && !(entry.requires.setting in settings)) return false;
  return true;
}

/** Whether an entry passes the search words, which look in its name, description, category and badges, and the chosen filter. */
export function matches(entry: CatalogueEntry, state: IntegrationState, query: string, filter: FilterId): boolean {
  if (filter === 'connected' && state.kind === 'idle') return false;
  if (filter !== 'all' && filter !== 'connected' && !entry.tags.includes(filter)) return false;
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const category = CATEGORIES.find((c) => c.id === entry.category)?.label ?? '';
  const text = [entry.name, entry.description, category, entry.keywords ?? '', ...entry.tags.map((t) => TAGS[t])].join(' ').toLowerCase();
  return words.every((word) => text.includes(word));
}

export interface CategoryGroup<T> {
  id: CategoryId;
  label: string;
  items: T[];
}

/** Entries by category, in the order of CATEGORIES; empty categories are left out. */
export function groupByCategory<T extends { category: CategoryId }>(entries: T[]): CategoryGroup<T>[] {
  return CATEGORIES.map((c) => ({ id: c.id, label: c.label, items: entries.filter((e) => e.category === c.id) })).filter(
    (g) => g.items.length > 0,
  );
}

/**
 * The state of something that is set up and fetched as a data source:
 * connected with its last update, or the problem. Without a status the
 * source is not fetched: notFetched says why.
 */
export function sourceState(status: SourceStatus | undefined, notFetched: string): IntegrationState {
  if (!status) return { kind: 'connected', detail: notFetched };
  if (status.failing) return { kind: 'problem', detail: describeStatus(status) };
  return { kind: 'connected', detail: describeStatus(status) };
}

/** A built-in source: its status once a dashboard shows it. */
export function builtinState(status: SourceStatus | undefined): IntegrationState {
  if (!status) return { kind: 'idle', detail: 'Built in · no setup' };
  return sourceState(status, '');
}

export const NOT_SET_UP: IntegrationState = { kind: 'idle', detail: 'Not set up' };

/** "1 feed", "3 feeds". */
export function count(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

/** The address hash of the Integrations page; Admin opens the tab on it. */
export const INTEGRATIONS_HASH = '#integrations';

/**
 * The search and filter kept in the address, such as
 * #integrations?q=solar&filter=nl, so a link or a reload keeps them. Null
 * when the hash is not the Integrations page's.
 */
export function searchFromHash(hash: string): { query: string; filter: FilterId } | null {
  if (hash !== INTEGRATIONS_HASH && !hash.startsWith(`${INTEGRATIONS_HASH}?`)) return null;
  const params = new URLSearchParams(hash.slice(INTEGRATIONS_HASH.length + 1));
  const filter = FILTERS.find((f) => f.id === params.get('filter'))?.id ?? 'all';
  return { query: params.get('q') ?? '', filter };
}

/** The hash for a search and filter; the defaults are left out. */
export function hashFor(query: string, filter: FilterId): string {
  const params = new URLSearchParams();
  if (query.trim() !== '') params.set('q', query.trim());
  if (filter !== 'all') params.set('filter', filter);
  const rest = params.toString();
  return rest ? `${INTEGRATIONS_HASH}?${rest}` : INTEGRATIONS_HASH;
}
