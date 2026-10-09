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
  previewLogin: 'Log in to the Nookboard web interface to see this preview.',
  previewMissing: 'There is no dashboard "{id}".',
  previewInvalid: 'This layout cannot be shown yet: {reason}',
  previewFailed: 'The preview could not be loaded.',
  pairTitle: 'Pair this screen',
  pairHelp: 'Open Nookboard on your phone or computer, choose this display and enter the code. It is valid for 10 minutes.',
  offline: 'No connection to the Nookboard server',
  announcement: 'Announcement',
  unknownWidget: 'Unknown widget: {type}',
  unknownWeather: 'Unknown weather',
  weatherLoading: 'Loading weather…',
  weatherFailed: 'Could not load the weather',
  weatherNotSetUp: 'No location set. Choose one under Settings.',
  forecastLoading: 'Loading forecast…',
  forecastFailed: 'Could not load the forecast',
  sunLoading: 'Loading sun times…',
  sunFailed: 'Could not load the sun times',
  sunNone: 'No sunrise or sunset today',
  sunrise: 'Sunrise',
  sunset: 'Sunset',
  sunDuration: '{hours} h {minutes} min',
  sunDaylight: '{duration} daylight',
  sunLonger: '{minutes} min longer than yesterday',
  sunShorter: '{minutes} min shorter than yesterday',
  sunSame: 'As long as yesterday',
  airQualityLoading: 'Loading air quality…',
  airQualityFailed: 'Could not load the air quality',
  airQuality: 'Air quality',
  aqiGood: 'Good',
  aqiFair: 'Fair',
  aqiModerate: 'Moderate',
  aqiPoor: 'Poor',
  aqiVeryPoor: 'Very poor',
  aqiExtremelyPoor: 'Extremely poor',
  pollenNone: 'No pollen in the air',
  pollenAlder: 'Alder',
  pollenBirch: 'Birch',
  pollenGrass: 'Grass',
  pollenMugwort: 'Mugwort',
  pollenOlive: 'Olive',
  pollenRagweed: 'Ragweed',
  pollenLow: 'Low',
  pollenModerate: 'Moderate',
  pollenHigh: 'High',
  pollenVeryHigh: 'Very high',
  newsLoading: 'Loading news…',
  newsFailed: 'Could not load the news',
  newsNotSetUp: 'No news feed. Add one under Settings.',
  calendarLoading: 'Loading calendar…',
  calendarFailed: 'Could not load the calendar',
  calendarNotSetUp: 'No calendar. Add one under Settings.',
  photosNone: 'No photos yet. Add them under Settings.',
  haLoading: 'Loading Home Assistant…',
  haFailed: 'Could not reach Home Assistant',
  haNotSetUp: 'Home Assistant is not connected. Connect it under Settings.',
  haMissing: 'Not found in Home Assistant',
  haOn: 'On',
  haOff: 'Off',
  haOpen: 'Open',
  haClosed: 'Closed',
  haDetected: 'Detected',
  haClear: 'Clear',
  haWet: 'Wet',
  haDry: 'Dry',
  haLocked: 'Locked',
  haUnlocked: 'Unlocked',
  haHome: 'Home',
  haAway: 'Away',
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
    previewLogin: 'Log in op de webinterface van Nookboard om dit voorbeeld te zien.',
    previewMissing: 'Er is geen dashboard "{id}".',
    previewInvalid: 'Deze indeling kan nog niet worden getoond: {reason}',
    previewFailed: 'Het voorbeeld kon niet worden geladen.',
    pairTitle: 'Koppel dit scherm',
    pairHelp: 'Open Nookboard op je telefoon of computer, kies deze display en vul de code in. De code is 10 minuten geldig.',
    offline: 'Geen verbinding met de Nookboard-server',
    announcement: 'Mededeling',
    unknownWidget: 'Onbekende widget: {type}',
    unknownWeather: 'Onbekend weer',
    weatherLoading: 'Weer laden…',
    weatherFailed: 'Het weer kon niet worden geladen',
    weatherNotSetUp: 'Geen locatie ingesteld. Kies er een onder Settings.',
    forecastLoading: 'Verwachting laden…',
    forecastFailed: 'De verwachting kon niet worden geladen',
    sunLoading: 'Zonnetijden laden…',
    sunFailed: 'De zonnetijden konden niet worden geladen',
    sunNone: 'Vandaag geen zonsopkomst of -ondergang',
    sunrise: 'Zonsopkomst',
    sunset: 'Zonsondergang',
    sunDuration: '{hours} u {minutes} min',
    sunDaylight: '{duration} daglicht',
    sunLonger: '{minutes} min langer dan gisteren',
    sunShorter: '{minutes} min korter dan gisteren',
    sunSame: 'Even lang als gisteren',
    airQualityLoading: 'Luchtkwaliteit laden…',
    airQualityFailed: 'De luchtkwaliteit kon niet worden geladen',
    airQuality: 'Luchtkwaliteit',
    aqiGood: 'Goed',
    aqiFair: 'Redelijk',
    aqiModerate: 'Matig',
    aqiPoor: 'Slecht',
    aqiVeryPoor: 'Zeer slecht',
    aqiExtremelyPoor: 'Extreem slecht',
    pollenNone: 'Geen pollen in de lucht',
    pollenAlder: 'Els',
    pollenBirch: 'Berk',
    pollenGrass: 'Gras',
    pollenMugwort: 'Bijvoet',
    pollenOlive: 'Olijf',
    pollenRagweed: 'Ambrosia',
    pollenLow: 'Laag',
    pollenModerate: 'Matig',
    pollenHigh: 'Hoog',
    pollenVeryHigh: 'Zeer hoog',
    newsLoading: 'Nieuws laden…',
    newsFailed: 'Het nieuws kon niet worden geladen',
    newsNotSetUp: 'Geen nieuwsfeed. Voeg er een toe onder Settings.',
    calendarLoading: 'Agenda laden…',
    calendarFailed: 'De agenda kon niet worden geladen',
    calendarNotSetUp: 'Geen agenda. Voeg er een toe onder Settings.',
    photosNone: "Nog geen foto's. Voeg ze toe onder Settings.",
    haLoading: 'Home Assistant laden…',
    haFailed: 'Home Assistant is niet bereikbaar',
    haNotSetUp: 'Home Assistant is niet gekoppeld. Koppel het onder Settings.',
    haMissing: 'Niet gevonden in Home Assistant',
    haOn: 'Aan',
    haOff: 'Uit',
    haOpen: 'Open',
    haClosed: 'Dicht',
    haDetected: 'Beweging',
    haClear: 'Geen beweging',
    haWet: 'Nat',
    haDry: 'Droog',
    haLocked: 'Op slot',
    haUnlocked: 'Niet op slot',
    haHome: 'Thuis',
    haAway: 'Weg',
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
