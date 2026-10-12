import type { HomeyCapability } from './api';
import type { Searchable } from './entities';

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
