// Small client for the Nookboard API. Every error response has the shape
// {"error": {"code", "message", "fields"?}}, see internal/api/server.go.
import type { AuditEvent } from './audit';

export interface FieldError {
  field: string;
  message: string;
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly fields: FieldError[] = [],
  ) {
    super(message);
  }

  /** The message for one form field, if the server rejected it. */
  fieldMessage(field: string): string | undefined {
    return this.fields.find((f) => f.field === field)?.message;
  }
}

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
  /** What Nookboard last found on the Cast device; only in the display list. */
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
  language: 'en' | 'nl';
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

/** Sends body as JSON, or as it is when it is a form, such as a photo upload. */
async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const isForm = body instanceof FormData;
  const response = await fetch(path, {
    method,
    headers: body === undefined || isForm ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : isForm ? body : JSON.stringify(body),
    credentials: 'same-origin',
  });
  if (response.status === 204) return undefined as T;
  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    throw new ApiError(response.status, 'invalid_response', 'The server sent an unexpected response.');
  }
  if (!response.ok) {
    const err = (payload as { error?: { code?: string; message?: string; fields?: FieldError[] } }).error;
    throw new ApiError(response.status, err?.code ?? 'unknown', err?.message ?? 'Something went wrong.', err?.fields ?? []);
  }
  return payload as T;
}

export const api = {
  setupNeeded: () => request<{ needed: boolean }>('GET', '/api/setup').then((r) => r.needed),
  setup: (setupToken: string, username: string, password: string) =>
    request<User>('POST', '/api/setup', { setupToken, username, password }),
  login: (username: string, password: string) => request<User>('POST', '/api/sessions', { username, password }),
  logout: () => request<void>('DELETE', '/api/sessions/current'),
  me: () => request<User>('GET', '/api/me'),
  system: () => request<{ version: string }>('GET', '/api/system'),
  photos: () => request<{ photos: Photo[] }>('GET', '/api/photos').then((r) => r.photos),
  uploadPhoto: (file: File) => {
    const form = new FormData();
    form.append('photo', file);
    return request<Photo>('POST', '/api/photos', form);
  },
  deletePhoto: (name: string) => request<void>('DELETE', `/api/photos/${encodeURIComponent(name)}`),
  auditEvents: (limit: number) =>
    request<{ events: AuditEvent[] }>('GET', `/api/audit-events?limit=${limit}`).then((r) => r.events),
  apiTokens: () => request<{ tokens: ApiToken[] }>('GET', '/api/api-tokens').then((r) => r.tokens),
  createApiToken: (name: string) => request<NewApiToken>('POST', '/api/api-tokens', { name }),
  revokeApiToken: (id: number) => request<void>('DELETE', `/api/api-tokens/${id}`),
  announce: (message: string, seconds: number) =>
    request<{ displays: string[] }>('POST', '/api/announcements', { message, seconds }),
  changePassword: (currentPassword: string, newPassword: string) =>
    request<void>('PUT', '/api/me/password', { currentPassword, newPassword }),
  displays: () => request<{ displays: Display[] }>('GET', '/api/displays').then((r) => r.displays),
  createDisplay: (d: { id: string; name: string; castDeviceId: string }) => request<Display>('POST', '/api/displays', d),
  updateDisplay: (
    id: string,
    changes: {
      name?: string;
      castDeviceId?: string;
      dashboardId?: string;
      activeFrom?: string;
      activeUntil?: string;
      schedule?: Slot[];
      rotation?: string[];
      rotateSeconds?: number;
      textScale?: number;
    },
  ) =>
    request<Display>('PATCH', `/api/displays/${encodeURIComponent(id)}`, changes),
  settings: () => request<{ settings: Settings; status: Record<string, SourceStatus> }>('GET', '/api/settings'),
  setLocation: (loc: Location) => request<Location>('PUT', '/api/settings/location', loc),
  clearLocation: () => request<void>('DELETE', '/api/settings/location'),
  setLanguage: (language: string) => request<{ language: string }>('PUT', '/api/settings/language', { language }),
  places: (name: string) =>
    request<{ places: Place[] }>('GET', `/api/places?name=${encodeURIComponent(name)}`).then((r) => r.places),
  addNewsFeed: (name: string, url: string) => request<NewsFeed>('POST', '/api/news-feeds', { name, url }),
  deleteNewsFeed: (id: number) => request<void>('DELETE', `/api/news-feeds/${id}`),
  addCalendar: (name: string, url: string) => request<Calendar>('POST', '/api/calendars', { name, url }),
  deleteCalendar: (id: number) => request<void>('DELETE', `/api/calendars/${id}`),
  setCalendarColor: (id: number, color: string) => request<void>('PATCH', `/api/calendars/${id}`, { color }),
  connectHomeAssistant: (url: string, token: string) =>
    request<HomeAssistant>('PUT', '/api/homeassistant', { url, token }),
  disconnectHomeAssistant: () => request<void>('DELETE', '/api/homeassistant'),
  homeAssistantEntities: () =>
    request<{ entities: HomeAssistantEntity[] }>('GET', '/api/homeassistant/entities').then((r) => r.entities),
  deleteDisplay: (id: string) => request<void>('DELETE', `/api/displays/${encodeURIComponent(id)}`),
  pairScreen: (id: string, code: string) =>
    request<void>('POST', `/api/displays/${encodeURIComponent(id)}/pairings`, { code }),
  unpairScreens: (id: string) =>
    request<{ removed: number }>('DELETE', `/api/displays/${encodeURIComponent(id)}/pairings`),
  widgetTypes: () => request<WidgetCatalogue>('GET', '/api/widget-types'),
  dashboardTemplates: () => request<{ templates: DashboardTemplate[] }>('GET', '/api/dashboard-templates').then((r) => r.templates),
  dashboards: () => request<{ dashboards: Dashboard[] }>('GET', '/api/dashboards').then((r) => r.dashboards),
  createDashboard: (d: Dashboard) => request<Dashboard>('POST', '/api/dashboards', d),
  saveDashboard: (d: Dashboard) =>
    request<Dashboard>('PUT', `/api/dashboards/${encodeURIComponent(d.id)}`, { name: d.name, theme: d.theme, widgets: d.widgets }),
  deleteDashboard: (id: string) => request<void>('DELETE', `/api/dashboards/${encodeURIComponent(id)}`),
  castDevices: () => request<{ devices: CastDevice[] }>('GET', '/api/cast-devices').then((r) => r.devices),
};

/** Turns "Kitchen TV" into "kitchen-tv", the form display ids must have. */
export function slugify(name: string): string {
  return name
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 32);
}
