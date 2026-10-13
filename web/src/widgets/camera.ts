/** How often a Camera widget asks for a new image without a setting; internal/display holds the same. */
export const DEFAULT_SECONDS = 10;
const MIN_SECONDS = 5;

/** The camera a widget shows: a whole number from 1 up, or undefined. */
export function cameraId(settings: Record<string, unknown> | undefined): number | undefined {
  const id = settings?.camera;
  return typeof id === 'number' && Number.isInteger(id) && id >= 1 ? id : undefined;
}

/** Seconds between images, never below the server's minimum. */
export function refreshSeconds(settings: Record<string, unknown> | undefined): number {
  const seconds = settings?.seconds;
  return typeof seconds === 'number' && seconds >= MIN_SECONDS ? seconds : DEFAULT_SECONDS;
}

/** What the server says about an image: when it arrived and whether the camera failed since. */
export interface Shot {
  takenAt: Date | null;
  stale: boolean;
}

/** Reads a camera image's headers: Last-Modified, and X-Image-Stale when the camera failed after it. */
export function readShot(headers: Headers): Shot {
  const modified = headers.get('Last-Modified');
  const ms = modified ? Date.parse(modified) : NaN;
  return { takenAt: Number.isNaN(ms) ? null : new Date(ms), stale: headers.get('X-Image-Stale') === 'true' };
}

/** The time an image arrived, as "14:02" in the screen's language, for the stale mark. */
export function shotTime(takenAt: Date, locale: string): string {
  return new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit' }).format(takenAt);
}
