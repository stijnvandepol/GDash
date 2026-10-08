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
    const rejected = this.fields.find((f) => f.field === field);
    return rejected ? (fieldMessages[field] ?? rejected.message) : undefined;
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
}

export interface CastDevice {
  id: string;
  name: string;
  model: string;
  addr: string;
  hasScreen: boolean;
}

// The API answers in English; the interface translates by error code and
// falls back to the server's message for codes it does not know.
const messages: Record<string, string> = {
  invalid_credentials: 'De gebruikersnaam of het wachtwoord klopt niet.',
  rate_limited: 'Te veel mislukte pogingen. Wacht 15 minuten en probeer het opnieuw.',
  invalid_setup_token: 'De setup-token klopt niet. Je vindt hem in de log van Daycast.',
  setup_done: 'Daycast is al ingesteld. Log in.',
  not_logged_in: 'Je sessie is verlopen. Log opnieuw in.',
  not_found: 'Dit display bestaat niet (meer).',
  conflict: 'Er bestaat al een display met dit adres, of het maximum aantal displays is bereikt.',
  discovery_failed: 'Zoeken naar Cast-apparaten lukte niet. Draait Daycast met host-networking?',
  invalid_input: 'Niet alle velden zijn goed ingevuld.',
  internal: 'Er ging iets mis op de server. Kijk in de log van Daycast.',
};

const fieldMessages: Record<string, string> = {
  id: 'Gebruik 1 tot 32 kleine letters, cijfers of streepjes.',
  name: 'Gebruik 1 tot 60 tekens, zonder spaties aan het begin of eind.',
  castDeviceId: 'Kies een Cast-apparaat uit de lijst.',
  username: 'Gebruik 1 tot 32 letters, cijfers, punten, streepjes of underscores.',
  password: 'Gebruik minstens 10 tekens.',
};

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
    throw new ApiError(response.status, 'invalid_response', 'De server gaf een onverwacht antwoord.');
  }
  if (!response.ok) {
    const err = (payload as { error?: { code?: string; message?: string; fields?: FieldError[] } }).error;
    const code = err?.code ?? 'unknown';
    throw new ApiError(response.status, code, messages[code] ?? err?.message ?? 'Er ging iets mis.', err?.fields ?? []);
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
  updateDisplay: (id: string, changes: { name?: string; castDeviceId?: string }) =>
    request<Display>('PATCH', `/api/displays/${encodeURIComponent(id)}`, changes),
  deleteDisplay: (id: string) => request<void>('DELETE', `/api/displays/${encodeURIComponent(id)}`),
  castDevices: () => request<{ devices: CastDevice[] }>('GET', '/api/cast-devices').then((r) => r.devices),
};

/** Turns "Keuken TV" into "keuken-tv", the form display ids must have. */
export function slugify(name: string): string {
  return name
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 32);
}
