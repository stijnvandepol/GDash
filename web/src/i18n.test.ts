import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { currentLanguage, locale, t } from './i18n.svelte';
import { display } from './state.svelte';
import type { Snapshot } from './protocol';
import { describeWeather } from './widgets/wmo';

function speak(language: string) {
  display.snapshot = { language } as Snapshot;
}

// Node has a navigator with the language of the machine; pin it, so the
// tests do not depend on where they run.
beforeEach(() => {
  vi.stubGlobal('navigator', { language: 'en-US' });
});

afterEach(() => {
  display.snapshot = null;
  vi.unstubAllGlobals();
});

describe('i18n', () => {
  it('uses the language from the snapshot', () => {
    speak('nl');
    expect(currentLanguage()).toBe('nl');
    expect(t('today')).toBe('Vandaag');
    expect(locale()).toBe('nl-NL');
  });

  it('falls back to the device language for an unknown or missing one', () => {
    speak('fr');
    expect(currentLanguage()).toBe('en');
    display.snapshot = null;
    expect(t('pairTitle')).toBe('Pair this screen');
    // A TV set to Dutch shows its pairing screen in Dutch.
    vi.stubGlobal('navigator', { language: 'nl-BE' });
    expect(t('pairTitle')).toBe('Koppel dit scherm');
  });

  it('fills in placeholders', () => {
    expect(t('updated', { time: '10:31' })).toBe('Updated 10:31');
    speak('nl');
    expect(t('wind', { speed: 14 })).toBe('Wind 14 km/u');
  });

  it('inserts values literally', () => {
    expect(t('unknownWidget', { type: '$&x' })).toBe('Unknown widget: $&x');
  });

  it('has Dutch words for every weather code', () => {
    for (const code of [0, 1, 2, 3, 45, 48, 51, 53, 55, 56, 57, 61, 63, 65, 66, 67, 71, 73, 75, 77, 80, 81, 82, 85, 86, 95, 96, 99]) {
      expect(describeWeather(code, 'nl'), `code ${code}`).toBeTruthy();
      expect(describeWeather(code, 'en'), `code ${code}`).toBeTruthy();
    }
    expect(describeWeather(1234, 'nl')).toBeUndefined();
  });
});
