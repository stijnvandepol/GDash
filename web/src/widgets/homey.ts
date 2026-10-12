import { locale, t, type TextKey } from '../i18n.svelte';
import type { Shown } from './hass';

/** One capability of a device as the server sends it (internal/homey.State). */
export interface CapabilityState {
  device: string;
  title: string;
  /** Such as measure_temperature or alarm_contact.window. */
  capability: string;
  /** Null when the device is unavailable or has no value yet. */
  value: number | boolean | string | null;
  units?: string;
  decimals?: number;
}

// What true and false mean depends on the capability, as in Home Assistant.
const BOOLEANS: Record<string, [TextKey, TextKey]> = {
  onoff: ['haOn', 'haOff'],
  alarm_motion: ['haDetected', 'haClear'],
  alarm_contact: ['haOpen', 'haClosed'],
  alarm_water: ['haWet', 'haDry'],
  locked: ['haLocked', 'haUnlocked'],
};
// Other alarms, such as smoke, CO or tamper, and any other yes or no.
const ALARM: [TextKey, TextKey] = ['homeyAlarm', 'homeyNoAlarm'];
const ON_OFF: [TextKey, TextKey] = ['haOn', 'haOff'];
const DEFAULT_DECIMALS = 1;

/** Turns a capability's value into a value for the screen. */
export function showCapability(state: CapabilityState): Shown {
  const { value } = state;
  if (value === null || value === '') return { value: '–', unit: '', dim: true };
  if (typeof value === 'number') {
    const format = new Intl.NumberFormat(locale(), { maximumFractionDigits: state.decimals ?? DEFAULT_DECIMALS });
    return { value: format.format(value), unit: state.units ?? '', dim: false };
  }
  if (typeof value === 'boolean') {
    // A sub-capability, such as alarm_contact.window, means what its base does.
    const base = state.capability.split('.')[0] ?? '';
    const words = BOOLEANS[base] ?? (base.startsWith('alarm_') ? ALARM : ON_OFF);
    return { value: t(words[value ? 0 : 1]), unit: '', dim: false };
  }
  // A choice, such as a thermostat mode "heat_cool", as readable text.
  const text = value.replace(/_/g, ' ');
  return { value: text.charAt(0).toUpperCase() + text.slice(1), unit: state.units ?? '', dim: false };
}

/** The name of a value on the screen: its device and what it measures. */
export function capabilityName(state: CapabilityState): string {
  return `${state.device} · ${state.title}`;
}

function isPlainValue(v: unknown): v is CapabilityState['value'] {
  return v === null || typeof v === 'number' || typeof v === 'boolean' || typeof v === 'string';
}

/** Reads the server's value: capability key to state. Anything else is ignored. */
export function capabilityStates(value: unknown): Record<string, CapabilityState> | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const out: Record<string, CapabilityState> = {};
  for (const [key, v] of Object.entries(value)) {
    const s = v as Partial<CapabilityState> | null;
    if (s && typeof s.device === 'string' && typeof s.title === 'string' && typeof s.capability === 'string' && isPlainValue(s.value ?? null)) {
      out[key] = { ...s, value: s.value ?? null } as CapabilityState;
    }
  }
  return out;
}

/**
 * The labels of the people who are home for one Who's home widget, from
 * the server's value: widget key to labels. Undefined when the value cannot
 * be read; the server only ever sends the labels of who is home.
 */
export function presentLabels(value: unknown, key: string): string[] | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const labels = (value as Record<string, unknown>)[key];
  return Array.isArray(labels) ? labels.filter((l): l is string => typeof l === 'string') : [];
}
