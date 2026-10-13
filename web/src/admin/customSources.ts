import type { CustomAuthKind, CustomLogin, CustomSource, CustomSourceInput } from './api';
import { parsePath } from './jsonPath';
import { describeAge } from './status';

/** How often a custom API can be fetched; internal/custom allows 30 seconds to 24 hours. */
export const INTERVALS: { seconds: number; label: string }[] = [
  { seconds: 30, label: 'Every 30 seconds' },
  { seconds: 60, label: 'Every minute' },
  { seconds: 300, label: 'Every 5 minutes' },
  { seconds: 900, label: 'Every 15 minutes' },
  { seconds: 1800, label: 'Every 30 minutes' },
  { seconds: 3600, label: 'Every hour' },
  { seconds: 21600, label: 'Every 6 hours' },
  { seconds: 86400, label: 'Once a day' },
];

export const AUTH_KINDS: { kind: CustomAuthKind; label: string; nameLabel?: string; secretLabel?: string }[] = [
  { kind: 'none', label: 'None (an open API)' },
  { kind: 'bearer', label: 'Bearer token', secretLabel: 'Token' },
  { kind: 'header', label: 'API key in a header', nameLabel: 'Header name', secretLabel: 'API key' },
  { kind: 'query', label: 'API key in the address', nameLabel: 'Parameter name', secretLabel: 'API key' },
  { kind: 'basic', label: 'User name and password', nameLabel: 'User name', secretLabel: 'Password' },
  { kind: 'login', label: 'Log in first (a session, such as Pi-hole)', secretLabel: 'Password' },
];

export const TOKEN_PLACES: { place: CustomLogin['place']; label: string; nameLabel?: string; placeholder?: string }[] = [
  { place: 'header', label: 'In a header', nameLabel: 'Header name', placeholder: 'X-FTL-SID' },
  { place: 'query', label: 'In the address', nameLabel: 'Parameter name', placeholder: 'sid' },
  { place: 'bearer', label: 'As a bearer token (Authorization: Bearer)' },
];

/** A login for the form, filled in as Pi-hole v6 wants it apart from the address. */
export function newLogin(url: string): CustomLogin {
  let origin = '';
  try {
    origin = new URL(url).origin;
  } catch {
    // No address yet: the login address is typed by hand.
  }
  return { url: origin ? `${origin}/api/auth` : '', body: '{"password": "{secret}"}', tokenPath: 'session.sid', place: 'header', validityPath: 'session.validity' };
}

/** The input to send: a login only for the kind that logs in, and no header name for a bearer token. */
export function withLogin(input: CustomSourceInput): CustomSourceInput {
  const { login, ...auth } = input.auth;
  if (auth.kind !== 'login' || !login) return { ...input, auth };
  return { ...input, auth: { ...auth, name: login.place === 'bearer' ? '' : auth.name, login: { ...login } } };
}

export const MAX_HEADERS = 10;
// The server takes a sample of at most 48 KB.
const MAX_SAMPLE_BYTES = 48 * 1024;

/** The form for a new source. */
export function emptyInput(): CustomSourceInput {
  return {
    name: '',
    mode: 'fetch',
    url: '',
    method: 'GET',
    body: '',
    auth: { kind: 'none', name: '', secret: '' },
    headers: [],
    intervalSeconds: 300,
    sample: null,
  };
}

/** The form for an existing source: secrets stay empty, which keeps them. */
export function toInput(source: CustomSource): CustomSourceInput {
  return {
    name: source.name,
    mode: source.mode,
    url: source.url,
    method: source.method,
    body: source.body,
    auth: { kind: source.auth.kind, name: source.auth.name, secret: '', ...(source.auth.login ? { login: { ...source.auth.login } } : {}) },
    headers: source.headers.map((h) => ({ ...h })),
    intervalSeconds: source.intervalSeconds,
    sample: source.sample ?? null,
  };
}

/** The sample as text for the form. */
export function sampleText(sample: unknown): string {
  return sample === null || sample === undefined ? '' : JSON.stringify(sample, null, 2);
}

/** Reads a pasted sample answer; empty means none. */
export function parseSample(text: string): { sample: unknown } | { error: string } {
  if (text.trim() === '') return { sample: null };
  if (new Blob([text]).size > MAX_SAMPLE_BYTES) return { error: 'Paste a sample of at most 48 KB.' };
  try {
    return { sample: JSON.parse(text) as unknown };
  } catch {
    return { error: 'This is not valid JSON. Paste the answer exactly as the API sends it.' };
  }
}

/** What the secret field of a custom API or camera says when a secret is stored and may be kept. */
export function secretPlaceholder(source: Pick<CustomSource, 'auth'> | undefined, kind: CustomAuthKind): string {
  return source?.auth.secretSet && source.auth.kind === kind ? 'Stored; leave empty to keep it' : '';
}

export function describeInterval(seconds: number): string {
  return INTERVALS.find((i) => i.seconds === seconds)?.label ?? `Every ${seconds} seconds`;
}

/** The size of an answer: "312 bytes" or "14 KB". */
export function describeSize(bytes: number): string {
  return bytes < 1024 ? `${bytes} bytes` : `${Math.round(bytes / 1024)} KB`;
}

/** The host of an address, to show in the list. */
export function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

/** internal/custom allows names of this many characters, 3 columns per list. */
export const MAX_VALUE_NAME = 40;
export const MAX_COLUMNS = 3;

/** "2 values · 1 list", or that none are named yet. */
export function describeNamed(source: Pick<CustomSource, 'values' | 'lists'>): string {
  const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;
  if (source.values.length + source.lists.length === 0) return 'No values named yet';
  return [count(source.values.length, 'value', 'values'), source.lists.length > 0 ? count(source.lists.length, 'list', 'lists') : '']
    .filter(Boolean)
    .join(' · ');
}

/**
 * A name for a value clicked at path: its last key in words, such as
 * "Temperature 2m" for current.temperature_2m or "Items 1" for items[0],
 * made unique among taken (ignoring case) with a number.
 */
export function suggestName(path: string, taken: string[]): string {
  const segments = parsePath(path)?.segments ?? [];
  const last = segments[segments.length - 1];
  const key = [...segments].reverse().find((s) => s.kind === 'key');
  let base = key?.kind === 'key' ? words(key.key) : 'Item';
  if (last?.kind === 'index' && last.index >= 0) base = `${base} ${last.index + 1}`;
  base = base.slice(0, MAX_VALUE_NAME);
  const used = new Set(taken.map((t) => t.toLowerCase()));
  let name = base;
  for (let n = 2; used.has(name.toLowerCase()); n++) name = `${base.slice(0, MAX_VALUE_NAME - String(n).length - 1)} ${n}`;
  return name;
}

/** feelsLike or feels_like as "Feels like". */
function words(key: string): string {
  const text = key
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .replace(/[_\-.]+/g, ' ')
    .trim()
    .toLowerCase();
  return text ? text[0]!.toUpperCase() + text.slice(1) : 'Value';
}

/** The form's fields for a source that is sent to GDash: it has no request of its own. */
export function pushInput(form: CustomSourceInput): CustomSourceInput {
  return { ...form, url: '', method: 'GET', body: '', auth: { kind: 'none', name: '', secret: '' }, headers: [] };
}

/** Where a household's system sends the data of source id; origin is GDash's own, such as http://192.168.1.2:8420. */
export function pushURL(origin: string, id: number): string {
  return `${origin}/api/custom-sources/${id}/data`;
}

/** Stands in for the API token in the examples until one is made. */
export const TOKEN_PLACEHOLDER = 'YOUR_API_TOKEN';

/** Examples that send data to url with token, ready to copy. */
export function pushExamples(url: string, token: string): { curl: string; homeyScript: string; homeAssistant: string } {
  const curl = [
    `curl -X PUT '${url}' \\`,
    `  -H 'Authorization: Bearer ${token}' \\`,
    `  -H 'Content-Type: application/json' \\`,
    `  -d '{"outside": 12.5, "home": {"Anna": true}}'`,
  ].join('\n');
  // HomeyScript on a Homey Pro (Early 2019) may run an older Node.js, so
  // this stays clear of ?. and ??.
  const homeyScript = `// Runs on the Homey every minute from a Flow: sends device values and who is home to GDash.
const GDASH_URL = '${url}';
const TOKEN = '${token}';
if (!TOKEN || TOKEN === '${TOKEN_PLACEHOLDER}') throw new Error('Put your GDash API token in TOKEN on the third line (GDash: Integrations, API tokens).');
const devices = await Homey.devices.getDevices();
const zones = await Homey.zones.getZones();
const out = { devices: {}, home: {} };
for (const d of Object.values(devices)) {
  const values = { zone: zones[d.zone] ? zones[d.zone].name : null };
  for (const [id, c] of Object.entries(d.capabilitiesObj || {})) values[id] = c.value;
  out.devices[d.name] = values;
}
// Not every Homey version lets a script read its users.
let users = {};
try { users = await Homey.users.getUsers(); } catch (e) {}
for (const u of Object.values(users)) out.home[u.name] = u.present === true;
const res = await fetch(GDASH_URL, {
  method: 'PUT',
  headers: { 'Authorization': 'Bearer ' + TOKEN, 'Content-Type': 'application/json' },
  body: JSON.stringify(out),
});
if (!res.ok) {
  // GDash says why, such as an API token that is not valid or was revoked.
  let reason = '';
  try { const body = await res.json(); reason = body.error ? ': ' + body.error.message : ''; } catch (e) {}
  throw new Error('GDash answered ' + res.status + reason);
}
return res.status; // 204 means GDash received it`;
  const homeAssistant = `# configuration.yaml
rest_command:
  gdash_send:
    url: "${url}"
    method: put
    headers:
      Authorization: "Bearer ${token}"
    content_type: "application/json"
    payload: >-
      {"outside": {{ states('sensor.outside_temperature') | float(0) }},
       "home": {"Anna": {{ is_state('person.anna', 'home') | tojson }}}}

# automations.yaml: runs it every minute
- alias: Send data to GDash
  triggers:
    - trigger: time_pattern
      minutes: "/1"
  actions:
    - action: rest_command.gdash_send`;
  return { curl, homeyScript, homeAssistant };
}

/** Whether the screens show a pushed source's data as late: none for two intervals and 30 seconds. */
export function isPushLate(lastPushAt: string | undefined, intervalSeconds: number, now = Date.now()): boolean {
  if (!lastPushAt) return false;
  return now > Date.parse(lastPushAt) + (2 * intervalSeconds + 30) * 1000;
}

/** "Last data received 3 minutes ago.", and whether that is late. */
export function describePush(lastPushAt: string | undefined, intervalSeconds: number, now = Date.now()): string {
  if (!lastPushAt) return 'Nothing received yet.';
  const ago = describeAge(Math.max(0, Math.round((now - Date.parse(lastPushAt)) / 60_000)));
  const late = isPushLate(lastPushAt, intervalSeconds, now) ? ' That is late; the screens mark it as old.' : '';
  return `Last data received ${ago}.${late}`;
}
