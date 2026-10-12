const SCALE = 1000;

/**
 * A latitude or longitude rounded to three decimals, about 100 m, the way the
 * server rounds a place (internal/fuel.Round): half away from zero, and
 * never a negative zero. The editor stores places rounded, so every widget
 * at the same town shares one request.
 */
export function roundCoordinate(degrees: number): number {
  const rounded = (Math.sign(degrees) * Math.round(Math.abs(degrees) * SCALE)) / SCALE;
  return rounded === 0 ? 0 : rounded;
}

/** The rounded coordinate as text, such as "51.765", as the server writes it. */
export function coordinate(degrees: number): string {
  return String(roundCoordinate(degrees));
}
