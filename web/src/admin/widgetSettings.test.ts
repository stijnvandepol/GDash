import { describe, expect, it } from 'vitest';
import { defaultSettings, isoDate } from './widgetSettings';
import type { WidgetType } from './api';

const today = new Date(2026, 9, 9);

describe('defaultSettings', () => {
  it('fills in every setting a type takes, and nothing else', () => {
    const countdown: WidgetType = {
      minW: 4,
      minH: 4,
      settings: { label: { kind: 'text', min: 1, max: 60, required: true }, date: { kind: 'date', required: true } },
    };
    expect(defaultSettings(countdown, today)).toEqual({ label: 'Holiday', date: '2026-11-08' });
  });

  it('gives a widget without settings an empty object', () => {
    expect(defaultSettings({ minW: 4, minH: 3, settings: {} }, today)).toEqual({});
  });

  it('leaves a calendar unset, so the person chooses one and no calendar is picked for them', () => {
    const waste: WidgetType = { minW: 6, minH: 4, settings: { calendar: { kind: 'calendar', required: true } } };
    expect(defaultSettings(waste, today)).toEqual({});
  });

  it('skips settings it has no default for', () => {
    const odd: WidgetType = { minW: 1, minH: 1, settings: { colour: { kind: 'text', required: false } } };
    expect(defaultSettings(odd, today)).toEqual({});
  });
});

describe('isoDate', () => {
  it('pads months and days', () => {
    expect(isoDate(new Date(2026, 0, 5))).toBe('2026-01-05');
  });
});
