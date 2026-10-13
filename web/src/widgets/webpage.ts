import type { Entry } from '../protocol';

/** A page that has not loaded after this long shows the placeholder. */
export const LOAD_TIMEOUT_MS = 20_000;

const MIN_ZOOM = 50;
const MAX_ZOOM = 200;
const DEFAULT_ZOOM = 100;
const MAX_REFRESH_MINUTES = 24 * 60;
const MINUTE_MS = 60_000;

/**
 * A page that may run scripts keeps its origin, so its own logins and
 * storage work; it never gets top navigation, popups, modal dialogs or
 * downloads. A page without scripts is sandboxed completely.
 */
const SANDBOX_WITH_SCRIPTS = 'allow-scripts allow-same-origin allow-forms';
const SANDBOX_WITHOUT_SCRIPTS = '';

/** What a Web page widget's frame shows. */
export interface PageFrame {
  name: string;
  src: string;
  sandbox: string;
  /** Size of the page in percent. */
  zoom: number;
  /** Milliseconds between reloads; 0 never reloads. */
  reloadMs: number;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function zoomOf(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isInteger(v) && v >= MIN_ZOOM && v <= MAX_ZOOM ? v : undefined;
}

/**
 * The frame of a Web page widget: its page from the snapshot, by the ID in
 * its settings. Null when the page is not there (removed under Integrations),
 * is not http or https, or is on ownOrigin: a frame that runs scripts and
 * keeps GDash's origin could reach out of its sandbox. The server refuses
 * such pages too; this is the screen's own check.
 */
export function pageFrame(entry: Entry | undefined, settings: Record<string, unknown> | undefined, ownOrigin: string): PageFrame | null {
  const id = settings?.page;
  if (typeof id !== 'number' || !isRecord(entry?.value)) return null;
  const page = entry.value[String(id)];
  if (!isRecord(page) || typeof page.url !== 'string') return null;
  let url: URL;
  try {
    url = new URL(page.url);
  } catch {
    return null;
  }
  if ((url.protocol !== 'http:' && url.protocol !== 'https:') || url.origin === ownOrigin) return null;
  const minutes = page.refreshMinutes;
  const reloads = typeof minutes === 'number' && Number.isInteger(minutes) && minutes >= 1 && minutes <= MAX_REFRESH_MINUTES;
  return {
    name: typeof page.name === 'string' ? page.name : '',
    src: url.href,
    sandbox: page.allowScripts === true ? SANDBOX_WITH_SCRIPTS : SANDBOX_WITHOUT_SCRIPTS,
    zoom: zoomOf(settings?.zoom) ?? zoomOf(page.zoom) ?? DEFAULT_ZOOM,
    reloadMs: reloads ? minutes * MINUTE_MS : 0,
  };
}

/**
 * The frame's size and scale for a zoom: at 50% the page is laid out twice
 * as large as the tile and drawn at half size, so more of it fits.
 */
export function frameScale(zoom: number): { size: string; scale: number } {
  return { size: `${(100 * 100) / zoom}%`, scale: zoom / 100 };
}

/** The display page reloads for a blocked frame at most this often. */
export const FRAME_RELOAD_GAP_MS = 10 * MINUTE_MS;

/**
 * Whether the display page should reload because the browser blocked a
 * web page's frame: the page's policy was made before the widget was
 * added, or before this screen was paired. Not again within the gap, so a
 * frame that stays blocked does not make the page reload forever.
 */
export function shouldReloadForFrames(lastReloadMs: number | null, nowMs: number): boolean {
  return lastReloadMs === null || nowMs - lastReloadMs >= FRAME_RELOAD_GAP_MS || nowMs < lastReloadMs;
}
