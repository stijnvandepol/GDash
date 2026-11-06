import type { ActiveBlock, Dashboard, Display } from './api';

// Words for the display list, kept out of the components so they can be
// tested.

/** The dashboard that exists on every server (internal/display.DefaultID). */
export const DEFAULT_DASHBOARD_ID = 'default';

/** When the display is on and what else it shows: "Always on · another dashboard at 2 set times". */
export function describeHours(d: Display): string {
  const parts = [d.active.length > 0 ? `On ${describeBlocks(d.active)}` : 'Always on'];
  const slots = d.schedule.length;
  if (slots > 0) parts.push(`another dashboard at ${plural(slots, 'set time')}`);
  const turns = d.rotation.length;
  if (turns > 0) parts.push(`takes turns with ${plural(turns, 'other dashboard')}`);
  return parts.join(' · ');
}

const DAY_ORDER = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'];
const DAY_NAMES = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

/** Days in words: "Weekdays", "Weekend", "Mon–Wed, Fri", "Sat–Sun"; none or all seven is "every day". */
export function describeDays(days: string[]): string {
  const on = DAY_ORDER.map((d) => days.includes(d));
  if (on.every((x) => !x) || on.every((x) => x)) return 'every day';
  if (on.join() === 'true,true,true,true,true,false,false') return 'Weekdays';
  if (on.join() === 'false,false,false,false,false,true,true') return 'Weekend';
  const runs: string[] = [];
  for (let i = 0; i < on.length; i++) {
    if (!on[i]) continue;
    let end = i;
    while (on[end + 1]) end++;
    runs.push(end === i ? DAY_NAMES[i]! : `${DAY_NAMES[i]}–${DAY_NAMES[end]}`);
    i = end;
  }
  return runs.join(', ');
}

/**
 * When the screen is on, in words: "07:00–22:00" for one daily block,
 * else each block with its days: "Mon–Wed 06:00–10:00, Fri 07:00–12:00".
 */
export function describeBlocks(blocks: ActiveBlock[]): string {
  const alone = blocks.length === 1 && describeDays(blocks[0]!.days) === 'every day';
  return blocks
    .map((b) => (alone ? '' : `${describeDays(b.days)} `) + `${b.from}–${b.until}`)
    .join(', ');
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
