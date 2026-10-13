import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Snapshot } from '../protocol';
import {
  activeWarnings,
  readArea,
  warningHeadline,
  warningName,
  warningWindow,
  warningsKey,
  warningsWaitingText,
  type WeatherWarning,
} from './warnings';

function speak(language: string) {
  display.snapshot = { language } as Snapshot;
}

beforeEach(() => {
  vi.stubGlobal('navigator', { language: 'en-US' });
});

afterEach(() => {
  display.snapshot = null;
  vi.unstubAllGlobals();
});

// Tuesday 13 October 2026; the tests run in Dutch time (vite.config.ts).
const at = (time: string, day = 13) => Date.parse(`2026-10-${day}T${time}:00+02:00`);

// The orange wind warning of internal/warnings/testdata/feeds-netherlands.json.
const wind: WeatherWarning = {
  id: '2.49.0.0.528.0.NL.20261012210000.aaaa-wind_1',
  level: 'orange',
  kind: 'wind',
  event: { nl: 'Zware windstoten', en: 'Severe Wind' },
  headline: { nl: 'Code oranje: zware windstoten in het zuiden', en: 'Code orange: severe gusts in the south' },
  onset: '2026-10-13T12:00:00Z',
  expires: '2026-10-13T22:00:00Z',
};

const entry = { fetchedAt: '0001-01-01T00:00:00Z', staleAfter: '0001-01-01T00:00:00Z', failing: false };

describe('warningsKey', () => {
  it('is the province, or here without one', () => {
    expect(warningsKey({ region: 'Zeeland' })).toBe('Zeeland');
    expect(warningsKey(undefined)).toBe('here');
    expect(warningsKey({ announce: true })).toBe('here');
    expect(warningsKey({ region: 3 })).toBeUndefined();
  });
});

describe('readArea', () => {
  it('reads the region and its warnings, dropping what is not a warning', () => {
    const value = { here: { region: 'Noord-Brabant', warnings: [wind, { ...wind, level: 'purple' }, { id: 'x' }] } };
    expect(readArea(value, 'here')).toEqual({ region: 'Noord-Brabant', warnings: [wind] });
  });

  it('is undefined for a missing key or a value that is no area', () => {
    expect(readArea({ here: { region: 'Zeeland', warnings: [] } }, 'Zeeland')).toBeUndefined();
    expect(readArea({ here: { warnings: [] } }, 'here')).toBeUndefined();
    expect(readArea([], 'here')).toBeUndefined();
    expect(readArea({}, undefined)).toBeUndefined();
  });
});

describe('activeWarnings', () => {
  it('drops the warnings that have expired on the screen clock', () => {
    const area = { region: 'Zeeland', warnings: [wind] };
    expect(activeWarnings(area, at('23:59'))).toEqual([wind]);
    expect(activeWarnings(area, at('00:00', 14))).toEqual([]);
  });
});

describe('warningName and warningHeadline', () => {
  it('name the kind in every language and give the KNMI headline in Dutch and English', () => {
    expect(warningName(wind)).toBe('Wind');
    expect(warningHeadline(wind)).toBe('Code orange: severe gusts in the south');
    speak('nl');
    expect(warningName({ ...wind, kind: 'thunderstorm' })).toBe('Onweer');
    expect(warningHeadline(wind)).toBe('Code oranje: zware windstoten in het zuiden');
    speak('fr');
    expect(warningName({ ...wind, kind: 'snow-ice' })).toBe('Neige et verglas');
    expect(warningHeadline(wind)).toBeUndefined();
  });

  it('falls back on the KNMI event for a kind GDash does not know', () => {
    expect(warningName({ ...wind, kind: '' })).toBe('Severe Wind');
    speak('nl');
    expect(warningName({ ...wind, kind: 'meteor' })).toBe('Zware windstoten');
  });
});

describe('warningWindow', () => {
  it('says until when while the warning is in effect', () => {
    expect(warningWindow(wind, at('15:00'))).toBe('until Wed 00:00');
    expect(warningWindow({ ...wind, expires: '2026-10-13T20:00:00Z' }, at('15:00'))).toBe('until 22:00');
  });

  it('names the start and end of a warning to come, with the day when it is not today', () => {
    expect(warningWindow(wind, at('09:00'))).toBe('14:00 – Wed 00:00');
    const tomorrow = { ...wind, onset: '2026-10-14T06:00:00Z', expires: '2026-10-14T12:00:00Z' };
    expect(warningWindow(tomorrow, at('09:00'))).toBe('Wed 08:00 – 14:00');
    speak('nl');
    expect(warningWindow(tomorrow, at('09:00'))).toBe('wo 08:00 – 14:00');
  });
});

describe('warningsWaitingText', () => {
  it('asks for a province when the server has none to follow', () => {
    const notSetUp = 'Choose a province for this widget, or set a weather location in the Netherlands under Integrations.';
    expect(warningsWaitingText(undefined, undefined)).toBe(notSetUp);
    expect(warningsWaitingText({ ...entry, value: {} }, undefined)).toBe(notSetUp);
    expect(warningsWaitingText(entry, { region: 'Zeeland' })).toBe('Loading weather warnings…');
    expect(warningsWaitingText({ ...entry, failing: true }, { region: 'Zeeland' })).toBe('Could not load the weather warnings');
  });
});
