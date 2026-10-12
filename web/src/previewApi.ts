// How the preview page and the editor around it talk: the editor tells the
// page which layout and text size to show, and the page asks the server for
// the snapshot (internal/api/preview.go). Shared by both, so the message and
// the request are defined once.
import { isSnapshot, type Snapshot, type Widget } from './protocol';

/** A layout that is not saved yet; the body of the server's POST preview. */
export interface PreviewDraft {
  name: string;
  theme: string;
  font: string;
  /** The theme after sunset; empty keeps the theme. */
  nightTheme: string;
  widgets: Widget[];
}

export const PREVIEW_MESSAGE = 'gdash.preview';

/** Sent by the editor to the preview page in its frame. */
export interface PreviewMessage {
  type: typeof PREVIEW_MESSAGE;
  /** Text size in percent. */
  textScale: number;
  /** The unsaved layout to show instead of the stored one. */
  draft: PreviewDraft | null;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null;
}

function isDraft(v: unknown): v is PreviewDraft {
  return isRecord(v) && typeof v.name === 'string' && typeof v.theme === 'string' && typeof v.font === 'string' && typeof v.nightTheme === 'string' && Array.isArray(v.widgets);
}

/** Checks a message before the page acts on it; the server validates the layout itself. */
export function isPreviewMessage(v: unknown): v is PreviewMessage {
  return isRecord(v) && v.type === PREVIEW_MESSAGE && typeof v.textScale === 'number' && (v.draft === null || isDraft(v.draft));
}

/** The request for the snapshot: the stored dashboard, or the draft when there is one. */
export function previewRequest(dashboardId: string, textScale: number, draft: PreviewDraft | null): { url: string; init: RequestInit } {
  const url = `/api/dashboards/${encodeURIComponent(dashboardId)}/preview?textScale=${encodeURIComponent(String(textScale))}`;
  if (!draft) return { url, init: { cache: 'no-store' } };
  return {
    url,
    init: { method: 'POST', cache: 'no-store', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(draft) },
  };
}

export type PreviewOutcome =
  | { kind: 'snapshot'; snapshot: Snapshot }
  /** The admin session is missing or has ended. */
  | { kind: 'login' }
  | { kind: 'missing' }
  /** The layout or text size is refused; reason says why, in the server's words. */
  | { kind: 'invalid'; reason: string }
  | { kind: 'error' };

/** What the server's answer means for the page. */
export function previewOutcome(status: number, body: unknown): PreviewOutcome {
  if (status === 200) return isSnapshot(body) ? { kind: 'snapshot', snapshot: body } : { kind: 'error' };
  if (status === 401) return { kind: 'login' };
  if (status === 404) return { kind: 'missing' };
  if (status === 422) return { kind: 'invalid', reason: invalidReason(body) };
  return { kind: 'error' };
}

function invalidReason(body: unknown): string {
  const error = isRecord(body) && isRecord(body.error) ? body.error : {};
  const field = Array.isArray(error.fields) ? error.fields[0] : undefined;
  if (isRecord(field) && typeof field.field === 'string' && typeof field.message === 'string') return `${field.field}: ${field.message}`;
  return typeof error.message === 'string' ? error.message : '';
}
