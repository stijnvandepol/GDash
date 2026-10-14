// What a News widget shows: the headlines of the feeds it was given, from
// the news.default entry, which holds every feed of the screen by its ID.

export interface Headline {
  title: string;
  published?: string;
  /** The feed's name, when the widget shows several feeds. */
  source?: string;
}

interface SourceFeed {
  name: string;
  items: Headline[];
}

export interface NewsView {
  /** The feed's name when the widget shows one feed; empty for several, which the screen titles in its own language. */
  title: string;
  items: Headline[];
}

/** The widget was given feeds that are no longer under Integrations. */
export const SOURCES_REMOVED = 'removed';

export const DEFAULT_ITEMS = 5;

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function isFeed(v: unknown): v is SourceFeed {
  return (
    isRecord(v) &&
    typeof v.name === 'string' &&
    Array.isArray(v.items) &&
    v.items.every((i) => isRecord(i) && typeof i.title === 'string')
  );
}

/** The feeds a widget's settings choose: a list of IDs, or undefined for every feed. */
export function chosenFeeds(settings: Record<string, unknown> | undefined): number[] | undefined {
  const feeds = settings?.feeds;
  if (!Array.isArray(feeds)) return undefined;
  const ids = feeds.filter((id): id is number => typeof id === 'number' && Number.isInteger(id));
  return ids.length > 0 ? ids : undefined;
}

/** When the headline was published, or -Infinity without a readable date. */
function publishedMs(item: Headline): number {
  const ms = item.published ? Date.parse(item.published) : NaN;
  return Number.isNaN(ms) ? -Infinity : ms;
}

function newestFirst(a: Headline, b: Headline): number {
  const [x, y] = [publishedMs(a), publishedMs(b)];
  return x === y ? 0 : x < y ? 1 : -1;
}

/**
 * The headlines a widget shows, newest first and cut to its maxItems, or
 * null when value is not the news entry, or SOURCES_REMOVED when none of
 * its chosen feeds is there any more. With one feed the title is its name;
 * with several each headline names its feed.
 */
export function newsView(value: unknown, settings: Record<string, unknown> | undefined): NewsView | typeof SOURCES_REMOVED | null {
  if (!isRecord(value)) return null;
  const chosen = chosenFeeds(settings);
  // In the order they were added, which is the order of their IDs.
  const ids = (chosen ?? Object.keys(value).map(Number)).filter((id) => Number.isInteger(id)).sort((a, b) => a - b);
  const feeds = [...new Set(ids)].map((id) => value[String(id)]).filter(isFeed);
  if (feeds.length === 0 && chosen) return SOURCES_REMOVED;
  const several = feeds.length > 1;
  const items = feeds.flatMap((f) => f.items.map((item) => (several ? { ...item, source: f.name } : item)));
  // Stable, so headlines without a date keep their feed's order at the end.
  items.sort(newestFirst);
  const maxItems = typeof settings?.maxItems === 'number' ? settings.maxItems : DEFAULT_ITEMS;
  return { title: feeds.length === 1 ? feeds[0]!.name : '', items: items.slice(0, maxItems) };
}
