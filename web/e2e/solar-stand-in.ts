// What e2e/upstream.ts answers for Open-Meteo's solar forecast, which the
// server asks at GDASH_TEST_UPSTREAM's /open-meteo-solar/v1/forecast: the
// same sunshine on the panels every day, in a made-up time zone where it
// is about 10 in the morning, so a test never runs across midnight at the
// panels.

/** W/m² on the panels, the means of the hours that end at 07:00 to 19:00. */
export const TODAY_IRRADIANCE = [50, 150, 300, 450, 600, 700, 750, 700, 600, 450, 300, 150, 50];
/** Tomorrow is cloudier: 40% of today. */
const TOMORROW_SHARE = 0.4;
const FIRST_SUNNY_LABEL = 7;
const LOCAL_HOUR = 10;

const pad = (n: number) => String(n).padStart(2, '0');

/** The answer at nowMs: two local days of hourly irradiance and the made-up offset. */
export function solarAnswer(nowMs: number): unknown {
  const offsetHours = LOCAL_HOUR - new Date(nowMs).getUTCHours();
  const localMidnight = new Date(nowMs + offsetHours * 3_600_000);
  localMidnight.setUTCHours(0, 0, 0, 0);
  const time: string[] = [];
  const irradiance: number[] = [];
  for (let i = 0; i < 48; i++) {
    const at = new Date(localMidnight.getTime() + i * 3_600_000);
    time.push(`${at.toISOString().slice(0, 10)}T${pad(at.getUTCHours())}:00`);
    const sun = TODAY_IRRADIANCE[(i % 24) - FIRST_SUNNY_LABEL] ?? 0;
    irradiance.push(i < 24 ? sun : Math.round(sun * TOMORROW_SHARE * 10) / 10);
  }
  return { utc_offset_seconds: offsetHours * 3600, hourly: { time, global_tilted_irradiance: irradiance } };
}
