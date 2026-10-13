import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Snapshot } from '../protocol';
import { waitingText } from './waiting';

const entry = { fetchedAt: '0001-01-01T00:00:00Z', staleAfter: '0001-01-01T00:00:00Z', failing: false };

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

describe('waitingText', () => {
  it('says what to set up when the server sends no entry', () => {
    expect(waitingText(undefined, 'weather')).toBe('No location set. Choose one under Integrations.');
    expect(waitingText(undefined, 'forecast')).toBe('No location set. Choose one under Integrations.');
    expect(waitingText(undefined, 'calendar')).toBe('No calendar. Add one under Integrations.');
    expect(waitingText(undefined, 'sun')).toBe('No location set. Choose one under Integrations.');
    expect(waitingText(undefined, 'airquality')).toBe('No location set. Choose one under Integrations.');
    expect(waitingText(undefined, 'waste')).toBe('No calendar. Add one under Integrations.');
    expect(waitingText(undefined, 'energyprice')).toBe('Loading the electricity price…');
    expect(waitingText(undefined, 'solar')).toBe('No solar panels set up. Add them under Integrations.');
    expect(waitingText(undefined, 'holidays')).toBe('Loading holidays…');
    expect(waitingText(undefined, 'fuel')).toBe('Set a weather location under Integrations, or choose a place for this widget.');
    expect(waitingText(undefined, 'rain')).toBe('Set a weather location under Integrations, or choose a place for this widget.');
    expect(waitingText(undefined, 'warnings')).toBe(
      'Choose a province for this widget, or set a weather location in the Netherlands under Integrations.',
    );
  });

  it('says loading while the first fetch runs', () => {
    expect(waitingText(entry, 'news')).toBe('Loading news…');
  });

  it('says the fetch failed', () => {
    expect(waitingText({ ...entry, failing: true }, 'weather')).toBe('Could not load the weather');
    expect(waitingText({ ...entry, failing: true }, 'sun')).toBe('Could not load the sun times');
    expect(waitingText({ ...entry, failing: true }, 'airquality')).toBe('Could not load the air quality');
    expect(waitingText({ ...entry, failing: true }, 'waste')).toBe('Could not load the waste collection');
    expect(waitingText({ ...entry, failing: true }, 'energyprice')).toBe('Could not load the electricity price');
    expect(waitingText({ ...entry, failing: true }, 'solar')).toBe('Could not load the solar forecast');
    expect(waitingText({ ...entry, failing: true }, 'fuel')).toBe('Could not load the fuel prices');
    expect(waitingText({ ...entry, failing: true }, 'rain')).toBe('Could not load the rain forecast');
    expect(waitingText({ ...entry, failing: true }, 'warnings')).toBe('Could not load the weather warnings');
  });

  it('speaks the household language', () => {
    speak('nl');
    expect(waitingText(entry, 'calendar')).toBe('Agenda laden…');
    expect(waitingText(entry, 'sun')).toBe('Zonnetijden laden…');
    expect(waitingText(entry, 'airquality')).toBe('Luchtkwaliteit laden…');
    expect(waitingText(entry, 'waste')).toBe('Afvalkalender laden…');
    expect(waitingText(entry, 'energyprice')).toBe('Stroomprijs laden…');
    expect(waitingText(entry, 'fuel')).toBe('Brandstofprijzen laden…');
    expect(waitingText(undefined, 'news')).toBe('Geen nieuwsfeed. Voeg er een toe onder Integrations.');
  });
});
