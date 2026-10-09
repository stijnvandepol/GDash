import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Snapshot } from '../protocol';
import { entityStates, show } from './hass';

beforeEach(() => {
  vi.stubGlobal('navigator', { language: 'en-US' });
});

afterEach(() => {
  display.snapshot = null;
  vi.unstubAllGlobals();
});

describe('show', () => {
  it('formats numbers in the screen language with the unit', () => {
    display.snapshot = { language: 'nl' } as Snapshot;
    expect(show('sensor.temp', { state: '21.46', name: 'Temp', unit: '°C' })).toEqual({ value: '21,5', unit: '°C', dim: false });
  });

  it('passes the unit on exactly as Home Assistant spells it', () => {
    // "W" once looked like "w" on the TV; the text was never changed, only
    // drawn too small beside the value (see .unit in HomeAssistant.svelte).
    expect(show('sensor.power', { state: '1234', name: 'Power', unit: 'W' }).unit).toBe('W');
    expect(show('sensor.energy', { state: '12.8', name: 'Energy', unit: 'kWh' }).unit).toBe('kWh');
  });

  it('names binary states after the kind of sensor', () => {
    expect(show('binary_sensor.door', { state: 'on', name: 'Door', deviceClass: 'door' }).value).toBe('Open');
    expect(show('binary_sensor.hall', { state: 'off', name: 'Hall', deviceClass: 'motion' }).value).toBe('Clear');
    expect(show('binary_sensor.x', { state: 'on', name: 'X' }).value).toBe('On');
    display.snapshot = { language: 'nl' } as Snapshot;
    expect(show('lock.front', { state: 'locked', name: 'Front' }).value).toBe('Op slot');
    expect(show('person.sam', { state: 'not_home', name: 'Sam' }).value).toBe('Weg');
  });

  it('dims states without a value and makes other states readable', () => {
    expect(show('sensor.x', { state: 'unavailable', name: 'X' })).toEqual({ value: '–', unit: '', dim: true });
    expect(show('climate.living', { state: 'heat_cool', name: 'Living' }).value).toBe('Heat cool');
  });
});

describe('entityStates', () => {
  it('keeps only well-formed entities', () => {
    const value = { 'sensor.a': { state: '1', name: 'A' }, 'sensor.b': { state: 2 }, 'sensor.c': null };
    expect(Object.keys(entityStates(value) ?? {})).toEqual(['sensor.a']);
    expect(entityStates([1, 2])).toBeUndefined();
  });
});
