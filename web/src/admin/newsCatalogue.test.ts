import { describe, expect, it } from 'vitest';
import { catalogueGroups, chosenFeed, feedDetail } from './newsCatalogue';
import type { NewsCatalogueEntry } from './types';

const entry = (id: string, name: string, country: string, category: string, language = 'nl'): NewsCatalogueEntry => ({
  id,
  name,
  country,
  category,
  language,
  url: `https://example.org/${id}`,
});

const CATEGORIES = ['general', 'regional', 'sport', 'tech'];
const SOURCES = [
  entry('bbc-world', 'BBC News World', 'GB', 'general', 'en'),
  entry('nos-sport', 'NOS Sport', 'NL', 'sport'),
  entry('nos', 'NOS Nieuws', 'NL', 'general'),
  entry('tweakers', 'Tweakers', 'NL', 'tech'),
  entry('vrt', 'VRT NWS', 'BE', 'general'),
  entry('l1', 'L1 (Limburg)', 'NL', 'regional'),
];

const names = (groups: ReturnType<typeof catalogueGroups>) =>
  groups.map((g) => `${g.label}: ${g.categories.map((c) => `${c.label} (${c.sources.map((s) => s.name).join(', ')})`).join('; ')}`);

describe('catalogueGroups', () => {
  it('puts the Netherlands first, then the countries in catalogue order, each by category', () => {
    expect(names(catalogueGroups(SOURCES, CATEGORIES))).toEqual([
      'Netherlands: General (NOS Nieuws); Regional (L1 (Limburg)); Sport (NOS Sport); Tech (Tweakers)',
      'United Kingdom: General (BBC News World)',
      'Belgium: General (VRT NWS)',
    ]);
  });

  it('finds sources by name, country, category or language, every word in any case', () => {
    expect(names(catalogueGroups(SOURCES, CATEGORIES, 'nos'))).toEqual(['Netherlands: General (NOS Nieuws); Sport (NOS Sport)']);
    expect(names(catalogueGroups(SOURCES, CATEGORIES, 'belgium'))).toEqual(['Belgium: General (VRT NWS)']);
    expect(names(catalogueGroups(SOURCES, CATEGORIES, 'Sport'))).toEqual(['Netherlands: Sport (NOS Sport)']);
    expect(names(catalogueGroups(SOURCES, CATEGORIES, 'english'))).toEqual(['United Kingdom: General (BBC News World)']);
    expect(names(catalogueGroups(SOURCES, CATEGORIES, '  general   netherlands '))).toEqual(['Netherlands: General (NOS Nieuws)']);
  });

  it('leaves out what nothing matches', () => {
    expect(catalogueGroups(SOURCES, CATEGORIES, 'nothing like this')).toEqual([]);
  });

  it('keeps a category the server added after the known ones, by its id', () => {
    const odd = [entry('x', 'Weather Today', 'NL', 'weather')];
    expect(names(catalogueGroups(odd, CATEGORIES))).toEqual(['Netherlands: weather (Weather Today)']);
  });
});

describe('chosen sources', () => {
  const feeds = [
    { id: 4, name: 'NOS Nieuws', url: 'https://example.org/nos' },
    { id: 9, name: 'Mine', url: 'https://mine.example.org/rss' },
  ];

  it('finds the chosen feed of an entry by its address', () => {
    expect(chosenFeed(feeds, SOURCES[2]!)?.id).toBe(4);
    expect(chosenFeed(feeds, SOURCES[3]!)).toBeUndefined();
  });

  it('describes a built-in source by where it is from and one of your own by its address', () => {
    expect(feedDetail(feeds[0]!, SOURCES)).toBe('Netherlands · General');
    expect(feedDetail(feeds[1]!, SOURCES)).toBe('https://mine.example.org/rss');
  });
});
