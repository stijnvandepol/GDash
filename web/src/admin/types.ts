// Shapes of the data the GDash API sends and receives. api.ts re-exports
// them, so components import everything from './api'.
export interface User {
  id: number;
  username: string;
}

export interface Display {
  id: string;
  name: string;
  castDeviceId: string;
  createdAt: string;
  /** Screens paired by code; only present in the display list. */
  pairedScreens?: number;
  /** Empty means the default dashboard. */
  dashboardId: string;
  /** Active hours as HH:MM; both empty means always on. */
  activeFrom: string;
  activeUntil: string;
  /** Other dashboards during parts of the day; the first matching slot wins. */
  schedule: Slot[];
  /** Dashboards shown in turn with the display's own, each for rotateSeconds. */
  rotation: string[];
  rotateSeconds: number;
  /** Text size in percent: 100, 125, 150 or 200. */
  textScale: number;
  /** What GDash last found on the Cast device; only in the display list. */
  cast?: CastStatus;
}

export interface CastStatus {
  state: 'connecting' | 'unreachable' | 'showing' | 'standby' | 'other_app' | 'resting';
  since: string;
  /** Why the device cannot be reached. */
  problem?: string;
}

/** A dashboard shown from one time of day until another, as HH:MM. */
export interface Slot {
  from: string;
  until: string;
  /** mon to sun; empty means every day. */
  days: string[];
  dashboardId: string;
}

export interface Widget {
  id: string;
  type: string;
  x: number;
  y: number;
  w: number;
  h: number;
  settings?: Record<string, unknown>;
}

export interface Dashboard {
  id: string;
  name: string;
  theme?: string;
  /** A typeface instead of the theme's own; empty for the theme's. */
  font?: string;
  /** The theme to switch to between sunset and sunrise at the weather location; empty keeps the theme. */
  nightTheme?: string;
  widgets: Widget[];
}

/** A ready-made layout to start a dashboard from. */
export interface DashboardTemplate {
  id: string;
  name: string;
  description: string;
  theme: string;
  widgets: Widget[];
}

export interface WidgetCatalogue {
  columns: number;
  rows: number;
  themes: string[];
  fonts: string[];
  types: Record<string, WidgetType>;
}

export interface WidgetType {
  minW: number;
  minH: number;
  settings: Record<string, SettingRule>;
}

/** A setting a widget type takes; internal/display validates the same. */
export interface SettingRule {
  kind: 'integer' | 'text' | 'date' | 'entities';
  /** Bounds of an integer, of a text's length in characters, or of how many entities. */
  min?: number;
  max?: number;
  required: boolean;
}

export interface Location {
  name: string;
  latitude: number;
  longitude: number;
}

export interface NewsFeed {
  id: number;
  name: string;
  url: string;
}

/** A calendar; its secret address never leaves the server. */
export interface Calendar {
  id: number;
  name: string;
  host: string;
  /** One of the names in colors.ts. */
  color: string;
  /** True when the address cannot be decrypted with this server's key. */
  unreadable: boolean;
}

export interface Settings {
  location: Location | null;
  newsFeeds: NewsFeed[];
  calendars: Calendar[];
  /** What the TVs show their texts and dates in. */
  language: 'en' | 'nl' | 'de' | 'fr';
  /** Null until Home Assistant is connected; the token never leaves the server. */
  homeAssistant: HomeAssistant | null;
}

export interface HomeAssistant {
  url: string;
  /** True when the connection cannot be decrypted with this server's key. */
  unreadable: boolean;
}

/** A Home Assistant entity, as the widget editor lists it. */
export interface HomeAssistantEntity {
  id: string;
  name: string;
  state: string;
  unit?: string;
}

/** Whether a data source works, by data key such as "weather.current". */
export interface SourceStatus {
  updatedAt?: string;
  failing: boolean;
}

export interface Place extends Location {
  /** Province or state and country, for telling places apart. */
  region: string;
}

export interface Photo {
  name: string;
  size: number;
  addedAt: string;
}

/** An API token for automations; the secret is only in the response that makes it. */
export interface ApiToken {
  id: number;
  name: string;
  createdAt: string;
  /** Null until the token is used. */
  lastUsedAt: string | null;
}

export interface NewApiToken extends ApiToken {
  token: string;
}

export interface CastDevice {
  id: string;
  name: string;
  model: string;
  addr: string;
  hasScreen: boolean;
}
