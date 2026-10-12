// WMO weather interpretation codes as used by Open-Meteo
// (https://open-meteo.com/en/docs, section "WMO Weather interpretation codes").
import type { Language } from '../i18n.svelte';

const english: Record<number, string> = {
  0: 'Clear sky',
  1: 'Mainly clear',
  2: 'Partly cloudy',
  3: 'Overcast',
  45: 'Fog',
  48: 'Freezing fog',
  51: 'Light drizzle',
  53: 'Drizzle',
  55: 'Dense drizzle',
  56: 'Light freezing drizzle',
  57: 'Freezing drizzle',
  61: 'Light rain',
  63: 'Rain',
  65: 'Heavy rain',
  66: 'Light freezing rain',
  67: 'Freezing rain',
  71: 'Light snow',
  73: 'Snow',
  75: 'Heavy snow',
  77: 'Snow grains',
  80: 'Light showers',
  81: 'Showers',
  82: 'Heavy showers',
  85: 'Snow showers',
  86: 'Heavy snow showers',
  95: 'Thunderstorm',
  96: 'Thunderstorm with hail',
  99: 'Severe thunderstorm with hail',
};

// The same codes in the words of Dutch weather reports.
const dutch: Record<number, string> = {
  0: 'Onbewolkt',
  1: 'Overwegend helder',
  2: 'Half bewolkt',
  3: 'Bewolkt',
  45: 'Mist',
  48: 'Aanvriezende mist',
  51: 'Lichte motregen',
  53: 'Motregen',
  55: 'Dichte motregen',
  56: 'Lichte aanvriezende motregen',
  57: 'Aanvriezende motregen',
  61: 'Lichte regen',
  63: 'Regen',
  65: 'Zware regen',
  66: 'Lichte ijzel',
  67: 'IJzel',
  71: 'Lichte sneeuw',
  73: 'Sneeuw',
  75: 'Zware sneeuw',
  77: 'Motsneeuw',
  80: 'Lichte buien',
  81: 'Buien',
  82: 'Zware buien',
  85: 'Sneeuwbuien',
  86: 'Zware sneeuwbuien',
  95: 'Onweer',
  96: 'Onweer met hagel',
  99: 'Zwaar onweer met hagel',
};

// The same codes in the words of German weather reports.
const german: Record<number, string> = {
  0: 'Wolkenlos',
  1: 'Überwiegend klar',
  2: 'Teils bewölkt',
  3: 'Bedeckt',
  45: 'Nebel',
  48: 'Reifnebel',
  51: 'Leichter Nieselregen',
  53: 'Nieselregen',
  55: 'Starker Nieselregen',
  56: 'Leichter gefrierender Nieselregen',
  57: 'Gefrierender Nieselregen',
  61: 'Leichter Regen',
  63: 'Regen',
  65: 'Starker Regen',
  66: 'Leichter Eisregen',
  67: 'Eisregen',
  71: 'Leichter Schneefall',
  73: 'Schneefall',
  75: 'Starker Schneefall',
  77: 'Schneegriesel',
  80: 'Leichte Schauer',
  81: 'Schauer',
  82: 'Heftige Schauer',
  85: 'Schneeschauer',
  86: 'Starke Schneeschauer',
  95: 'Gewitter',
  96: 'Gewitter mit Hagel',
  99: 'Schweres Gewitter mit Hagel',
};

// The same codes in the words of Météo-France.
const french: Record<number, string> = {
  0: 'Ciel dégagé',
  1: 'Plutôt dégagé',
  2: 'Partiellement nuageux',
  3: 'Couvert',
  45: 'Brouillard',
  48: 'Brouillard givrant',
  51: 'Bruine légère',
  53: 'Bruine',
  55: 'Bruine dense',
  56: 'Bruine verglaçante légère',
  57: 'Bruine verglaçante',
  61: 'Pluie légère',
  63: 'Pluie',
  65: 'Forte pluie',
  66: 'Pluie verglaçante légère',
  67: 'Pluie verglaçante',
  71: 'Neige légère',
  73: 'Neige',
  75: 'Forte neige',
  77: 'Grains de neige',
  80: 'Averses légères',
  81: 'Averses',
  82: 'Fortes averses',
  85: 'Averses de neige',
  86: 'Fortes averses de neige',
  95: 'Orage',
  96: 'Orage avec grêle',
  99: 'Violent orage avec grêle',
};

const DESCRIPTIONS: Record<Language, Record<number, string>> = { en: english, nl: dutch, de: german, fr: french };

/** The words for a WMO weather code, or undefined for an unknown code. */
export function describeWeather(code: number, language: Language): string | undefined {
  return DESCRIPTIONS[language][code];
}

/** The icon groups WeatherIcon draws. */
export type IconKind = 'sun' | 'moon' | 'partly' | 'cloud' | 'fog' | 'drizzle' | 'rain' | 'snow' | 'thunder';

/** The icon for a WMO code; a clear night gets a moon. Unknown codes get a cloud. */
export function iconFor(code: number, day = true): IconKind {
  if (code === 0) return day ? 'sun' : 'moon';
  if (code === 1 || code === 2) return 'partly';
  if (code === 45 || code === 48) return 'fog';
  if (code >= 51 && code <= 57) return 'drizzle';
  if ((code >= 61 && code <= 67) || (code >= 80 && code <= 82)) return 'rain';
  if ((code >= 71 && code <= 77) || code === 85 || code === 86) return 'snow';
  if (code >= 95 && code <= 99) return 'thunder';
  return 'cloud';
}
