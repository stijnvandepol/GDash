// Small client for the GDash API. Every error response has the shape
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

// The shapes the API answers with live in types.ts, so this file holds the calls.
export type * from './types';
import type {
  ApiToken,
  Calendar,
  CastDevice,
  Dashboard,
  DashboardTemplate,
  Display,
  HomeAssistant,
  HomeAssistantEntity,
  Location,
  NewApiToken,
  NewsFeed,
  Photo,
  Place,
  Settings,
  Slot,
  SourceStatus,
  User,
  WidgetCatalogue,
} from './types';

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
  if (!response.ok) throw await toApiError(response);
  try {
    return (await response.json()) as T;
  } catch {
    throw new ApiError(response.status, 'invalid_response', 'The server sent an unexpected response.');
  }
}

/** Posts a JSON body and returns the answer as a file, such as a backup. */
async function download(path: string, body: unknown): Promise<Blob> {
  const response = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    credentials: 'same-origin',
  });
  if (!response.ok) throw await toApiError(response);
  return response.blob();
}

/** The ApiError for a failed response, from its {"error": …} body. */
async function toApiError(response: Response): Promise<ApiError> {
  try {
    const payload = (await response.json()) as { error?: { code?: string; message?: string; fields?: FieldError[] } };
    const err = payload.error;
    return new ApiError(response.status, err?.code ?? 'unknown', err?.message ?? 'Something went wrong.', err?.fields ?? []);
  } catch {
    return new ApiError(response.status, 'invalid_response', 'The server sent an unexpected response.');
  }
}

export const api = {
  backup: (password: string) => download('/api/backup', { password }),
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
    request<Dashboard>('PUT', `/api/dashboards/${encodeURIComponent(d.id)}`, { name: d.name, theme: d.theme, font: d.font ?? '', widgets: d.widgets }),
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
