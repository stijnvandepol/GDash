import { describe, expect, it } from 'vitest';
import { capabilityChoices, initials, presenceUsers } from './homey';

describe('initials', () => {
  it('takes the first and last word, or two letters of one', () => {
    expect(initials('Anna de Vries')).toBe('AV');
    expect(initials('bram')).toBe('BR');
    expect(initials('  Émile  ')).toBe('ÉM');
    expect(initials('Q')).toBe('Q');
    expect(initials('')).toBe('?');
  });
});

describe('presenceUsers', () => {
  it('keeps users with a label', () => {
    expect(presenceUsers([{ id: 'u1', label: 'AN' }, { id: 'u2' }, null, 'u3'])).toEqual([{ id: 'u1', label: 'AN' }]);
    expect(presenceUsers(undefined)).toEqual([]);
  });
});

describe('capabilityChoices', () => {
  it('names a value after its device and title, with its zone and value beside it', () => {
    const [choice] = capabilityChoices([
      { key: 'd1/measure_temperature', zone: 'Kitchen', device: 'Thermostat', title: 'Temperature', capability: 'measure_temperature', value: 21.5, units: '°C' },
    ]);
    expect(choice).toEqual({ id: 'd1/measure_temperature', name: 'Thermostat · Temperature', zone: 'Kitchen', detail: 'Kitchen · 21.5 °C' });
    expect(capabilityChoices([{ key: 'd2/onoff', device: 'Lamp', title: 'On', capability: 'onoff', value: null }])[0]?.detail).toBe('–');
  });
});
