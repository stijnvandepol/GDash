import { describe, expect, it } from 'vitest';
import { capabilityChoices, describeSnapshot, homeyScript, initials, isSnapshotLate, presenceUsers, snapshotURL } from './homey';

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

describe('the HomeyScript', () => {
  const script = homeyScript(snapshotURL('http://192.168.1.2:8420'), 'gdk_token');

  it('sends to GDash with the token', () => {
    expect(script).toContain("'http://192.168.1.2:8420/api/homey/snapshot'");
    expect(script).toContain("TOKEN = 'gdk_token'");
    expect(script).toContain("method: 'PUT'");
  });

  it('parses as the body of an async function, as HomeyScript runs it', () => {
    const AsyncFunction = Object.getPrototypeOf(async () => {}).constructor;
    expect(() => new AsyncFunction('Homey', 'fetch', script)).not.toThrow();
  });

  it('refuses to run with the placeholder, and says why GDash refused it', () => {
    expect(homeyScript('http://gdash', 'YOUR_API_TOKEN')).toContain("TOKEN === 'YOUR_API_TOKEN'");
    expect(script).toContain('body.error.message');
  });

  it('stays clear of syntax an older Homey may not know', () => {
    expect(script).not.toContain('?.');
    expect(script).not.toContain('??');
  });
});

describe('describeSnapshot', () => {
  const at = '2026-10-13T08:00:00Z';
  const after = (seconds: number) => Date.parse(at) + seconds * 1000;

  it('says when the last snapshot arrived, and when that is late', () => {
    expect(describeSnapshot(undefined)).toContain('Nothing received yet');
    expect(describeSnapshot(at, after(120))).toBe('Last data received 2 minutes ago.');
    expect(isSnapshotLate(at, after(180))).toBe(false);
    expect(isSnapshotLate(at, after(181))).toBe(true);
    expect(describeSnapshot(at, after(240))).toContain('late');
  });
});