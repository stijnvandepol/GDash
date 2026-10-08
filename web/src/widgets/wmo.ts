// WMO weather interpretation codes as used by Open-Meteo
// (https://open-meteo.com/en/docs, section "WMO Weather interpretation codes").
const descriptions: Record<number, string> = {
  0: 'Helder',
  1: 'Overwegend helder',
  2: 'Half bewolkt',
  3: 'Bewolkt',
  45: 'Mist',
  48: 'Rijpmist',
  51: 'Lichte motregen',
  53: 'Motregen',
  55: 'Dichte motregen',
  56: 'Lichte ijzel',
  57: 'IJzel',
  61: 'Lichte regen',
  63: 'Regen',
  65: 'Zware regen',
  66: 'Lichte ijzelregen',
  67: 'IJzelregen',
  71: 'Lichte sneeuw',
  73: 'Sneeuw',
  75: 'Zware sneeuw',
  77: 'Sneeuwkorrels',
  80: 'Lichte buien',
  81: 'Buien',
  82: 'Zware buien',
  85: 'Sneeuwbuien',
  86: 'Zware sneeuwbuien',
  95: 'Onweer',
  96: 'Onweer met hagel',
  99: 'Zwaar onweer met hagel',
};

export function describeWeather(code: number): string {
  return descriptions[code] ?? 'Onbekend weer';
}
