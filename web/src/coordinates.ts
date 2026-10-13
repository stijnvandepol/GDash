const PLACE_DECIMALS = 3;

/**
 * A latitude or longitude rounded to three decimals, about 100 m, the way the
 * server rounds a place (internal/fuel.Round): half away from zero, and
 * never a negative zero. The editor stores places rounded, so every widget
 * at the same town shares one request. Rain rounds to two decimals
 * (internal/rain.Round).
 */
export function roundCoordinate(degrees: number, decimals = PLACE_DECIMALS): number {
  const scale = 10 ** decimals;
  const rounded = (Math.sign(degrees) * Math.round(Math.abs(degrees) * scale)) / scale;
  return rounded === 0 ? 0 : rounded;
}

/** The rounded coordinate as text, such as "51.765", as the server writes it. */
export function coordinate(degrees: number, decimals = PLACE_DECIMALS): string {
  return String(roundCoordinate(degrees, decimals));
}
