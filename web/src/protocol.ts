// Message format between the GDash server and every display. Mirrors
// internal/display/display.go and internal/datahub/hub.go.

export const CAST_NAMESPACE = 'urn:x-cast:app.gdash.display';

export interface Entry {
  value?: unknown;
  fetchedAt: string;
  staleAfter: string;
  failing: boolean;
}

export interface Widget {
  id: string;
  type: string;
  x: number;
  y: number;
  w: number;
  h: number;
  source?: string;
  settings?: Record<string, unknown>;
}

export interface Dashboard {
  id: string;
  name: string;
  theme: string;
  /** A typeface instead of the theme's own; empty or absent for the theme's. */
  font?: string;
  columns: number;
  rows: number;
  widgets: Widget[];
}

/** A message shown over the dashboard until expiresAt, on the server's clock. */
export interface Announcement {
  title?: string;
  message: string;
  expiresAt: string;
}

export interface Snapshot {
  type: 'snapshot';
  version: number;
  serverTime: string;
  /** False outside the display's active hours: the screen stays dark. */
  active?: boolean;
  dashboard: Dashboard;
  data: Record<string, Entry>;
  /** The household's display language, "en", "nl", "de" or "fr". */
  language?: string;
  /** Dashboards shown in turn, each for pageSeconds; see pages.ts. */
  pages?: Dashboard[];
  pageSeconds?: number;
  /** Text size in percent; see stage.ts. */
  textScale?: number;
  /** Absent when there is none; see announcement.ts. */
  announcement?: Announcement;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null;
}

function isWidget(v: unknown): v is Widget {
  return (
    isRecord(v) &&
    typeof v.id === 'string' &&
    typeof v.type === 'string' &&
    [v.x, v.y, v.w, v.h].every((n) => Number.isInteger(n))
  );
}

/**
 * Checks the shape of a snapshot before the display trusts it. Messages
 * arrive over the network and, on Cast, from any sender on the LAN.
 */
export function isSnapshot(v: unknown): v is Snapshot {
  if (!isRecord(v) || v.type !== 'snapshot' || typeof v.version !== 'number') return false;
  const d = v.dashboard;
  return (
    isRecord(d) &&
    Number.isInteger(d.columns) &&
    Number.isInteger(d.rows) &&
    Array.isArray(d.widgets) &&
    d.widgets.every(isWidget) &&
    isRecord(v.data)
  );
}
