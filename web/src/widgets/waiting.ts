import { t, type TextKey } from '../i18n.svelte';
import type { Entry } from '../protocol';

/** The data sources a widget can wait for. */
export type Source =
  | 'weather'
  | 'forecast'
  | 'sun'
  | 'airquality'
  | 'waste'
  | 'holidays'
  | 'energyprice'
  | 'solar'
  | 'news'
  | 'calendar'
  | 'homeassistant'
  | 'homey'
  | 'homeypresence'
  | 'departures'
  | 'fuel'
  | 'rain'
  | 'warnings'
  | 'p2000'
  | 'traffic';

const TEXTS: Record<Source, { notSetUp: TextKey; failed: TextKey; loading: TextKey }> = {
  weather: { notSetUp: 'weatherNotSetUp', failed: 'weatherFailed', loading: 'weatherLoading' },
  // The forecast comes from the weather location.
  forecast: { notSetUp: 'weatherNotSetUp', failed: 'forecastFailed', loading: 'forecastLoading' },
  // The sun times come from the weather location too.
  sun: { notSetUp: 'weatherNotSetUp', failed: 'sunFailed', loading: 'sunLoading' },
  // The air quality is fetched for the weather location.
  airquality: { notSetUp: 'weatherNotSetUp', failed: 'airQualityFailed', loading: 'airQualityLoading' },
  // Waste pickups come from a waste calendar under Integrations.
  waste: { notSetUp: 'wasteNotSetUp', failed: 'wasteFailed', loading: 'wasteLoading' },
  // The holidays need no setup: until the server has read them, they are loading.
  holidays: { notSetUp: 'holidaysLoading', failed: 'holidaysFailed', loading: 'holidaysLoading' },
  // The price needs no setup: until the server has fetched it, it is loading.
  energyprice: { notSetUp: 'energyPriceLoading', failed: 'energyPriceFailed', loading: 'energyPriceLoading' },
  // The server sends no entry until the panels and a location are set up.
  solar: { notSetUp: 'solarNotSetUp', failed: 'solarFailed', loading: 'solarLoading' },
  news: { notSetUp: 'newsNotSetUp', failed: 'newsFailed', loading: 'newsLoading' },
  calendar: { notSetUp: 'calendarNotSetUp', failed: 'calendarFailed', loading: 'calendarLoading' },
  homeassistant: { notSetUp: 'haNotSetUp', failed: 'haFailed', loading: 'haLoading' },
  homey: { notSetUp: 'homeyNotSetUp', failed: 'homeyFailed', loading: 'homeyLoading' },
  // Who is home comes from Homey's users, which the API key may not read.
  homeypresence: { notSetUp: 'homeyNotSetUp', failed: 'homeyPresenceFailed', loading: 'homeyLoading' },
  departures: { notSetUp: 'departuresNotSetUp', failed: 'departuresFailed', loading: 'departuresLoading' },
  // Without a place the widget follows the weather location, and without
  // that the server sends no entry.
  fuel: { notSetUp: 'fuelNotSetUp', failed: 'fuelFailed', loading: 'fuelLoading' },
  // Rain follows the weather location like fuel.
  rain: { notSetUp: 'rainNotSetUp', failed: 'rainFailed', loading: 'rainLoading' },
  // Without a province the widget takes the weather location's, which
  // must be in the Netherlands.
  warnings: { notSetUp: 'warningsNotSetUp', failed: 'warningsFailed', loading: 'warningsLoading' },
  // The server fetches as soon as a widget has a region or places.
  p2000: { notSetUp: 'p2000NotSetUp', failed: 'p2000Failed', loading: 'p2000Loading' },
  // The server sends no entry until ANWB traffic info is on, or a widget
  // shows fixed cameras.
  traffic: { notSetUp: 'trafficNotSetUp', failed: 'trafficFailed', loading: 'trafficLoading' },
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
