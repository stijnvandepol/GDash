// Texts on the TV, in the display languages the server offers
// (internal/settings.Languages). The web interface for admins stays English,
// so "Settings" is left untranslated where a text points there.
import { display } from './state.svelte';

export type Language = 'en' | 'nl';

const LOCALES: Record<Language, string> = { en: 'en-GB', nl: 'nl-NL' };

const en = {
  today: 'Today',
  tomorrow: 'Tomorrow',
  allDay: 'All day',
  nothingPlanned: 'Nothing planned this week.',
  news: 'News',
  updated: 'Updated {time}',
  wind: 'Wind {speed} km/h',
  daysToGo: 'days to go',
  dayToGo: 'day to go',
  hasPassed: 'Has passed',
  connecting: 'Connecting to Nookboard…',
  noDisplay: 'There is no display "{id}". Check the address in the Nookboard web interface.',
  pairTitle: 'Pair this screen',
  pairHelp: 'Open Nookboard on your phone or computer, choose this display and enter the code. It is valid for 10 minutes.',
  offline: 'No connection to the Nookboard server',
  unknownWidget: 'Unknown widget: {type}',
  unknownWeather: 'Unknown weather',
  weatherLoading: 'Loading weather…',
  weatherFailed: 'Could not load the weather',
  weatherNotSetUp: 'No location set. Choose one under Settings.',
  forecastLoading: 'Loading forecast…',
  forecastFailed: 'Could not load the forecast',
  newsLoading: 'Loading news…',
  newsFailed: 'Could not load the news',
  newsNotSetUp: 'No news feed. Add one under Settings.',
  calendarLoading: 'Loading calendar…',
  calendarFailed: 'Could not load the calendar',
  calendarNotSetUp: 'No calendar. Add one under Settings.',
  photosNone: 'No photos yet. Add them under Settings.',
};

export type TextKey = keyof typeof en;

const TEXTS: Record<Language, Record<TextKey, string>> = {
  en,
  nl: {
    today: 'Vandaag',
    tomorrow: 'Morgen',
    allDay: 'Hele dag',
    nothingPlanned: 'Niets gepland deze week.',
    news: 'Nieuws',
    updated: 'Bijgewerkt {time}',
    wind: 'Wind {speed} km/u',
    daysToGo: 'dagen te gaan',
    dayToGo: 'dag te gaan',
    hasPassed: 'Voorbij',
    connecting: 'Verbinden met Nookboard…',
    noDisplay: 'Er is geen display "{id}". Controleer het adres in de webinterface van Nookboard.',
    pairTitle: 'Koppel dit scherm',
    pairHelp: 'Open Nookboard op je telefoon of computer, kies deze display en vul de code in. De code is 10 minuten geldig.',
    offline: 'Geen verbinding met de Nookboard-server',
    unknownWidget: 'Onbekende widget: {type}',
    unknownWeather: 'Onbekend weer',
    weatherLoading: 'Weer laden…',
    weatherFailed: 'Het weer kon niet worden geladen',
    weatherNotSetUp: 'Geen locatie ingesteld. Kies er een onder Settings.',
    forecastLoading: 'Verwachting laden…',
    forecastFailed: 'De verwachting kon niet worden geladen',
    newsLoading: 'Nieuws laden…',
    newsFailed: 'Het nieuws kon niet worden geladen',
    newsNotSetUp: 'Geen nieuwsfeed. Voeg er een toe onder Settings.',
    calendarLoading: 'Agenda laden…',
    calendarFailed: 'De agenda kon niet worden geladen',
    calendarNotSetUp: 'Geen agenda. Voeg er een toe onder Settings.',
    photosNone: "Nog geen foto's. Voeg ze toe onder Settings.",
  },
};

/**
 * The device's own language, for the pairing screen: it shows before the
 * first snapshot says which language the household chose.
 */
function deviceLanguage(): Language {
  const tag = typeof navigator === 'undefined' ? '' : navigator.language.toLowerCase();
  return tag.startsWith('nl') ? 'nl' : 'en';
}

/** The language of the screen: the household's choice, else the device's. */
export function currentLanguage(): Language {
  const chosen = display.snapshot?.language;
  return chosen === 'en' || chosen === 'nl' ? chosen : deviceLanguage();
}

/** The text for key in the current language, with {name} placeholders filled in. */
export function t(key: TextKey, values: Record<string, string | number> = {}): string {
  let text = TEXTS[currentLanguage()][key];
  // A function as replacement: a value such as "$&" is inserted as it is.
  for (const [name, value] of Object.entries(values)) text = text.replace(`{${name}}`, () => String(value));
  return text;
}

/** The BCP 47 locale for dates and times in the current language. */
export function locale(): string {
  return LOCALES[currentLanguage()];
}
