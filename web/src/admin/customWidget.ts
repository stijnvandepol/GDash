import { relativeTo, resolvePath } from './jsonPath';

// Helpers for the Custom widget's settings in the editor. The server checks
// the same limits (internal/custom/widget.go).

export const MODES = [
  { mode: 'value', label: 'One value' },
  { mode: 'list', label: 'A list' },
  { mode: 'keyValues', label: 'Labelled values' },
] as const;

export const TONES = [
  { tone: '', label: 'No colour' },
  { tone: 'ok', label: 'Good' },
  { tone: 'warn', label: 'Warning' },
  { tone: 'bad', label: 'Bad' },
] as const;

export const MAX_FIELDS = 3;
export const MAX_PAIRS = 8;
export const MAX_RULES = 6;

export interface Pair {
  label: string;
  path: string;
}

export interface Rule {
  when: string;
  tone?: string;
  message?: string;
  hide?: boolean;
  announce?: boolean;
}

/** Where a clicked value's path goes. */
export type Target = { kind: 'path' } | { kind: 'field'; index: number } | { kind: 'pair'; index: number } | { kind: 'rule'; index: number };

/** A list from the settings, keeping only items of the right shape. */
export function strings(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((v): v is string => typeof v === 'string') : [];
}

export function pairs(value: unknown): Pair[] {
  if (!Array.isArray(value)) return [];
  return value.map((p: Record<string, unknown>) => ({
    label: typeof p?.label === 'string' ? p.label : '',
    path: typeof p?.path === 'string' ? p.path : '',
  }));
}

export function rules(value: unknown): Rule[] {
  return Array.isArray(value) ? value.filter((r): r is Rule => typeof r === 'object' && r !== null && typeof r.when === 'string') : [];
}

/**
 * The text a clicked path becomes for a target: relative to the list for
 * a field, and for a rule in list mode a path from row when it points into
 * the list, as rules read each row that way.
 */
export function pathFor(target: Target, clicked: string, mode: string, listPath: string): string {
  const inList = mode === 'list' ? relativeTo(listPath, clicked) : null;
  if (target.kind === 'field') return inList ?? clicked;
  if (target.kind === 'rule' && inList !== null) return inList === '$' ? 'row' : inList.startsWith('[') ? `row${inList}` : `row.${inList}`;
  if (target.kind === 'rule' && mode === 'value' && clicked === listPath) return 'value';
  return clicked;
}

/** What a path shows in the sample, for the hint under its field. */
export function previewOf(doc: unknown, path: string): string {
  if (doc === undefined || path === '') return '';
  const { found, value } = resolvePath(doc, path);
  if (!found) return 'Not found in this data';
  const text = JSON.stringify(value) ?? 'null';
  return text.length > 80 ? `${text.slice(0, 79)}…` : text;
}

/** The data a rule check is evaluated with: the value, or the first row of the list. */
export function checkContext(doc: unknown, mode: string, path: string): { value?: unknown; row?: unknown } {
  const { value } = resolvePath(doc, path);
  if (mode === 'value') return { value };
  if (mode === 'list' && Array.isArray(value)) return { row: value[0] };
  return {};
}
