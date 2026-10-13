import { locale, t } from '../i18n.svelte';

/** What a KNMI station measured, as the server sends it (internal/knmi.Observation). */
export interface Observation {
  station: string;
  observedAt: string;
  temperatureC: number;
  humidity: number | null;
  windKmh: number | null;
}

/** Whether a Weather widget shows measured values instead of the model's. */
export function showsMeasured(settings: Record<string, unknown> | undefined): boolean {
  return settings?.current === 'knmi';
}

const orNull = (v: unknown): number | null => (typeof v === 'number' && Number.isFinite(v) ? v : null);

/** The observation in the server's value, or undefined when it has none with a temperature. */
export function readObservation(value: unknown): Observation | undefined {
  if (typeof value !== 'object' || value === null) return undefined;
  const o = value as Record<string, unknown>;
  if (typeof o.station !== 'string' || typeof o.observedAt !== 'string' || orNull(o.temperatureC) === null) return undefined;
  if (Number.isNaN(Date.parse(o.observedAt))) return undefined;
  return { station: o.station, observedAt: o.observedAt, temperatureC: o.temperatureC as number, humidity: orNull(o.humidity), windKmh: orNull(o.windKmh) };
}

/** "Measured at Volkel 10:20". */
export function measuredText(observation: Observation): string {
  const time = new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit' }).format(new Date(observation.observedAt));
  return t('measuredAt', { station: observation.station, time });
}

/** "Humidity 87%". */
export function humidityText(percent: number): string {
  return t('humidity', { percent: Math.round(percent) });
}
