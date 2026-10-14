// OLED care, for a display that has it on (Snapshot.screenCare "oled"). An
// OLED pixel ages by how long and how bright it is lit, so what stays in
// one place all day, such as tile borders, titles and the clock's colon,
// burns in first. Against that the stage drifts slowly instead of jumping,
// and once an hour the whole screen is black for a second, which gives the
// panel's own pixel refresh a chance and makes no pixel lit around the
// clock. The dimming of static chrome and the slow breathing of brightness
// are CSS, in App.svelte.

/** How far the stage drifts from its place, in stage pixels, either way. */
export const DRIFT_PX = 8;

// Two periods without a common divisor, so the path only repeats after
// hours and covers the whole square on the way.
const DRIFT_PERIOD_X_MS = 23 * 60_000;
const DRIFT_PERIOD_Y_MS = 37 * 60_000;

/** How long the screen is black for the hourly refresh. */
export const REFRESH_MS = 1000;
const HOUR_MS = 3_600_000;
// Half a minute past the hour, so the clock's turn of the hour is seen.
const REFRESH_OFFSET_MS = 30_000;
// How early a browser timer may fire.
const EARLY_TIMER_MS = 1000;

/**
 * Where the stage is at nowMs, in stage pixels: a slow loop through the
 * square of DRIFT_PX either way, rounded to a tenth of a pixel. The screen
 * moves to the next spot over a minute, so the drift is never a jump.
 */
export function driftAt(nowMs: number): [number, number] {
  const x = DRIFT_PX * Math.sin((2 * Math.PI * nowMs) / DRIFT_PERIOD_X_MS);
  const y = DRIFT_PX * Math.sin((2 * Math.PI * nowMs) / DRIFT_PERIOD_Y_MS + Math.PI / 4);
  return [Math.round(x * 10) / 10, Math.round(y * 10) / 10];
}

/**
 * How long from nowMs to the next hourly refresh. A timer that fires a
 * little early does not refresh twice: under a second counts as the hour
 * after.
 */
export function msToNextRefresh(nowMs: number): number {
  const into = (((nowMs - REFRESH_OFFSET_MS) % HOUR_MS) + HOUR_MS) % HOUR_MS;
  const left = HOUR_MS - into;
  return left < EARLY_TIMER_MS ? left + HOUR_MS : left;
}
