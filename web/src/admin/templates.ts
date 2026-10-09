import type { Dashboard, DashboardTemplate, Widget } from './api';
import { freeId } from './dashboardFile';
import { WIDGET_LABELS } from './widgetSettings';

/** One widget of a template, drawn as a box inside the thumbnail; sizes are percentages. */
export interface ThumbnailBox {
  id: string;
  label: string;
  left: number;
  top: number;
  width: number;
  height: number;
}

/** The dashboard to create from a template: its layout under a new, free id. */
export function fromTemplate(template: DashboardTemplate, takenIds: string[]): Dashboard {
  return {
    id: freeId(template.name, takenIds),
    name: template.name,
    theme: template.theme,
    widgets: template.widgets.map(({ id, type, x, y, w, h, settings }) => ({ id, type, x, y, w, h, settings })),
  };
}

/** The boxes that draw widgets on a grid of columns x rows as a small picture. */
export function thumbnailBoxes(widgets: Widget[], columns: number, rows: number): ThumbnailBox[] {
  return widgets.map((w) => ({
    id: w.id,
    label: WIDGET_LABELS[w.type] ?? w.type,
    left: (w.x / columns) * 100,
    top: (w.y / rows) * 100,
    width: (w.w / columns) * 100,
    height: (w.h / rows) * 100,
  }));
}
