// Small client for the Daycast API. Every error response has the shape
// {"error": {"code", "message", "fields"?}}, see internal/api/server.go.

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

export interface WidgetCatalogue {
  columns: number;
  rows: number;
  themes: string[];
  types: Record<string, { minW: number; minH: number; hasMaxItems: boolean }>;
}

export interface CastDevice {
  id: string;
  name: string;
  model: string;
  addr: string;
  hasScreen: boolean;
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method,
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
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
  displays: () => request<{ displays: Display[] }>('GET', '/api/displays').then((r) => r.displays),
  createDisplay: (d: { id: string; name: string; castDeviceId: string }) => request<Display>('POST', '/api/displays', d),
  updateDisplay: (
    id: string,
    changes: { name?: string; castDeviceId?: string; dashboardId?: string; activeFrom?: string; activeUntil?: string },
  ) =>
    request<Display>('PATCH', `/api/displays/${encodeURIComponent(id)}`, changes),
  deleteDisplay: (id: string) => request<void>('DELETE', `/api/displays/${encodeURIComponent(id)}`),
  pairScreen: (id: string, code: string) =>
    request<void>('POST', `/api/displays/${encodeURIComponent(id)}/pairings`, { code }),
  unpairScreens: (id: string) =>
    request<{ removed: number }>('DELETE', `/api/displays/${encodeURIComponent(id)}/pairings`),
  widgetTypes: () => request<WidgetCatalogue>('GET', '/api/widget-types'),
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
