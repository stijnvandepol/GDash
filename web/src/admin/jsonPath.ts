// The path syntax of Custom widgets, as internal/custom/path.go defines it,
// so the editor can show what a path points at in a sample answer and
// write a path for a value that is clicked. The server checks and resolves
// paths itself; this copy only previews.

export type Segment = { kind: 'key'; key: string } | { kind: 'index'; index: number } | { kind: 'all' };

export interface ParsedPath {
  rooted: boolean;
  segments: Segment[];
}

const MAX_PATH_LENGTH = 200;
const MAX_SEGMENTS = 32;
const MAX_INDEX = 1_000_000;
const NAME = /^[A-Za-z_][A-Za-z0-9_]*/;

/** Parses a path, or returns null when the server would refuse it. */
export function parsePath(text: string): ParsedPath | null {
  if (text === '' || text.length > MAX_PATH_LENGTH) return null;
  const path: ParsedPath = { rooted: false, segments: [] };
  let i = 0;
  const first = NAME.exec(text);
  if (text[0] === '$') {
    path.rooted = true;
    i = 1;
  } else if (first) {
    path.segments.push({ kind: 'key', key: first[0] });
    i = first[0].length;
  } else if (text[0] !== '[') {
    return null;
  }
  while (i < text.length) {
    const step = text[i] === '.' ? readName(text, i + 1) : text[i] === '[' ? readBracket(text, i) : null;
    if (!step || path.segments.length === MAX_SEGMENTS) return null;
    path.segments.push(step.segment);
    i = step.end;
  }
  return path;
}

function readName(text: string, start: number): { segment: Segment; end: number } | null {
  const match = NAME.exec(text.slice(start));
  return match ? { segment: { kind: 'key', key: match[0] }, end: start + match[0].length } : null;
}

function readBracket(text: string, start: number): { segment: Segment; end: number } | null {
  const rest = text.slice(start);
  if (rest.startsWith('[*]')) return { segment: { kind: 'all' }, end: start + 3 };
  const index = /^\[(-?[0-9]+)\]/.exec(rest);
  if (index) {
    const n = Number(index[1]);
    return Math.abs(n) > MAX_INDEX ? null : { segment: { kind: 'index', index: n }, end: start + index[0].length };
  }
  const quote = rest[1];
  if (quote !== '"' && quote !== "'") return null;
  let key = '';
  for (let j = 2; j < rest.length; j++) {
    const c = rest[j]!;
    if (c === quote) return rest[j + 1] === ']' ? { segment: { kind: 'key', key }, end: start + j + 2 } : null;
    if (c === '\\') {
      const next = rest[++j];
      if (next === 'n') key += '\n';
      else if (next === '\\' || next === '"' || next === "'") key += next;
      else return null;
    } else {
      key += c;
    }
  }
  return null;
}

/**
 * The value at a path, as the server finds it: found is false for a missing
 * key or element, and a path with [*] always gives a list.
 */
export function resolvePath(doc: unknown, text: string): { found: boolean; value: unknown } {
  const path = parsePath(text);
  if (!path) return { found: false, value: undefined };
  let current: unknown[] = [doc];
  let many = false;
  for (const segment of path.segments) {
    const next: unknown[] = [];
    for (const value of current) step(next, value, segment);
    if (segment.kind === 'all') many = true;
    current = next;
  }
  if (many) return { found: true, value: current };
  return current.length === 1 ? { found: true, value: current[0] } : { found: false, value: undefined };
}

function step(out: unknown[], value: unknown, segment: Segment) {
  if (segment.kind === 'key') {
    if (isObject(value) && Object.prototype.hasOwnProperty.call(value, segment.key)) out.push(value[segment.key]);
  } else if (Array.isArray(value)) {
    if (segment.kind === 'all') {
      out.push(...value);
    } else {
      const i = segment.index < 0 ? segment.index + value.length : segment.index;
      if (i >= 0 && i < value.length) out.push(value[i]);
    }
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** The path to a place in an answer, from its keys and indexes: data.items[0].name. */
export function formatPath(steps: (string | number)[]): string {
  if (steps.length === 0) return '$';
  let text = '';
  for (const s of steps) {
    if (typeof s === 'number') text += `[${s}]`;
    else if (/^[A-Za-z_][A-Za-z0-9_]*$/.test(s)) text += text === '' ? s : `.${s}`;
    else text += `["${s.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\n')}"]`;
  }
  return text;
}

/**
 * The part of a clicked path below an item of the list at listPath, for a
 * field of a list widget: "name" for data.items[0].name in data.items, or
 * "$" for the item itself. Null when the path is not inside that list.
 */
export function relativeTo(listPath: string, clicked: string): string | null {
  if (listPath === '') return null;
  // $ is the top of the answer: "$" and "[0].id" are written without it.
  const fromTop = (p: string) => (p.startsWith('$.') ? p.slice(2) : p.startsWith('$') ? p.slice(1) : p);
  const list = fromTop(listPath);
  const path = fromTop(clicked);
  const match = /^\[-?[0-9]+\]/.exec(path.slice(list.length));
  if (!path.startsWith(list) || !match) return null;
  const rest = path.slice(list.length + match[0].length);
  if (rest === '') return '$';
  return rest.startsWith('.') ? rest.slice(1) : rest;
}
