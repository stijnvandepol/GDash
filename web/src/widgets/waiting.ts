import { t, type TextKey } from '../i18n.svelte';
import type { Entry } from '../protocol';

/** The data sources a widget can wait for. */
export type Source = 'weather' | 'forecast' | 'news' | 'calendar' | 'homeassistant';

const TEXTS: Record<Source, { notSetUp: TextKey; failed: TextKey; loading: TextKey }> = {
  weather: { notSetUp: 'weatherNotSetUp', failed: 'weatherFailed', loading: 'weatherLoading' },
  // The forecast comes from the weather location.
  forecast: { notSetUp: 'weatherNotSetUp', failed: 'forecastFailed', loading: 'forecastLoading' },
  news: { notSetUp: 'newsNotSetUp', failed: 'newsFailed', loading: 'newsLoading' },
  calendar: { notSetUp: 'calendarNotSetUp', failed: 'calendarFailed', loading: 'calendarLoading' },
  homeassistant: { notSetUp: 'haNotSetUp', failed: 'haFailed', loading: 'haLoading' },
};

/**
 * What a widget shows while it has nothing to show. The server sends no
 * entry for a source that is not set up, and an entry without a value while
 * the first fetch runs or fails.
 */
export function waitingText(entry: Entry | undefined, source: Source): string {
  const texts = TEXTS[source];
  if (entry === undefined) return t(texts.notSetUp);
  return t(entry.failing ? texts.failed : texts.loading);
}
