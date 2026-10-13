import { api, ApiError, type CustomSource, type NamedList, type NamedValue } from './api';
import { errorMessage } from './errors';
import { parsePath, resolvePath } from './jsonPath';

// Helpers for the Custom widget's settings in the editor. The server checks
// the same limits (internal/custom/widget.go).

export const MODES = [
  { mode: 'value', label: 'One value' },
  { mode: 'list', label: 'A list' },
  { mode: 'keyValues', label: 'Labelled values' },
  { mode: 'badges', label: 'Badges' },
] as const;

export const TONES = [
  { tone: '', label: 'No colour' },
  { tone: 'ok', label: 'Good' },
  { tone: 'warn', label: 'Warning' },
  { tone: 'bad', label: 'Bad' },
] as const;

export const MAX_PAIRS = 8;
export const MAX_RULES = 6;
// A badge's label and a labelled value's label; internal/custom allows 40.
const MAX_LABEL = 40;
const MAX_BADGE_LETTERS = 3;

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

/** What a path shows in the sample, for the hint next to a value. */
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

/**
 * What a rule's condition gives for a widget now, in words: checked against
 * the source's sample, or a fresh answer without one.
 */
export async function checkRule(source: CustomSource | undefined, when: string, mode: string, path: string): Promise<string> {
  if (!source) return 'Choose an API first.';
  try {
    const doc = source.sample ?? (await api.testCustomSource(source.id)).data;
    const answer = await api.checkRule({ when, data: doc, ...checkContext(doc, mode, path) });
    return answer.error ? `Does not apply now: ${answer.error}` : `Gives ${JSON.stringify(answer.result)} now`;
  } catch (err) {
    const reason = err instanceof ApiError ? err.fieldMessage('when') : undefined;
    return reason ? `Cannot be read: ${reason}` : errorMessage(err, () => {});
  }
}

/** The named list a list widget shows: the one with its list path and columns. */
export function findList(lists: NamedList[], path: string, fields: string[]): NamedList | undefined {
  return lists.find((l) => l.path === path && l.columns.length === fields.length && l.columns.every((c, i) => c.path === fields[i]));
}

/** The settings a picked named value writes in value mode: its path, unit and decimals. */
export function valueSettings(value: NamedValue): { path: string; unit: string; decimals: number | '' } {
  return { path: value.path, unit: value.unit ?? '', decimals: value.decimals ?? '' };
}

/**
 * The label a line gets when a named value is picked for it and it has
 * none yet: the name for a labelled value, initials for a badge.
 */
export function defaultLabel(mode: string, name: string): string {
  if (mode !== 'badges') return name.slice(0, MAX_LABEL);
  const words = name.split(/\s+/).filter(Boolean);
  const letters = words.length > 1 ? words.map((w) => w[0]).join('') : name.slice(0, 2);
  return letters.slice(0, MAX_BADGE_LETTERS).toUpperCase();
}

// The rule builder. A rule's condition is stored in the expression language
// of internal/custom; the editor writes and reads a few fixed forms of it,
// such as number(value) > 25, and leaves anything else to the advanced field.

export const OPERATORS = [
  { op: 'above', label: 'is above', operand: 'number' },
  { op: 'below', label: 'is below', operand: 'number' },
  { op: 'equals', label: 'equals', operand: 'text' },
  { op: 'isNot', label: 'is not', operand: 'text' },
  { op: 'contains', label: 'contains', operand: 'text' },
  { op: 'on', label: 'is on', operand: 'none' },
  { op: 'off', label: 'is off', operand: 'none' },
] as const;

export type Operator = (typeof OPERATORS)[number]['op'];

/** A condition as the builder shows it; subject is a path in a condition, such as value or row.price. */
export interface Condition {
  subject: string;
  op: Operator;
  operand: string;
}

export interface Subject {
  /** The path as a condition writes it. */
  path: string;
  label: string;
}

// Words a condition reads as something else than a key of the answer.
const RESERVED = new Set(['value', 'row', 'and', 'or', 'not', 'true', 'false', 'null']);
const NUMBER = /^-?\d+(?:\.\d+)?$/;
const STRING = /^"((?:[^"\\]|\\.)*)"$/;

/** A path from the top of the answer as a condition writes it: $.value for a key called value. */
export function conditionPath(path: string): string {
  const parsed = parsePath(path);
  const first = parsed?.segments[0];
  if (parsed && !parsed.rooted && first?.kind === 'key' && RESERVED.has(first.key)) return `$.${path}`;
  return path;
}

/** A column of a list, read from the current item: row.price, or row for the item itself. */
export function rowPath(column: string): string {
  if (column === '$') return 'row';
  if (column.startsWith('$')) return `row${column.slice(1)}`;
  return column.startsWith('[') ? `row${column}` : `row.${column}`;
}

/** What a rule can test in this mode: the value itself, each item's columns, and the named values. */
export function subjects(mode: string, values: NamedValue[], list: NamedList | undefined): Subject[] {
  const out: Subject[] = [];
  if (mode === 'value') out.push({ path: 'value', label: 'the value' });
  if (mode === 'list' && list) out.push(...list.columns.map((c) => ({ path: rowPath(c.path), label: `each item's ${c.name}` })));
  for (const v of values) out.push({ path: conditionPath(v.path), label: v.name });
  return out.filter((s, i) => out.findIndex((o) => o.path === s.path) === i);
}

function quote(text: string): string {
  return `"${text.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

function unquote(literal: string): string {
  return literal.slice(1, -1).replace(/\\(.)/g, (_, c: string) => (c === 'n' ? '\n' : c));
}

/** A number as typed, also with a decimal comma, or null. */
function numberOf(text: string): string | null {
  const t = text.trim().replace(',', '.');
  return NUMBER.test(t) ? String(Number(t)) : null;
}

/** The condition text for the builder's choice, or '' while it is incomplete. */
export function buildCondition(c: Condition): string {
  const s = c.subject;
  // A condition is one line: a pasted tab or line break is dropped.
  const operand = c.operand.replace(/[\u0000-\u001f\u007f]/g, '');
  if (!s) return '';
  const n = numberOf(operand);
  switch (c.op) {
    case 'on':
      return `isOn(${s})`;
    case 'off':
      return `not isOn(${s})`;
    case 'above':
    case 'below':
      return n === null ? '' : `number(${s}) ${c.op === 'above' ? '>' : '<'} ${n}`;
    case 'equals':
    case 'isNot': {
      if (operand.trim() === '') return '';
      const op = c.op === 'equals' ? '==' : '!=';
      return n !== null ? `number(${s}) ${op} ${n}` : `${s} ${op} ${quote(operand)}`;
    }
    case 'contains':
      return operand.trim() === '' ? '' : `contains(lower(${s}), ${quote(operand.toLowerCase())})`;
  }
}

const COMPARISONS: Record<string, Operator> = { '>': 'above', '<': 'below', '==': 'equals', '!=': 'isNot' };

/** The builder's view of a condition, or null for one it cannot show, such as value > 1 and value < 5. */
export function parseCondition(when: string): Condition | null {
  const text = when.trim();
  const valid = (c: Condition) => (parsePath(c.subject) ? c : null);
  let m = /^not isOn\((.+)\)$/.exec(text);
  if (m) return valid({ subject: m[1]!, op: 'off', operand: '' });
  m = /^isOn\((.+)\)$/.exec(text);
  if (m) return valid({ subject: m[1]!, op: 'on', operand: '' });
  m = /^contains\((?:lower\((.+)\)|(.+)), ("(?:[^"\\]|\\.)*")\)$/.exec(text);
  if (m) return valid({ subject: (m[1] ?? m[2])!, op: 'contains', operand: unquote(m[3]!) });
  m = /^(?:number\((.+)\)|(.+)) (>|<|==|!=) (.+)$/.exec(text);
  if (!m) return null;
  const op = COMPARISONS[m[3]!]!;
  const literal = m[4]!;
  let operand: string;
  if (NUMBER.test(literal)) operand = literal;
  else if (STRING.test(literal) && (op === 'equals' || op === 'isNot')) operand = unquote(literal);
  else return null;
  return valid({ subject: (m[1] ?? m[2])!, op, operand });
}
