// The report a TV page sends about its own browser after it has drawn the
// dashboard, and again after it reconnects (internal/screenreport). A TV's
// built-in browser is hard to inspect; this is how the Displays page shows
// why a dashboard looks different there. It holds nothing about the
// household: the browser, the screen size, which CSS features exist, and
// script errors without the page's address.
import type { Features } from './compat';

/** Limits of internal/screenreport, which refuses anything longer. */
const MAX_ERRORS = 5;
const MAX_USER_AGENT = 512;
const MAX_MESSAGE = 300;
const MAX_SOURCE = 200;
const MAX_FIRST_RENDER_MS = 3_600_000;
const MAX_VISIBILITY_EVENTS = 10;

export interface ScriptError {
  message: string;
  /** Where in GDash's scripts: "/assets/App-x.js:1:200". */
  source: string;
}

/** A change of the page's visibility or focus. */
export interface VisibilityEvent {
  state: 'hidden' | 'visible' | 'blur' | 'focus';
  /** When it happened, as an ISO time. */
  at: string;
}

/** Why a screen reports: it connected, or its page was just hidden. */
export type ReportReason = 'connected' | 'hidden';

export interface ScreenReport {
  reason: ReportReason;
  userAgent: string;
  viewport: { width: number; height: number };
  devicePixelRatio: number;
  stageScale: number;
  features: Features;
  /** Whether the page uses its fallback for container queries. */
  compat: boolean;
  fontsLoaded: boolean;
  firstRenderMs: number | null;
  errors: ScriptError[];
  /** The last changes of the page's visibility and focus, oldest first. */
  visibility: VisibilityEvent[];
}

/** Text in one line without control characters, cut to max characters. */
export function plainText(text: string, max: number): string {
  // The class holds the C0 and C1 control characters.
  const line = text.replace(/[\u0000-\u001f\u007f-\u009f]+/g, ' ').replace(/\s+/g, ' ').trim();
  const chars = [...line];
  return chars.length <= max ? line : chars.slice(0, max - 1).join('') + '…';
}

/** Where an error happened, by path only: a page address could hold a launch token. */
export function scriptSource(filename: string, line: number, column: number): string {
  if (!filename) return '';
  let path: string;
  try {
    path = new URL(filename).pathname;
  } catch {
    return '';
  }
  return plainText(`${path}:${line}:${column}`, MAX_SOURCE);
}

/** The first MAX_ERRORS uncaught errors of the page: the first is usually the cause. */
export function createErrorLog() {
  const errors: ScriptError[] = [];
  return {
    add(message: string, source: string): void {
      if (errors.length >= MAX_ERRORS) return;
      errors.push({ message: plainText(message, MAX_MESSAGE) || 'Unknown error', source: plainText(source, MAX_SOURCE) });
    },
    list(): ScriptError[] {
      return errors.map((e) => ({ ...e }));
    },
  };
}

/** The last MAX_VISIBILITY_EVENTS changes of the page's visibility and focus. */
export function createVisibilityLog() {
  const events: VisibilityEvent[] = [];
  return {
    add(state: VisibilityEvent['state'], at: Date): void {
      events.push({ state, at: at.toISOString() });
      if (events.length > MAX_VISIBILITY_EVENTS) events.shift();
    },
    list(): VisibilityEvent[] {
      return events.map((e) => ({ ...e }));
    },
  };
}

/** A message for a rejected promise, whatever it was rejected with. */
export function rejectionMessage(reason: unknown): string {
  if (reason instanceof Error) return `${reason.name}: ${reason.message}`;
  return `Unhandled rejection: ${String(reason)}`;
}

/** What the page measured, before it is cut to the limits the server takes. */
export interface Measured {
  reason: ReportReason;
  userAgent: string;
  width: number;
  height: number;
  devicePixelRatio: number;
  stageScale: number;
  features: Features;
  compat: boolean;
  fontsLoaded: boolean;
  firstRenderMs: number | null;
  errors: ScriptError[];
  visibility: VisibilityEvent[];
}

/** The report for what the page measured, within the server's limits. */
export function buildReport(m: Measured): ScreenReport {
  const round = (n: number, digits: number) => Math.round(n * 10 ** digits) / 10 ** digits;
  return {
    reason: m.reason,
    userAgent: plainText(m.userAgent, MAX_USER_AGENT) || 'unknown',
    viewport: { width: Math.max(1, Math.round(m.width)), height: Math.max(1, Math.round(m.height)) },
    devicePixelRatio: round(m.devicePixelRatio, 3) || 1,
    stageScale: round(m.stageScale, 4),
    features: m.features,
    compat: m.compat,
    fontsLoaded: m.fontsLoaded,
    firstRenderMs: m.firstRenderMs === null ? null : Math.min(MAX_FIRST_RENDER_MS, Math.max(0, Math.round(m.firstRenderMs))),
    errors: m.errors.slice(0, MAX_ERRORS),
    visibility: m.visibility.slice(-MAX_VISIBILITY_EVENTS),
  };
}

/**
 * Sends a report for the display this screen shows. A failure only costs
 * the report. keepalive lets a report that the page is hidden arrive also
 * when the TV suspends or closes the page right after.
 */
export async function sendReport(displayId: string, report: ScreenReport): Promise<void> {
  try {
    const response = await fetch(`/api/displays/${encodeURIComponent(displayId)}/screen-reports`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(report),
      keepalive: true,
    });
    // 401: an admin previewing in a browser, which has no screen to report.
    if (!response.ok && response.status !== 401) console.warn(`gdash: the screen report was refused (${response.status})`);
  } catch (err) {
    console.warn('gdash: could not send the screen report', err);
  }
}
