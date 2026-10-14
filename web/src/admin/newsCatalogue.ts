import type { NewsCatalogueEntry, NewsFeed } from './types';

// The picker of built-in news sources under Integrations, News feeds: the
// catalogue grouped by country (the Netherlands first) and category, and
// narrowed by a search.

/** Names of the countries in the catalogue; one that lacks a name shows its code. */
export const COUNTRY_NAMES: Record<string, string> = {
  NL: 'Netherlands',
  BE: 'Belgium',
  GB: 'United Kingdom',
  DE: 'Germany',
  FR: 'France',
  US: 'United States',
  QA: 'Qatar',
};

/** Names of the categories, which internal/news holds as Categories. */
export const CATEGORY_LABELS: Record<string, string> = {
  general: 'General',
  international: 'International',
  regional: 'Regional',
  sport: 'Sport',
  economy: 'Economy',
  tech: 'Tech',
  science: 'Science',
  culture: 'Culture & media',
  offbeat: 'Offbeat',
  kids: 'Children',
};

const LANGUAGE_NAMES: Record<string, string> = { nl: 'Dutch', en: 'English', de: 'German', fr: 'French' };

const HOME_COUNTRY = 'NL';

export interface CategoryGroup {
  category: string;
  label: string;
  sources: NewsCatalogueEntry[];
}

export interface CountryGroup {
  country: string;
  label: string;
  categories: CategoryGroup[];
}

const countryName = (code: string) => COUNTRY_NAMES[code] ?? code;
const categoryLabel = (id: string) => CATEGORY_LABELS[id] ?? id;

/** What a search can match in an entry: its name, country, category and language, in any case. */
function searchText(e: NewsCatalogueEntry): string {
  return `${e.name} ${countryName(e.country)} ${e.country} ${categoryLabel(e.category)} ${LANGUAGE_NAMES[e.language] ?? e.language}`.toLowerCase();
}

/**
 * The catalogue as the picker shows it: by country, the Netherlands first
 * and then in catalogue order, and within a country by category in the
 * server's order, narrowed to the entries that hold every word of query.
 * Empty groups are left out.
 */
export function catalogueGroups(sources: NewsCatalogueEntry[], categories: string[], query = ''): CountryGroup[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const shown = sources.filter((e) => words.every((w) => searchText(e).includes(w)));
  const countries = [...new Set(shown.map((e) => e.country))].sort((a, b) => Number(b === HOME_COUNTRY) - Number(a === HOME_COUNTRY));
  const order = [...categories, ...new Set(shown.map((e) => e.category).filter((c) => !categories.includes(c)))];
  return countries.map((country) => ({
    country,
    label: countryName(country),
    categories: order
      .map((category) => ({ category, label: categoryLabel(category), sources: shown.filter((e) => e.country === country && e.category === category) }))
      .filter((g) => g.sources.length > 0),
  }));
}

/** The chosen feed that is this catalogue entry, by its address. */
export function chosenFeed(feeds: NewsFeed[], entry: NewsCatalogueEntry): NewsFeed | undefined {
  return feeds.find((f) => f.url === entry.url);
}

/** What the list of chosen sources says under a name: where a built-in source is from, or the address of one's own. */
export function feedDetail(feed: NewsFeed, sources: NewsCatalogueEntry[]): string {
  const entry = sources.find((e) => e.url === feed.url);
  return entry ? `${countryName(entry.country)} · ${categoryLabel(entry.category)}` : feed.url;
}
