import { roundCoordinate } from '../coordinates';
import type { Place, WidgetType } from './api';
import type { IconName } from './Icon.svelte';

/** The groups of the editor's widget palette, in the order they are shown. */
export const WIDGET_CATEGORIES = [
  { id: 'time', label: 'Time & date' },
  { id: 'weather', label: 'Weather & nature' },
  { id: 'home', label: 'Home' },
  { id: 'agenda', label: 'Agenda & household' },
  { id: 'travel', label: 'Travel' },
  { id: 'media', label: 'Media & news' },
] as const;

export type WidgetCategory = (typeof WIDGET_CATEGORIES)[number]['id'];

export interface WidgetInfo {
  label: string;
  category: WidgetCategory;
  /** One line in the palette: what the widget shows. */
  description: string;
  icon: IconName;
}

/**
 * Every widget type the editor knows, one line each, in palette order
 * within its category. A type the server has but this table lacks is
 * listed under Other by its id.
 */
export const WIDGETS: Record<string, WidgetInfo> = {
  clock: { label: 'Clock', category: 'time', description: 'The time and date, large', icon: 'clock' },
  countdown: { label: 'Countdown', category: 'time', description: 'Days left until a date', icon: 'countdown' },
  holidays: { label: 'Holidays', category: 'time', description: 'Coming school and public holidays (NL)', icon: 'holidays' },
  weather: { label: 'Weather', category: 'weather', description: 'Temperature and sky now', icon: 'weather' },
  forecast: { label: 'Forecast', category: 'weather', description: 'The coming days at a glance', icon: 'forecast' },
  sun: { label: 'Sun', category: 'weather', description: 'Sunrise and sunset today', icon: 'sun' },
  moon: { label: 'Moon', category: 'weather', description: 'The phase of the moon, and when it is full', icon: 'moon' },
  airquality: { label: 'Air quality', category: 'weather', description: 'Air quality and pollen', icon: 'air' },
  rain: { label: 'Rain', category: 'weather', description: 'Rain expected in the coming two hours', icon: 'rain' },
  warnings: { label: 'Weather warnings', category: 'weather', description: 'KNMI warnings for your province', icon: 'alert' },
  homeassistant: { label: 'Home Assistant', category: 'home', description: 'Sensors and switches at home', icon: 'house' },
  energyprice: { label: 'Energy price', category: 'home', description: "Today's electricity price", icon: 'bolt' },
  solar: { label: 'Solar forecast', category: 'home', description: 'Expected solar production today', icon: 'solar' },
  custom: { label: 'Custom API', category: 'home', description: 'Values from an API under Integrations', icon: 'braces' },
  homey: { label: 'Homey', category: 'home', description: 'Device values from your Homey', icon: 'house' },
  homeypresence: { label: 'Who’s home (Homey)', category: 'home', description: 'Who is home, from your Homey', icon: 'people' },
  camera: { label: 'Camera', category: 'home', description: 'A still from a camera at home', icon: 'camera' },
  calendar: { label: 'Calendar', category: 'agenda', description: 'Your next appointments', icon: 'calendar' },
  waste: { label: 'Waste collection', category: 'agenda', description: 'The next waste pickups', icon: 'bin' },
  text: { label: 'Note', category: 'agenda', description: 'A short note for the household', icon: 'note' },
  departures: { label: 'Departures', category: 'travel', description: 'Next buses and trains', icon: 'bus' },
  fuel: { label: 'Fuel prices', category: 'travel', description: 'The cheapest fuel nearby', icon: 'fuel' },
  news: { label: 'News', category: 'media', description: 'Headlines from your feeds', icon: 'news' },
  photos: { label: 'Photos', category: 'media', description: 'A slideshow of your photos', icon: 'photo' },
  nowplaying: { label: 'Now playing', category: 'media', description: 'What plays on your Cast speakers', icon: 'music' },
  webpage: { label: 'Web page', category: 'media', description: 'A page from Integrations, such as Grafana', icon: 'browser' },
};

/** Names of the widget types; unknown types show their id. */
export const WIDGET_LABELS: Record<string, string> = Object.fromEntries(Object.entries(WIDGETS).map(([type, w]) => [type, w.label]));

export interface PaletteItem {
  type: string;
  label: string;
  description: string;
  icon: IconName;
}

export interface PaletteGroup {
  label: string;
  items: PaletteItem[];
}

/**
 * The palette for the types this server has, narrowed to those whose name
 * or description holds every word of the search. Empty groups are left out.
 */
export function paletteGroups(types: string[], query = ''): PaletteGroup[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const matches = (item: PaletteItem) => words.every((word) => `${item.label} ${item.description}`.toLowerCase().includes(word));
  const item = (type: string): PaletteItem => {
    const w = WIDGETS[type];
    return { type, label: w?.label ?? type, description: w?.description ?? '', icon: w?.icon ?? 'widget' };
  };
  const known = Object.keys(WIDGETS).filter((type) => types.includes(type));
  const groups: PaletteGroup[] = [
    ...WIDGET_CATEGORIES.map((c) => ({ label: c.label, items: known.filter((type) => WIDGETS[type]!.category === c.id).map(item) })),
    { label: 'Other', items: types.filter((type) => !WIDGETS[type]).map(item) },
  ];
  return groups.map((g) => ({ ...g, items: g.items.filter(matches) })).filter((g) => g.items.length > 0);
}

export const SETTING_LABELS: Record<string, string> = {
  maxItems: 'How many at most',
  days: 'Days',
  text: 'Text',
  label: 'Label',
  date: 'Date',
  seconds: 'Seconds per photo',
  calendar: 'Calendar',
  stop: 'Stop',
  radius: 'Radius (km)',
  fuel: 'Fuel',
  region: 'Region',
  show: 'Show',
  announce: 'Announce code orange and red on every screen',
  fit: 'Fit',
  source: 'Photos from',
  zoom: 'Zoom (%)',
};

/** What an optional choice says when nothing is chosen, per setting; a choice without one always has a value. */
export const CHOICE_UNSET: Record<string, string> = {
  region: 'From the weather location',
};

// What "how many" counts, per widget: a setting named after what the
// household sees on the TV rather than after the field.
const ITEM_NAMES: Record<string, string> = {
  calendar: 'Appointments',
  news: 'Headlines',
  departures: 'Departures',
  fuel: 'Petrol stations',
  holidays: 'Holidays',
};

/** The label of a setting in the editor, worded for the widget it belongs to. */
export function settingLabel(widgetType: string, key: string): string {
  if (key === 'maxItems' && ITEM_NAMES[widgetType]) return `${ITEM_NAMES[widgetType]} at most`;
  if (key === 'days' && widgetType === 'forecast') return 'Days ahead';
  if (key === 'region') return widgetType === 'holidays' ? 'School holiday region' : widgetType === 'warnings' ? 'Province' : 'Region';
  if (key === 'seconds' && widgetType === 'camera') return 'Refresh every (seconds)';
  return SETTING_LABELS[key] ?? key;
}

/** Names of the options of choice settings in the editor: the fuels by the ANWB's codes, the holiday choices, and how photos and camera images fit. */
export const CHOICE_LABELS: Record<string, string> = {
  EURO95: 'Euro 95 / E10',
  EURO98: 'Euro 98',
  DIESEL: 'Diesel',
  AUTOGAS: 'LPG',
  CNG: 'CNG',
  north: 'North',
  middle: 'Middle',
  south: 'South',
  both: 'School and public holidays',
  school: 'School holidays',
  public: 'Public holidays',
  contain: 'Whole image',
  cover: 'Fill the tile',
  uploaded: 'Uploaded photos',
  immich: 'Immich album',
  synology: 'Synology Photos album',
};

/** How often a new Camera widget refreshes, in seconds; internal/display holds the same default. */
export const CAMERA_SECONDS = 10;

const COUNTDOWN_DAYS = 30;
/** The longest place name the server accepts. */
const MAX_PLACE_NAME = 100;

/** The settings a new widget starts with: every setting it takes, filled in. */
export function defaultSettings(type: WidgetType, today = new Date()): Record<string, unknown> {
  const later = new Date(today.getFullYear(), today.getMonth(), today.getDate() + COUNTDOWN_DAYS);
  const defaults: Record<string, unknown> = {
    maxItems: 5,
    days: 5,
    seconds: 30,
    text: 'Write a note for the household here.',
    label: 'Holiday',
    date: isoDate(later),
    entities: [],
    capabilities: [],
    users: [],
    radius: 5,
    fuel: 'EURO95',
    // The middle region has most schools; the editor does not guess the
    // region from the weather location, as the borders follow
    // municipalities, not latitude.
    region: 'middle',
    show: 'both',
    announce: false,
    fit: 'contain',
    source: 'uploaded',
  };
  // stop and stopName stay unset until a stop is chosen: there is no sensible default.
  // A Custom widget starts as one value; its API and path are chosen in its form.
  if (type.settings.source?.kind === 'customSource') return { mode: 'value' };
  // A fuel widget without a place follows the weather location.
  const settings: Record<string, unknown> = {};
  for (const [key, rule] of Object.entries(type.settings)) {
    // A default choice only where the widget offers it: a holiday region is
    // not a province.
    if (key in defaults && (rule.kind !== 'choice' || rule.options?.includes(defaults[key] as string))) settings[key] = defaults[key];
  }
  // A camera is asked for a new image more often than a photo changes.
  if (type.settings.camera?.kind === 'camera') settings.seconds = CAMERA_SECONDS;
  return settings;
}

/** What a place setting stores for a search result: its name and its place rounded to about 100 m. */
export function placeSetting(place: Place): { name: string; latitude: number; longitude: number } {
  return { name: place.name.slice(0, MAX_PLACE_NAME), latitude: roundCoordinate(place.latitude), longitude: roundCoordinate(place.longitude) };
}

/** A local date as YYYY-MM-DD, the format date settings use. */
export function isoDate(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}
