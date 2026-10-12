import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { currentLanguage, LANGUAGES, locale, t, TEXTS } from './i18n.svelte';
import { display } from './state.svelte';
import type { Snapshot } from './protocol';
import { describeWeather } from './widgets/wmo';

function speak(language: string) {
  display.snapshot = { language } as Snapshot;
}

const WEATHER_CODES = [0, 1, 2, 3, 45, 48, 51, 53, 55, 56, 57, 61, 63, 65, 66, 67, 71, 73, 75, 77, 80, 81, 82, 85, 86, 95, 96, 99];

/** The {name} placeholders of a text, sorted. */
function placeholders(text: string): string[] {
  return (text.match(/\{\w+\}/g) ?? []).sort();
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
    speak('es');
    expect(currentLanguage()).toBe('en');
    display.snapshot = null;
    expect(t('pairTitle')).toBe('Connect this screen');
    // A TV set to Dutch shows its pairing screen in Dutch.
    vi.stubGlobal('navigator', { language: 'nl-BE' });
    expect(t('pairTitle')).toBe('Verbind dit scherm');
    vi.stubGlobal('navigator', { language: 'de-AT' });
    expect(t('pairTitle')).toBe('Diesen Bildschirm verbinden');
    vi.stubGlobal('navigator', { language: 'fr' });
    expect(t('pairTitle')).toBe('Connecter cet écran');
  });

  it('tells a new screen where to enter its code', () => {
    // The web interface shows each display's address, so every language names it the same way.
    for (const language of LANGUAGES) {
      speak(language);
      const help = t('pairHelp', { address: '192.168.1.20:8420', id: 'kitchen' });
      expect(help, language).toContain('192.168.1.20:8420');
      expect(help, language).toContain('/d/kitchen');
    }
  });

  it('fills in placeholders', () => {
    expect(t('updated', { time: '10:31' })).toBe('Updated 10:31');
    speak('nl');
    expect(t('wind', { speed: 14 })).toBe('Wind 14 km/u');
  });

  it('inserts values literally', () => {
    expect(t('unknownWidget', { type: '$&x' })).toBe('Unknown widget: $&x');
  });

  it('has words for every weather code in every language', () => {
    for (const language of LANGUAGES) {
      for (const code of WEATHER_CODES) {
        expect(describeWeather(code, language), `${language} code ${code}`).toBeTruthy();
      }
      expect(describeWeather(1234, language)).toBeUndefined();
    }
  });
});

describe('every language', () => {
  const english = Object.keys(TEXTS.en).sort();

  it.each(LANGUAGES.filter((language) => language !== 'en'))('%s has exactly the texts English has', (language) => {
    expect(Object.keys(TEXTS[language]).sort()).toEqual(english);
  });

  it.each(LANGUAGES)('%s has no empty text and keeps the placeholders of English', (language) => {
    for (const [key, text] of Object.entries(TEXTS[language])) {
      expect(text.trim(), `${language} ${key}`).not.toBe('');
      expect(placeholders(text), `${language} ${key}`).toEqual(placeholders(TEXTS.en[key as keyof typeof TEXTS.en]));
    }
  });

  it('has a different weather word than English for most codes', () => {
    // A guard against a language that was copied from English and never translated.
    for (const language of LANGUAGES.filter((l) => l !== 'en')) {
      const same = WEATHER_CODES.filter((code) => describeWeather(code, language) === describeWeather(code, 'en'));
      expect(same.length, language).toBeLessThan(WEATHER_CODES.length / 4);
    }
  });
});

describe('German', () => {
  it('speaks German with German dates', () => {
    speak('de');
    expect(currentLanguage()).toBe('de');
    expect(locale()).toBe('de-DE');
    expect(t('tomorrow')).toBe('Morgen');
    expect(t('wind', { speed: 14 })).toBe('Wind 14 km/h');
    expect(t('sunDuration', { hours: 11, minutes: 5 })).toBe('11 Std. 5 Min.');
    expect(describeWeather(95, 'de')).toBe('Gewitter');
    const monday = Date.UTC(2026, 9, 12, 12);
    const date = new Intl.DateTimeFormat(locale(), { weekday: 'long', day: 'numeric', month: 'long', timeZone: 'UTC' });
    expect(date.format(monday)).toBe('Montag, 12. Oktober');
    expect(new Intl.DateTimeFormat(locale(), { weekday: 'short', timeZone: 'UTC' }).format(monday)).toBe('Mo');
  });
});

describe('French', () => {
  it('speaks French with French dates', () => {
    speak('fr');
    expect(currentLanguage()).toBe('fr');
    expect(locale()).toBe('fr-FR');
    expect(t('today')).toBe('Aujourd’hui');
    expect(t('sunLonger', { minutes: 3 })).toBe('3 min de plus qu’hier');
    expect(t('unknownWidget', { type: 'x' })).toBe('Widget inconnu : x');
    expect(describeWeather(61, 'fr')).toBe('Pluie légère');
    const monday = Date.UTC(2026, 9, 12, 12);
    const date = new Intl.DateTimeFormat(locale(), { weekday: 'long', day: 'numeric', month: 'long', timeZone: 'UTC' });
    expect(date.format(monday)).toBe('lundi 12 octobre');
    expect(new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit', timeZone: 'UTC' }).format(Date.UTC(2026, 9, 12, 7, 5))).toBe('07:05');
  });
});
