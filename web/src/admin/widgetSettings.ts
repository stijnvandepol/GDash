import { roundCoordinate } from '../coordinates';
import type { Place, WidgetType } from './api';

/** Names for the widget picker; unknown types show their id. */
export const WIDGET_LABELS: Record<string, string> = {
  clock: 'Clock',
  weather: 'Weather',
  forecast: 'Forecast',
  sun: 'Sun',
  airquality: 'Air quality',
  waste: 'Waste collection',
  energyprice: 'Energy price',
  calendar: 'Calendar',
  news: 'News',
  text: 'Note',
  countdown: 'Countdown',
  photos: 'Photos',
  homeassistant: 'Home Assistant',
  homey: 'Homey',
  homeypresence: 'Who’s home (Homey)',
  departures: 'Departures',
  fuel: 'Fuel prices',
  custom: 'Custom API',
};

export const SETTING_LABELS: Record<string, string> = {
  maxItems: 'Items',
  days: 'Days',
  text: 'Text',
  label: 'Label',
  date: 'Date',
  seconds: 'Seconds per photo',
  calendar: 'Calendar',
  stop: 'Stop',
  radius: 'Radius (km)',
  fuel: 'Fuel',
};

/** Names of the fuels in the editor, by the ANWB's codes. */
export const FUEL_LABELS: Record<string, string> = {
  EURO95: 'Euro 95 / E10',
  EURO98: 'Euro 98',
  DIESEL: 'Diesel',
  AUTOGAS: 'LPG',
  CNG: 'CNG',
};

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
  };
  // stop and stopName stay unset until a stop is chosen: there is no sensible default.
  // A Custom widget starts as one value; its API and path are chosen in its form.
  if (type.settings.source?.kind === 'customSource') return { mode: 'value' };
  // A fuel widget without a place follows the weather location.
  const settings: Record<string, unknown> = {};
  for (const key of Object.keys(type.settings)) {
    if (key in defaults) settings[key] = defaults[key];
  }
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
