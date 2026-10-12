import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Snapshot } from '../protocol';
import { capabilityName, capabilityStates, presentLabels, showCapability, type CapabilityState } from './homey';

beforeEach(() => {
  vi.stubGlobal('navigator', { language: 'en-US' });
});

afterEach(() => {
  display.snapshot = null;
  vi.unstubAllGlobals();
});

const state = (capability: string, value: CapabilityState['value'], more: Partial<CapabilityState> = {}): CapabilityState => ({
  device: 'Sensor',
  title: 'Title',
  capability,
  value,
  ...more,
});

describe('showCapability', () => {
  it('formats numbers in the screen language with Homey’s units and decimals', () => {
    display.snapshot = { language: 'nl' } as Snapshot;
    expect(showCapability(state('measure_temperature', 21.46, { units: '°C' }))).toEqual({ value: '21,5', unit: '°C', dim: false });
    expect(showCapability(state('measure_power', 1234.4, { units: 'W', decimals: 0 })).value).toBe('1.234');
    expect(showCapability(state('meter_power', 12.3456, { units: 'kWh', decimals: 2 })).value).toBe('12,35');
  });

  it('puts yes and no into words that fit the capability', () => {
    expect(showCapability(state('onoff', true)).value).toBe('On');
    expect(showCapability(state('alarm_motion', false)).value).toBe('Clear');
    expect(showCapability(state('alarm_contact.window', true)).value).toBe('Open');
    expect(showCapability(state('alarm_smoke', true)).value).toBe('Alarm');
    expect(showCapability(state('alarm_tamper', false)).value).toBe('OK');
    expect(showCapability(state('some_flag', false)).value).toBe('Off');
    display.snapshot = { language: 'nl' } as Snapshot;
    expect(showCapability(state('locked', true)).value).toBe('Op slot');
    expect(showCapability(state('alarm_water', false)).value).toBe('Droog');
  });

  it('dims a missing value and makes choices readable', () => {
    expect(showCapability(state('measure_temperature', null, { units: '°C' }))).toEqual({ value: '–', unit: '', dim: true });
    expect(showCapability(state('thermostat_mode', 'heat_cool')).value).toBe('Heat cool');
  });
});

describe('capabilityStates', () => {
  it('keeps only well-formed capabilities', () => {
    const value = {
      'd1/onoff': { device: 'Lamp', title: 'On', capability: 'onoff', value: true },
      'd1/dim': { device: 'Lamp', title: 'Dim', capability: 'dim' },
      'd1/x': { device: 'Lamp', title: 'X', capability: 'x', value: { nested: 1 } },
      'd1/y': null,
    };
    const states = capabilityStates(value) ?? {};
    expect(Object.keys(states)).toEqual(['d1/onoff', 'd1/dim']);
    expect(states['d1/dim']?.value).toBeNull();
    expect(capabilityStates([1])).toBeUndefined();
  });

  it('names a value after its device and title', () => {
    expect(capabilityName(state('measure_temperature', 20, { device: 'Kitchen', title: 'Temperature' }))).toBe('Kitchen · Temperature');
  });
});

describe('presentLabels', () => {
  it('reads the labels of one widget', () => {
    const value = { 'hall/home': ['AN', 3, 'CS'], 'hall/other': ['BR'] };
    expect(presentLabels(value, 'hall/home')).toEqual(['AN', 'CS']);
    expect(presentLabels(value, 'kitchen/home')).toEqual([]);
    expect(presentLabels(null, 'hall/home')).toBeUndefined();
  });
});
