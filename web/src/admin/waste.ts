import { WASTE_COLOR, WASTE_KINDS, type Pickup, type WasteKind } from '../widgets/waste';

/** How far ahead a test of a waste calendar looks (internal/api wasteTestDays). */
export const WASTE_TEST_WEEKS = 8;

/** The name of each recognised kind in the web interface, which is in English. */
export const WASTE_KIND_NAMES: Record<Exclude<WasteKind, 'other'>, string> = {
  gft: 'GFT (organic waste)',
  pmd: 'PMD (plastic & packaging)',
  paper: 'Paper',
  rest: 'General waste',
  glass: 'Glass',
  textile: 'Textile',
  tree: 'Christmas trees',
};

/** A recognised kind in a test: its first pickup and how many there are. */
export interface FoundKind {
  kind: Exclude<WasteKind, 'other'>;
  name: string;
  /** A colour name of colors.ts, or undefined for the theme's muted text. */
  color: string | undefined;
  first: string;
  count: number;
}

/**
 * What a test found: the recognised kinds in the order of the widget, and
 * the titles of the pickups that are no known kind, which the widget shows
 * as they are.
 */
export function summarisePickups(pickups: Pickup[]): { kinds: FoundKind[]; others: string[] } {
  const kinds: FoundKind[] = [];
  const others: string[] = [];
  for (const kind of WASTE_KINDS) {
    if (kind === 'other') continue;
    const ofKind = pickups.filter((p) => p.kind === kind);
    if (ofKind.length === 0) continue;
    const first = ofKind.reduce((a, b) => (b.date < a ? b.date : a), ofKind[0]!.date);
    kinds.push({ kind, name: WASTE_KIND_NAMES[kind], color: WASTE_COLOR[kind], first, count: ofKind.length });
  }
  for (const p of pickups) {
    if (p.kind === 'other' && !others.includes(p.title)) others.push(p.title);
  }
  return { kinds, others };
}
