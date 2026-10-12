import type { Dashboard, Display } from './api';

// Words for the display list, kept out of the components so they can be
// tested.

/** The dashboard that exists on every server (internal/display.DefaultID). */
export const DEFAULT_DASHBOARD_ID = 'default';

/** When the display is on and what else it shows: "Always on · another dashboard at 2 set times". */
export function describeHours(d: Display): string {
  const parts = [d.activeFrom ? `On ${d.activeFrom}–${d.activeUntil}` : 'Always on'];
  const slots = d.schedule.length;
  if (slots > 0) parts.push(`another dashboard at ${plural(slots, 'set time')}`);
  const turns = d.rotation.length;
  if (turns > 0) parts.push(`takes turns with ${plural(turns, 'other dashboard')}`);
  return parts.join(' · ');
}

/**
 * The screens a display is open on besides a Cast TV. For a Cast display
 * these are extras next to the TV, which GDash pairs by itself; without
 * Cast they are the only screens. Empty when there are none.
 */
export function describeScreens(d: Display): string {
  const n = d.pairedScreens ?? 0;
  if (n === 0) return '';
  return d.castDeviceId ? `Also open on ${plural(n, 'other screen')}` : `Shown on ${plural(n, 'screen')}`;
}

/**
 * The dashboard a new display starts with: the newest one someone made,
 * since that is usually the one made for it, else the default dashboard.
 */
export function suggestedDashboard(dashboards: Dashboard[]): string {
  let newest: Dashboard | undefined;
  let newestAt = -Infinity;
  for (const b of dashboards) {
    // A time Date cannot read counts as oldest, so it never wins by accident.
    const at = Date.parse(b.createdAt ?? '') || 0;
    if (b.id === DEFAULT_DASHBOARD_ID || at <= newestAt) continue;
    newest = b;
    newestAt = at;
  }
  return newest?.id ?? DEFAULT_DASHBOARD_ID;
}

function plural(n: number, word: string): string {
  return n === 1 ? `1 ${word}` : `${n} ${word}s`;
}
