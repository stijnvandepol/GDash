// Small client for the GDash API. Every error response has the shape
// {"error": {"code", "message", "fields"?}}, see internal/api/server.go.
import type { AuditEvent } from './audit';
import type { Pickup } from '../widgets/waste';

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
  Camera,
  CameraInput,
  CameraTest,
  CastDevice,
  CustomSource,
  CustomSourceInput,
  CustomTest,
  CustomValues,
  Dashboard,
  DashboardTemplate,
  DepartureStop,
  Display,
  HomeAssistant,
  HomeAssistantEntity,
  Homey,
  HomeyCapability,
  HomeyUser,
  Immich,
  ImmichAlbum,
  KnmiInfo,
  Location,
  NewApiToken,
  NewsCatalogue,
  NewsFeed,
  PendingLogin,
  ScreenCareChanges,
  Photo,
  Place,
  Settings,
  Slot,
  Solar,
  SourceStatus,
  Synology,
  SystemInfo,
  TrafficInfo,
  TwoStepEnrolment,
  TwoStepStatus,
  User,
  WebPage,
  WebPageCheck,
  WebPageInput,
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
  /** Whether the first account is still to be made, and whether this browser needs the setup token for it. */
  setupState: () => request<{ needed: boolean; tokenRequired: boolean }>('GET', '/api/setup'),
  setup: (setupToken: string, username: string, password: string) =>
    request<User>('POST', '/api/setup', { setupToken, username, password }),
  /** The user, or a pending login when the account has two-step login on. */
  login: (username: string, password: string) =>
    request<User | PendingLogin>('POST', '/api/sessions', { username, password }),
  finishLogin: (pending: string, code: string) => request<User>('POST', '/api/sessions/two-step', { pending, code }),
  twoStep: () => request<TwoStepStatus>('GET', '/api/me/two-step'),
  startTwoStep: (password: string) => request<TwoStepEnrolment>('POST', '/api/me/two-step/enrolment', { password }),
  enableTwoStep: (code: string) =>
    request<{ recoveryCodes: string[] }>('PUT', '/api/me/two-step', { code }).then((r) => r.recoveryCodes),
  disableTwoStep: (password: string, code: string) => request<void>('DELETE', '/api/me/two-step', { password, code }),
  logout: () => request<void>('DELETE', '/api/sessions/current'),
  me: () => request<User>('GET', '/api/me'),
  system: () => request<SystemInfo>('GET', '/api/system'),
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
  createDisplay: (d: { id: string; name: string; castDeviceId: string; dashboardId: string }) =>
    request<Display>('POST', '/api/displays', d),
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
      screensaverMinutes?: number;
    } & ScreenCareChanges,
  ) =>
    request<Display>('PATCH', `/api/displays/${encodeURIComponent(id)}`, changes),
  settings: () => request<{ settings: Settings; status: Record<string, SourceStatus> }>('GET', '/api/settings'),
  setLocation: (loc: Location) => request<Location>('PUT', '/api/settings/location', loc),
  clearLocation: () => request<void>('DELETE', '/api/settings/location'),
  setSolar: (panels: Solar) => request<Solar>('PUT', '/api/settings/solar', panels),
  clearSolar: () => request<void>('DELETE', '/api/settings/solar'),
  setLanguage: (language: string) => request<{ language: string }>('PUT', '/api/settings/language', { language }),
  setUpdateCheck: (enabled: boolean) => request<{ enabled: boolean }>('PUT', '/api/settings/update-check', { enabled }),
  places: (name: string) =>
    request<{ places: Place[] }>('GET', `/api/places?name=${encodeURIComponent(name)}`).then((r) => r.places),
  newsCatalogue: () => request<NewsCatalogue>('GET', '/api/news-catalogue'),
  addNewsFeed: (name: string, url: string) => request<NewsFeed>('POST', '/api/news-feeds', { name, url }),
  deleteNewsFeed: (id: number) => request<void>('DELETE', `/api/news-feeds/${id}`),
  addCalendar: (name: string, url: string) => request<Calendar>('POST', '/api/calendars', { name, url }),
  addWasteCalendar: (name: string, url: string) => request<Calendar>('POST', '/api/calendars', { name, url, purpose: 'waste' }),
  /** Reads a waste calendar now, without storing it: the pickups of the next 8 weeks. */
  testWasteCalendar: (url: string) =>
    request<{ pickups: Pickup[] }>('POST', '/api/waste-calendar-tests', { url }).then((r) => r.pickups),
  deleteCalendar: (id: number) => request<void>('DELETE', `/api/calendars/${id}`),
  setCalendarColor: (id: number, color: string) => request<void>('PATCH', `/api/calendars/${id}`, { color }),
  connectHomeAssistant: (url: string, token: string) =>
    request<HomeAssistant>('PUT', '/api/homeassistant', { url, token }),
  disconnectHomeAssistant: () => request<void>('DELETE', '/api/homeassistant'),
  homeAssistantEntities: () =>
    request<{ entities: HomeAssistantEntity[] }>('GET', '/api/homeassistant/entities').then((r) => r.entities),
  connectHomey: (url: string, key: string) => request<Homey>('PUT', '/api/homey', { url, key }),
  useHomeyPush: () => request<Homey>('PUT', '/api/homey', { mode: 'push' }),
  disconnectHomey: () => request<void>('DELETE', '/api/homey'),
  homeyCapabilities: () =>
    request<{ capabilities: HomeyCapability[] }>('GET', '/api/homey/capabilities').then((r) => r.capabilities),
  homeyUsers: () => request<{ users: HomeyUser[] }>('GET', '/api/homey/users').then((r) => r.users),
  connectImmich: (url: string, key: string) => request<Immich>('PUT', '/api/immich', { url, key }),
  disconnectImmich: () => request<void>('DELETE', '/api/immich'),
  immichAlbums: () => request<{ albums: ImmichAlbum[] }>('GET', '/api/immich/albums').then((r) => r.albums),
  chooseImmichAlbum: (albumId: string) => request<Immich>('PUT', '/api/immich/album', { albumId }),
  connectSynology: (link: string, password: string, acceptCertificate: boolean) =>
    request<Synology & { photos: number }>('PUT', '/api/synology', { link, password, acceptCertificate }),
  disconnectSynology: () => request<void>('DELETE', '/api/synology'),
  cameras: () => request<{ cameras: Camera[]; unreadable: boolean }>('GET', '/api/cameras'),
  addCamera: (input: CameraInput) => request<Camera>('POST', '/api/cameras', input),
  saveCamera: (id: number, input: CameraInput) => request<Camera>('PUT', `/api/cameras/${id}`, input),
  deleteCamera: (id: number) => request<void>('DELETE', `/api/cameras/${id}`),
  testCamera: (id: number) => request<CameraTest>('POST', `/api/cameras/${id}/tests`),
  traffic: () => request<TrafficInfo>('GET', '/api/traffic'),
  setTraffic: (enabled: boolean, acknowledged: boolean) =>
    request<Pick<TrafficInfo, 'enabled' | 'acknowledgedAt'>>('PUT', '/api/traffic', { enabled, acknowledged }),
  knmi: () => request<KnmiInfo>('GET', '/api/knmi'),
  connectKnmi: (key: string) => request<{ connected: boolean }>('PUT', '/api/knmi', { key }),
  disconnectKnmi: () => request<void>('DELETE', '/api/knmi'),
  departureStops: (query: string) =>
    request<{ stops: DepartureStop[] }>('GET', `/api/departures/stops?q=${encodeURIComponent(query)}`).then((r) => r.stops),
  customSources: () => request<{ sources: CustomSource[] }>('GET', '/api/custom-sources').then((r) => r.sources),
  addCustomSource: (input: CustomSourceInput) => request<CustomSource>('POST', '/api/custom-sources', input),
  saveCustomSource: (id: number, input: CustomSourceInput) =>
    request<CustomSource>('PUT', `/api/custom-sources/${id}`, input),
  saveCustomSourceValues: (id: number, values: CustomValues) =>
    request<CustomSource>('PUT', `/api/custom-sources/${id}/values`, values),
  deleteCustomSource: (id: number) => request<void>('DELETE', `/api/custom-sources/${id}`),
  testCustomSource: (id: number) => request<CustomTest>('POST', `/api/custom-sources/${id}/tests`),
  /** Fetches a source as the form has it, without saving; with id, a change of that source that keeps its secrets. */
  tryCustomSource: (input: CustomSourceInput, id?: number) =>
    request<CustomTest>('POST', '/api/custom-source-tests', id === undefined ? input : { ...input, id }),
  webPages: () => request<{ pages: WebPage[] }>('GET', '/api/web-pages').then((r) => r.pages),
  addWebPage: (input: WebPageInput) => request<WebPage>('POST', '/api/web-pages', input),
  saveWebPage: (id: number, input: WebPageInput) => request<WebPage>('PUT', `/api/web-pages/${id}`, input),
  deleteWebPage: (id: number) => request<void>('DELETE', `/api/web-pages/${id}`),
  checkWebPage: (id: number) => request<WebPageCheck>('POST', `/api/web-pages/${id}/checks`),
  checkRule: (check: { when: string; data: unknown; value?: unknown; row?: unknown }) =>
    request<{ result?: unknown; error?: string }>('POST', '/api/custom-rule-checks', check),
  deleteDisplay: (id: string) => request<void>('DELETE', `/api/displays/${encodeURIComponent(id)}`),
  pairScreen: (id: string, code: string) =>
    request<void>('POST', `/api/displays/${encodeURIComponent(id)}/pairings`, { code }),
  unpairScreens: (id: string) =>
    request<{ removed: number }>('DELETE', `/api/displays/${encodeURIComponent(id)}/pairings`),
  takeOverCast: (id: string) => request<void>('POST', `/api/displays/${encodeURIComponent(id)}/cast-takeovers`),
  widgetTypes: () => request<WidgetCatalogue>('GET', '/api/widget-types'),
  dashboardTemplates: () => request<{ templates: DashboardTemplate[] }>('GET', '/api/dashboard-templates').then((r) => r.templates),
  dashboards: () => request<{ dashboards: Dashboard[] }>('GET', '/api/dashboards').then((r) => r.dashboards),
  createDashboard: (d: Dashboard) => request<Dashboard>('POST', '/api/dashboards', d),
  saveDashboard: (d: Dashboard) =>
    request<Dashboard>('PUT', `/api/dashboards/${encodeURIComponent(d.id)}`, { name: d.name, theme: d.theme, font: d.font ?? '', nightTheme: d.nightTheme ?? '', widgets: d.widgets }),
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
