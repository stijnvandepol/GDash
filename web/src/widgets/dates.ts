const DAY_MS = 86_400_000;

/** The local midnight of a YYYY-MM-DD date, or null when it is not one. */
export function parseLocalDate(date: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
  if (!match) return null;
  const [y, m, d] = [Number(match[1]), Number(match[2]), Number(match[3])];
  const result = new Date(y, m - 1, d);
  // new Date rolls 2026-02-30 over to March; that is not a valid date.
  return result.getMonth() === m - 1 && result.getDate() === d ? result : null;
}

/** Whole days from the local day of now until a YYYY-MM-DD date. */
export function daysUntil(date: string, now: Date): number | null {
  const target = parseLocalDate(date);
  if (!target) return null;
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  // Rounding absorbs the hour that a daylight saving change adds or removes.
  return Math.round((target.getTime() - today.getTime()) / DAY_MS);
}
