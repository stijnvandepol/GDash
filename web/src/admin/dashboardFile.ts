import { slugify, type Dashboard, type Widget } from './api';

// Marks a file as an exported GDash dashboard, so importing any other
// JSON file fails with a clear message instead of a list of field errors.
const FORMAT = 'gdash-dashboard';
// Files exported under the project's earlier names, Daycast and Nookboard.
const OLD_FORMATS = ['daycast-dashboard', 'nookboard-dashboard'];
const VERSION = 1;
// The server accepts request bodies up to 64 KiB.
const MAX_FILE_BYTES = 64 * 1024;
const MAX_ID_LENGTH = 32;

/** A dashboard as the server will create it from an imported file. */
export interface ImportedDashboard {
  name: string;
  theme?: string;
  font?: string;
  nightTheme?: string;
  widgets: Widget[];
}

/**
 * The file for an exported dashboard: its name, theme, font, night theme and widgets. The
 * widgets' data sources are left out; the server sets them from the type.
 */
export function toFile(d: Dashboard): string {
  const widgets = d.widgets.map(({ id, type, x, y, w, h, settings }) => ({ id, type, x, y, w, h, settings }));
  return JSON.stringify(
    { format: FORMAT, version: VERSION, name: d.name, theme: d.theme, font: d.font || undefined, nightTheme: d.nightTheme || undefined, widgets },
    null,
    2,
  );
}

/**
 * Reads an exported dashboard. It checks the file's shape only; the server
 * validates the layout itself when the dashboard is created. Throws an
 * Error with a message for the user.
 */
export function fromFile(text: string): ImportedDashboard {
  if (new Blob([text]).size > MAX_FILE_BYTES) throw new Error('This file is too large for a dashboard.');
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    throw new Error('This is not a GDash dashboard file.');
  }
  const d = data as Record<string, unknown>;
  if (typeof d !== 'object' || d === null || (d.format !== FORMAT && !OLD_FORMATS.includes(String(d.format)))) {
    throw new Error('This is not a GDash dashboard file.');
  }
  if (d.version !== VERSION) throw new Error('This dashboard file comes from a newer GDash. Update GDash first.');
  if (typeof d.name !== 'string' || !Array.isArray(d.widgets)) throw new Error('This dashboard file is damaged.');
  return {
    name: d.name,
    theme: typeof d.theme === 'string' ? d.theme : undefined,
    font: typeof d.font === 'string' ? d.font : undefined,
    nightTheme: typeof d.nightTheme === 'string' ? d.nightTheme : undefined,
    widgets: d.widgets as Widget[],
  };
}

/** An id for name that no dashboard in taken uses yet: "evening", "evening-2", … */
export function freeId(name: string, taken: string[]): string {
  const base = slugify(name) || 'dashboard';
  if (!taken.includes(base)) return base;
  for (let n = 2; ; n++) {
    const suffix = `-${n}`;
    const id = base.slice(0, MAX_ID_LENGTH - suffix.length) + suffix;
    if (!taken.includes(id)) return id;
  }
}
