import type { CustomSource, NamedList, NamedValue } from './api';
import { pairs, type Pair } from './customWidget';
import type { CategoryId } from './integrations';
import { PRESETS, madeFrom, presetOf, type Preset, type PresetView } from './presets';
import type { PaletteEntry, WidgetCategory } from './widgetSettings';

// The widgets of the presets in the dashboard editor. A preset's widget is
// stored as a Custom widget like any other, so exports and the server do
// not change; the editor knows it by the preset of its source and offers
// the preset's own name, views and values instead of paths.

/** Where a preset's widget is in the palette, by the preset's category on the Integrations page. */
const PALETTE_CATEGORY: Record<CategoryId, WidgetCategory> = {
  weather: 'weather',
  travel: 'travel',
  energy: 'home',
  home: 'home',
  network: 'home',
  lists: 'agenda',
  media: 'media',
  news: 'media',
  own: 'home',
};

/** A preset's named value or list by name. */
function namedValue(preset: Preset, name: string): NamedValue | undefined {
  return preset.values.find((v) => v.name === name);
}

function namedList(preset: Preset, name: string | undefined): NamedList | undefined {
  return preset.lists.find((l) => l.name === name);
}

/** The settings of a Custom widget that shows view of the preset from source. */
export function viewSettings(preset: Preset, view: PresetView, source: number): Record<string, unknown> {
  const settings: Record<string, unknown> = { source, mode: view.mode };
  if (view.title) settings.title = view.title;
  if (view.mode === 'value') {
    const shown = view.values?.[0];
    const value = shown && namedValue(preset, shown.name);
    if (value) {
      settings.path = value.path;
      if (shown.label) settings.label = shown.label;
      if (value.unit) settings.unit = value.unit;
      if (value.decimals !== undefined) settings.decimals = value.decimals;
    }
  } else if (view.mode === 'list') {
    const list = namedList(preset, view.list);
    if (list) {
      settings.path = list.path;
      settings.fields = list.columns.map((c) => c.path);
    }
  } else {
    settings.pairs = (view.values ?? []).flatMap((shown) => {
      const value = namedValue(preset, shown.name);
      return value ? [{ label: shown.label ?? shown.name, path: value.path }] : [];
    });
  }
  if (view.rules && view.rules.length > 0) settings.rules = view.rules.map((r) => ({ ...r }));
  return settings;
}

/** The view a widget shows now: the one of its mode that shows the same value or list, else the first of its mode. */
export function viewOf(preset: Preset, settings: Record<string, unknown>): PresetView | undefined {
  const sameMode = preset.views.filter((v) => v.mode === settings.mode);
  const path = settings.path;
  const same = sameMode.find((v) => {
    if (v.mode === 'value') return namedValue(preset, v.values?.[0]?.name ?? '')?.path === path;
    if (v.mode === 'list') return namedList(preset, v.list)?.path === path;
    return true;
  });
  return same ?? sameMode[0];
}

/** A Custom widget's preset, and the sources it can show, when its source was set up with one. */
export function presetWidget(settings: Record<string, unknown>, sources: CustomSource[]): { preset: Preset; sources: CustomSource[] } | undefined {
  const source = sources.find((s) => s.id === settings.source);
  const preset = presetOf(source);
  return preset ? { preset, sources: madeFrom(preset, sources) } : undefined;
}

/** What a preset's widget can show, for its line in the palette: "Power now or meter readings". */
export function describeViews(preset: Preset): string {
  const names = preset.views.map((v, i) => (i === 0 ? v.name : v.name.charAt(0).toLowerCase() + v.name.slice(1)));
  return names.length > 1 ? `${names.slice(0, -1).join(', ')} or ${names[names.length - 1]}` : names[0]!;
}

/** The palette's entry for each preset that is set up, in its category. */
export function presetPaletteEntries(sources: CustomSource[]): PaletteEntry[] {
  return PRESETS.filter((p) => madeFrom(p, sources).length > 0).map((p) => ({
    key: `preset-${p.id}`,
    type: 'custom',
    preset: p.id,
    label: p.name,
    description: describeViews(p),
    icon: p.icon,
    category: PALETTE_CATEGORY[p.category],
  }));
}

/** A new widget of a preset: its first view, from its first setup, at the view's size or the type's smallest. */
export function newPresetWidget(
  preset: Preset,
  sources: CustomSource[],
  smallest: { w: number; h: number },
): { settings: Record<string, unknown>; w: number; h: number } | undefined {
  const source = madeFrom(preset, sources)[0];
  const view = preset.views[0];
  if (!source || !view) return undefined;
  return { settings: viewSettings(preset, view, source.id), w: Math.max(view.w, smallest.w), h: Math.max(view.h, smallest.h) };
}

/**
 * The pairs after a named value is turned on or off, in the preset's
 * order. A value keeps the label it has; a new one is labelled with its name.
 */
export function togglePair(preset: Preset, current: unknown, value: NamedValue, on: boolean): Pair[] {
  const now = pairs(current);
  const labelOf = (path: string, name: string) => now.find((p) => p.path === path)?.label || name;
  const shown = new Set(now.map((p) => p.path));
  if (on) shown.add(value.path);
  else shown.delete(value.path);
  // A pair of another path, such as from an import, stays at the end.
  const known = preset.values.filter((v) => shown.has(v.path)).map((v) => ({ label: labelOf(v.path, v.name), path: v.path }));
  const other = now.filter((p) => !preset.values.some((v) => v.path === p.path));
  return [...known, ...other];
}
