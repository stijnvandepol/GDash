// The title over a tile, which says what the tile shows: Who's home over
// a row of initials, Waste collection over a list of bins. One title, drawn
// by App.svelte the same way for every widget. A widget that explains
// itself, such as the clock, has none unless the admin types one; the
// setting hideTitle turns it off.
import type { TextKey } from '../i18n.svelte';
import type { Entry } from '../protocol';
import { SOURCES_REMOVED, newsView } from './news';
import { FUEL_TEXT, fuelOf, readPlace } from './fuel';
import { radiusOf, roadsOf } from './traffic';
import { readArea, warningsKey } from './warnings';
import { shownFor } from './custom';

/** A text in the screen's language, as t of i18n.svelte.ts gives it. */
export type Text = (key: TextKey, values?: Record<string, string | number>) => string;

export interface TitledWidget {
  type: string;
  settings?: Record<string, unknown>;
}

const SEPARATOR = ' · ';

/** The widgets whose title is one text. */
const FIXED_TITLES: Partial<Record<string, TextKey>> = {
  airquality: 'airQuality',
  waste: 'wasteTitle',
  energyprice: 'energyPriceTitle',
  solar: 'solarTitle',
  holidays: 'holidaysTitle',
  moon: 'moonTitle',
  homeypresence: 'whoIsHome',
  nowplaying: 'nowPlayingTitle',
  camera: 'cameraTitle',
};

/** The widgets that show values of a smart home system, named after it in every language. */
const SYSTEM_TITLES: Partial<Record<string, string>> = {
  homeassistant: 'Home Assistant',
  homey: 'Homey',
};

function textOf(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

function joined(...parts: (string | undefined)[]): string {
  return parts.filter((p): p is string => typeof p === 'string' && p !== '').join(SEPARATOR);
}

/** The name of the web page a Web page widget shows, from the snapshot. */
function pageName(entry: Entry | undefined, settings: Record<string, unknown> | undefined): string {
  const value = entry?.value;
  const id = settings?.page;
  if (typeof id !== 'number' || typeof value !== 'object' || value === null) return '';
  const page = (value as Record<string, unknown>)[String(id)];
  return typeof page === 'object' && page !== null ? textOf((page as Record<string, unknown>).name) : '';
}

/** The title a widget has unless the admin typed one, or null for none. */
export function defaultTitle(widget: TitledWidget, entry: Entry | undefined, text: Text): string | null {
  const settings = widget.settings;
  switch (widget.type) {
    case 'news': {
      const view = newsView(entry?.value, settings);
      return (view && view !== SOURCES_REMOVED && view.title) || text('news');
    }
    case 'departures':
      return textOf(settings?.stopName) || textOf(settings?.stop) || text('departuresTitle');
    case 'fuel':
      return joined(text('fuelTitle'), readPlace(settings?.place)?.name, text(FUEL_TEXT[fuelOf(settings)]!));
    case 'rain':
      return joined(text('rainTitle'), readPlace(settings?.place)?.name);
    case 'warnings':
      return joined(text('warningsTitle'), readArea(entry?.value, warningsKey(settings))?.region);
    case 'traffic': {
      const roads = roadsOf(settings);
      return joined(text('trafficTitle'), roads.length > 0 ? roads.join(', ') : text('trafficWithin', { km: radiusOf(settings) }));
    }
    case 'webpage':
      return pageName(entry, settings) || text('webPageTitle');
    case 'custom':
      // A Custom widget's own title is its setting title; badges are
      // mostly who is home.
      return settings?.mode === 'badges' ? text('whoIsHome') : null;
  }
  const system = SYSTEM_TITLES[widget.type];
  if (system) return system;
  const key = FIXED_TITLES[widget.type];
  return key ? text(key) : null;
}

/** The title over the tile: the admin's own, else the default, or null when hidden or there is none. */
export function tileTitle(widget: TitledWidget, entry: Entry | undefined, text: Text): string | null {
  if (widget.settings?.hideTitle === true) return null;
  return textOf(widget.settings?.title) || defaultTitle(widget, entry, text);
}

/**
 * Whether the title is drawn in the warning colour, as each widget did with
 * its own: the data is older than it should be. A Custom widget's entry is
 * built for every screen anew, so its own state says it; key is its
 * CustomKey.
 */
export function titleIsStale(widget: TitledWidget, entry: Entry | undefined, key: string, nowMs: number): boolean {
  if (widget.type === 'custom') {
    const shown = shownFor(entry, key);
    return shown?.stale === true || shown?.failing === true;
  }
  return entry !== undefined && nowMs > Date.parse(entry.staleAfter);
}

// Below these the data needs every pixel; a title would crowd it.
const MIN_TITLED_HEIGHT = 90;
const MIN_TITLED_WIDTH = 140;

/**
 * Whether a tile with this content box, in stage pixels, has room for a
 * title. Every title is drawn the same size, so a row of tiles reads as
 * one; a tile without room leaves it out.
 */
export function titleFits(content: { width: number; height: number }): boolean {
  return content.height >= MIN_TITLED_HEIGHT && content.width >= MIN_TITLED_WIDTH;
}

export interface Spacing {
  /** The gap between tiles and around the grid. */
  gap: number;
  /** The padding inside a tile. */
  padding: number;
}

/**
 * The content box of a tile w by h cells, in stage pixels, on a stage of
 * this size with a grid of columns by rows, as App.svelte lays it out.
 */
export function tileContent(
  stage: { width: number; height: number },
  grid: { columns: number; rows: number },
  cells: { w: number; h: number },
  space: Spacing,
): { width: number; height: number } {
  const track = (length: number, count: number, span: number) => {
    const cell = (length - 2 * space.gap - (count - 1) * space.gap) / count;
    return cell * span + space.gap * (span - 1) - 2 * space.padding;
  };
  return { width: track(stage.width, grid.columns, cells.w), height: track(stage.height, grid.rows, cells.h) };
}
