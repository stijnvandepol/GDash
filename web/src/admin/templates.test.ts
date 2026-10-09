import { describe, expect, it } from 'vitest';
import type { DashboardTemplate } from './api';
import { fromTemplate, thumbnailBoxes } from './templates';

const kitchen: DashboardTemplate = {
  id: 'kitchen',
  name: 'Kitchen',
  description: 'Clock and news.',
  theme: 'warm-light',
  widgets: [
    { id: 'clock', type: 'clock', x: 0, y: 0, w: 12, h: 7 },
    { id: 'news', type: 'news', x: 12, y: 7, w: 12, h: 7, settings: { maxItems: 5 } },
  ],
};

describe('fromTemplate', () => {
  it('takes name, theme and widgets and picks a free id', () => {
    const d = fromTemplate(kitchen, ['default']);
    expect(d).toEqual({ id: 'kitchen', name: 'Kitchen', theme: 'warm-light', widgets: kitchen.widgets });
  });

  it('numbers the id when the template was used before', () => {
    expect(fromTemplate(kitchen, ['kitchen']).id).toBe('kitchen-2');
    expect(fromTemplate({ ...kitchen, name: 'Bedroom / night' }, []).id).toBe('bedroom-night');
  });

  it('does not carry over fields the server sets itself', () => {
    // The server sends each widget's data source too; the create request leaves it out.
    const served = { id: 'news', type: 'news', x: 0, y: 0, w: 6, h: 4, source: 'news.default' };
    const created = fromTemplate({ ...kitchen, widgets: [served] }, []);
    expect(Object.keys(created.widgets[0] ?? {})).not.toContain('source');
  });
});

describe('thumbnailBoxes', () => {
  it('scales widgets to percentages of the grid and labels them', () => {
    expect(thumbnailBoxes(kitchen.widgets, 24, 14)).toEqual([
      { id: 'clock', label: 'Clock', left: 0, top: 0, width: 50, height: 50 },
      { id: 'news', label: 'News', left: 50, top: 50, width: 50, height: 50 },
    ]);
  });

  it('shows the type of a widget it has no name for', () => {
    expect(thumbnailBoxes([{ id: 'x', type: 'tides', x: 0, y: 0, w: 4, h: 4 }], 24, 14)[0]?.label).toBe('tides');
  });
});
