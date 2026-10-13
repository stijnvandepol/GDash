import type { HomeyCapability } from './api';
import type { Searchable } from './entities';
import { describeAge } from './status';

/** Where a Homey that sends its data sends it; origin is GDash's own, such as http://192.168.1.2:8420. */
export function snapshotURL(origin: string): string {
  return `${origin}/api/homey/snapshot`;
}

/**
 * The HomeyScript that sends a Homey's devices and users to GDash, with url
 * and token filled in. A Homey Pro (Early 2019) may run an older Node.js,
 * so it stays clear of ?. and ??.
 */
export function homeyScript(url: string, token: string): string {
  return `// Sends every device and who is home to GDash. Run it every minute from a Flow.
const GDASH_URL = '${url}', TOKEN = '${token}';
const devs = await Homey.devices.getDevices(), zones = await Homey.zones.getZones();
let users = {};
try { users = await Homey.users.getUsers(); } catch (e) {}
const out = { at: new Date().toISOString(), devices: [], users: [] };
for (const d of Object.values(devs)) {
  if (!d.capabilitiesObj) continue;
  const caps = {};
  for (const [id, c] of Object.entries(d.capabilitiesObj)) caps[id] = { title: c.title, value: c.value, units: c.units || null };
  out.devices.push({ id: d.id, name: d.name, zone: zones[d.zone] ? zones[d.zone].name : null, available: d.available !== false, caps: caps });
}
for (const u of Object.values(users)) out.users.push({ id: u.id, name: u.name, present: u.present === true, asleep: u.asleep === true });
const r = await fetch(GDASH_URL, { method: 'PUT', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + TOKEN }, body: JSON.stringify(out) });
if (!r.ok) throw new Error('GDash ' + r.status);
return true;`;
}

/** The screens mark a Homey's data as old this long after its last snapshot (cmd/gdash). */
const SNAPSHOT_FRESH_MS = 3 * 60_000;

/** Whether the last snapshot, received at updatedAt, is older than the screens accept. */
export function isSnapshotLate(updatedAt: string | undefined, now = Date.now()): boolean {
  return updatedAt !== undefined && now > Date.parse(updatedAt) + SNAPSHOT_FRESH_MS;
}

/** "Last data received 2 minutes ago.", or that nothing arrived yet. */
export function describeSnapshot(updatedAt: string | undefined, now = Date.now()): string {
  if (!updatedAt) return 'Nothing received yet. Add the script and its Flow on your Homey.';
  const ago = describeAge(Math.max(0, Math.round((now - Date.parse(updatedAt)) / 60_000)));
  const late = isSnapshotLate(updatedAt, now) ? ' That is late; the screens mark Homey values as old. Check the Flow on your Homey.' : '';
  return `Last data received ${ago}.${late}`;
}

/** The longest badge label the server accepts (internal/display). */
export const MAX_BADGE_LABEL = 12;

/** A person a Who's home widget looks for, as its users setting stores them. */
export interface PresenceUser {
  id: string;
  label: string;
}

/** A Homey value as the search lists it. */
export interface CapabilityChoice extends Searchable {
  detail: string;
}

/** Initials for a badge: of the first and last word, or the first two letters of one word. */
export function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  const first = [...(words[0] ?? '')];
  const last = [...(words[words.length - 1] ?? '')];
  const letters = words.length > 1 ? `${first[0] ?? ''}${last[0] ?? ''}` : first.slice(0, 2).join('');
  return letters.toUpperCase() || '?';
}

/** Reads a users setting; entries that are not a user with a label are left out. */
export function presenceUsers(value: unknown): PresenceUser[] {
  if (!Array.isArray(value)) return [];
  return value.filter(
    (v): v is PresenceUser =>
      typeof v === 'object' && v !== null && typeof (v as PresenceUser).id === 'string' && typeof (v as PresenceUser).label === 'string',
  );
}

/** Formats a value for the editor, such as "21.5 °C" or "true". */
function currentValue(c: HomeyCapability): string {
  if (c.value === null) return '–';
  return `${String(c.value)}${c.units ? ` ${c.units}` : ''}`;
}

/** The capabilities as the search finds and lists them. */
export function capabilityChoices(list: HomeyCapability[]): CapabilityChoice[] {
  return list.map((c) => ({
    id: c.key,
    name: `${c.device} · ${c.title}`,
    zone: c.zone,
    detail: [c.zone, currentValue(c)].filter(Boolean).join(' · '),
  }));
}
