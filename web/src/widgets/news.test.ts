import { describe, expect, it } from 'vitest';
import { SOURCES_REMOVED, chosenFeeds, newsView } from './news';

const nos = {
  name: 'NOS Nieuws',
  items: [
    { title: 'NOS at nine', published: '2026-10-14T09:00:00Z' },
    { title: 'NOS at seven', published: '2026-10-14T07:00:00Z' },
  ],
};
const tweakers = {
  name: 'Tweakers',
  items: [
    { title: 'Tweakers at eight', published: '2026-10-14T08:00:00Z' },
    { title: 'Tweakers without a date' },
  ],
};
const value = { '3': nos, '12': tweakers };

describe('newsView', () => {
  it('merges every feed newest first, each headline naming its feed, when the widget chose none', () => {
    const view = newsView(value, { maxItems: 3 });
    expect(view).toEqual({
      title: '',
      items: [
        { title: 'NOS at nine', published: '2026-10-14T09:00:00Z', source: 'NOS Nieuws' },
        { title: 'Tweakers at eight', published: '2026-10-14T08:00:00Z', source: 'Tweakers' },
        { title: 'NOS at seven', published: '2026-10-14T07:00:00Z', source: 'NOS Nieuws' },
      ],
    });
  });

  it('shows only the chosen feed, under its name and without labels', () => {
    expect(newsView(value, { feeds: [12] })).toEqual({ title: 'Tweakers', items: tweakers.items });
  });

  it('keeps headlines without a date at the end', () => {
    const view = newsView(value, { feeds: [3, 12] });
    const items = view !== SOURCES_REMOVED ? (view?.items ?? []) : [];
    expect(items[items.length - 1]?.title).toBe('Tweakers without a date');
  });

  it('says the sources were removed when none of the chosen feeds is there', () => {
    expect(newsView(value, { feeds: [99] })).toBe(SOURCES_REMOVED);
    // One of two still there is shown alone.
    expect(newsView(value, { feeds: [3, 99] })).toEqual({ title: 'NOS Nieuws', items: nos.items.slice(0, 5) });
  });

  it('cuts to five headlines by default', () => {
    const many = { '1': { name: 'Many', items: Array.from({ length: 8 }, (_, i) => ({ title: `H${i}` })) } };
    const view = newsView(many, undefined);
    expect(view !== SOURCES_REMOVED && view?.items.map((i) => i.title)).toEqual(['H0', 'H1', 'H2', 'H3', 'H4']);
  });

  it('ignores what is not the news entry', () => {
    expect(newsView(undefined, {})).toBeNull();
    expect(newsView([nos], {})).toBeNull();
    // The shape from before headlines were kept per feed.
    expect(newsView({ title: 'NOS', items: [] }, {})).toEqual({ title: '', items: [] });
    expect(newsView({ '1': { name: 'Broken', items: [{ title: 5 }] } }, { feeds: [1] })).toBe(SOURCES_REMOVED);
  });
});

describe('chosenFeeds', () => {
  it('reads the IDs, and no list or an empty one as every feed', () => {
    expect(chosenFeeds({ feeds: [3, 12] })).toEqual([3, 12]);
    expect(chosenFeeds({ feeds: [] })).toBeUndefined();
    expect(chosenFeeds({})).toBeUndefined();
    expect(chosenFeeds({ feeds: ['3', 1.5] })).toBeUndefined();
  });
});
