import { describe, expect, it } from 'vitest';
import type { Dashboard, Display } from './api';
import { describeHours, describeScreens, suggestedDashboard } from './displays';

function display(changes: Partial<Display> = {}): Display {
  return {
    id: 'keuken',
    name: 'Keuken',
    castDeviceId: '',
    createdAt: '2026-10-01T10:00:00Z',
    dashboardId: '',
    activeFrom: '',
    activeUntil: '',
    schedule: [],
    rotation: [],
    rotateSeconds: 0,
    textScale: 100,
    screensaverMinutes: 15,
    screenCare: 'off',
    dimAtNight: 'off',
    dimFrom: '',
    dimUntil: '',
    ...changes,
  };
}

const slot = { from: '06:30', until: '09:00', days: [], dashboardId: 'morning' };

describe('describeHours', () => {
  it('says when the display is on', () => {
    expect(describeHours(display())).toBe('Always on');
    expect(describeHours(display({ activeFrom: '07:00', activeUntil: '22:00' }))).toBe('On 07:00–22:00');
  });

  it('counts the other dashboards in plain words', () => {
    expect(describeHours(display({ schedule: [slot] }))).toBe('Always on · another dashboard at 1 set time');
    expect(describeHours(display({ schedule: [slot, slot, slot], rotation: ['news'] }))).toBe(
      'Always on · another dashboard at 3 set times · takes turns with 1 other dashboard',
    );
  });
});

describe('describeScreens', () => {
  it('says nothing without screens', () => {
    expect(describeScreens(display())).toBe('');
    expect(describeScreens(display({ castDeviceId: 'abc', pairedScreens: 0 }))).toBe('');
  });

  it('counts the screens of a display without Cast', () => {
    expect(describeScreens(display({ pairedScreens: 1 }))).toBe('Shown on 1 screen');
    expect(describeScreens(display({ pairedScreens: 2 }))).toBe('Shown on 2 screens');
  });

  it('counts the screens next to a Cast TV as extras', () => {
    expect(describeScreens(display({ castDeviceId: 'abc', pairedScreens: 1 }))).toBe('Also open on 1 other screen');
    expect(describeScreens(display({ castDeviceId: 'abc', pairedScreens: 3 }))).toBe('Also open on 3 other screens');
  });
});

describe('suggestedDashboard', () => {
  const board = (id: string, createdAt?: string): Dashboard => ({ id, name: id, widgets: [], createdAt });

  it('is the default dashboard when there is no other', () => {
    expect(suggestedDashboard([])).toBe('default');
    expect(suggestedDashboard([board('default', '2026-10-12T10:00:00Z')])).toBe('default');
  });

  it('is the newest dashboard besides the default', () => {
    const list = [
      board('default', '2026-10-12T12:00:00Z'),
      board('kitchen', '2026-10-10T10:00:00.5Z'),
      board('hall', '2026-10-10T10:00:00Z'),
      board('old'),
    ];
    expect(suggestedDashboard(list)).toBe('kitchen');
  });
});
