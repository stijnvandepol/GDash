import type { WidgetType } from './api';

/** Names for the widget picker; unknown types show their id. */
export const WIDGET_LABELS: Record<string, string> = {
  clock: 'Clock',
  weather: 'Weather',
  forecast: 'Forecast',
  sun: 'Sun',
  airquality: 'Air quality',
  calendar: 'Calendar',
  news: 'News',
  text: 'Note',
  countdown: 'Countdown',
  photos: 'Photos',
  homeassistant: 'Home Assistant',
};

export const SETTING_LABELS: Record<string, string> = {
  maxItems: 'Items',
  days: 'Days',
  text: 'Text',
  label: 'Label',
  date: 'Date',
  seconds: 'Seconds per photo',
};

const COUNTDOWN_DAYS = 30;

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
  };
  const settings: Record<string, unknown> = {};
  for (const key of Object.keys(type.settings)) {
    if (key in defaults) settings[key] = defaults[key];
  }
  return settings;
}

/** A local date as YYYY-MM-DD, the format date settings use. */
export function isoDate(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}
