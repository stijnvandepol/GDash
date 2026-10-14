import { describe, expect, it } from 'vitest';
import type { CastDevice, Display } from './api';
import { deviceChoices } from './castDevices';

const device = (id: string, name: string, model: string, hasScreen = false): CastDevice => ({ id, name, model, addr: '192.0.2.1:8009', hasScreen });

const tv: Display = {
  id: 'living',
  name: 'Living room',
  castDeviceId: 'tv',
  createdAt: '2026-10-01T10:00:00Z',
  dashboardId: '',
  activeFrom: '',
  activeUntil: '',
  schedule: [],
  rotation: [],
  rotateSeconds: 0,
  textScale: 100,
  screensaverMinutes: 15,
};

describe('deviceChoices', () => {
  const devices = [
    device('mini', 'Kitchen', 'Google Nest Mini'),
    device('tv', 'Living room TV', 'Chromecast', true),
    device('group', 'Whole house', 'Google Cast Group'),
  ];

  it('lists groups first and blocks the TVs that show a dashboard', () => {
    expect(deviceChoices(devices, [tv], [])).toEqual([
      { id: 'group', name: 'Whole house', detail: 'Speaker group' },
      { id: 'mini', name: 'Kitchen', detail: 'Google Nest Mini' },
      { id: 'tv', name: 'Living room TV', detail: 'Chromecast', blockedBy: 'Living room' },
    ]);
  });

  it('keeps a chosen device that is not found now, so it can be removed', () => {
    const choices = deviceChoices([], [], ['gone']);
    expect(choices).toEqual([{ id: 'gone', name: 'Not found on the network', detail: 'gone', missing: true }]);
  });
});
