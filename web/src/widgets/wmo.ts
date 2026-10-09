// WMO weather interpretation codes as used by Open-Meteo
// (https://open-meteo.com/en/docs, section "WMO Weather interpretation codes").
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

/** The words for a WMO weather code, or undefined for an unknown code. */
export function describeWeather(code: number, language: 'en' | 'nl'): string | undefined {
  return (language === 'nl' ? dutch : english)[code];
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
