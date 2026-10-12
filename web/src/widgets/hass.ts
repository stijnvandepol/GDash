import { locale, t, type TextKey } from '../i18n.svelte';

/** One entity as the server sends it (internal/homeassistant.State). */
export interface EntityState {
  state: string;
  name: string;
  unit?: string;
  deviceClass?: string;
}

/** What the screen shows for an entity. */
export interface Shown {
  value: string;
  unit: string;
  /** True for states without a value, such as "unavailable". */
  dim: boolean;
}

/** One value of a smart home device as Readings.svelte shows it. */
export interface Reading {
  id: string;
  name: string;
  shown: Shown;
  /** True when the device or value is not known any more. */
  missing: boolean;
}

// Binary sensors say "on" and "off"; what that means depends on the kind.
const BINARY: Record<string, [TextKey, TextKey]> = {
  door: ['haOpen', 'haClosed'],
  garage_door: ['haOpen', 'haClosed'],
  window: ['haOpen', 'haClosed'],
  opening: ['haOpen', 'haClosed'],
  motion: ['haDetected', 'haClear'],
  occupancy: ['haDetected', 'haClear'],
  presence: ['haHome', 'haAway'],
  moisture: ['haWet', 'haDry'],
};

const WORDS: Record<string, TextKey> = {
  on: 'haOn',
  off: 'haOff',
  open: 'haOpen',
  closed: 'haClosed',
  locked: 'haLocked',
  unlocked: 'haUnlocked',
  home: 'haHome',
  not_home: 'haAway',
};

/** Turns an entity's raw state into a value for the screen. */
export function show(id: string, entity: EntityState): Shown {
  const { state } = entity;
  if (state === 'unavailable' || state === 'unknown' || state === '') {
    return { value: '–', unit: '', dim: true };
  }
  const number = Number(state);
  if (state.trim() !== '' && Number.isFinite(number)) {
    const value = new Intl.NumberFormat(locale(), { maximumFractionDigits: 1 }).format(number);
    return { value, unit: entity.unit ?? '', dim: false };
  }
  const binary = id.startsWith('binary_sensor.') ? BINARY[entity.deviceClass ?? ''] : undefined;
  if (binary && (state === 'on' || state === 'off')) {
    return { value: t(binary[state === 'on' ? 0 : 1]), unit: '', dim: false };
  }
  const word = WORDS[state];
  if (word) return { value: t(word), unit: '', dim: false };
  // Other states, such as "partlycloudy" or "heat_cool", as readable text.
  const text = state.replace(/_/g, ' ');
  return { value: text.charAt(0).toUpperCase() + text.slice(1), unit: entity.unit ?? '', dim: false };
}

/** Reads the server's value: entity ID to state. Anything else is ignored. */
export function entityStates(value: unknown): Record<string, EntityState> | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const out: Record<string, EntityState> = {};
  for (const [id, v] of Object.entries(value)) {
    const e = v as Partial<EntityState> | null;
    if (e && typeof e.state === 'string' && typeof e.name === 'string') out[id] = e as EntityState;
  }
  return out;
}
