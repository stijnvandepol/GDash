import { describe, expect, it } from 'vitest';
import { checkContext, pairs, pathFor, previewOf, rules, strings } from './customWidget';

const doc = { data: { items: [{ name: 'Milk', price: 1.25 }, { name: 'Bread', price: 2.5 }] }, temp: 21.5 };

describe('pathFor', () => {
  it('makes a field relative to the list', () => {
    expect(pathFor({ kind: 'field', index: 0 }, 'data.items[1].name', 'list', 'data.items')).toBe('name');
    expect(pathFor({ kind: 'field', index: 0 }, 'temp', 'list', 'data.items')).toBe('temp');
  });

  it('writes a rule in a list as a path from row', () => {
    expect(pathFor({ kind: 'rule', index: 0 }, 'data.items[0].price', 'list', 'data.items')).toBe('row.price');
    expect(pathFor({ kind: 'rule', index: 0 }, 'data.items[0]', 'list', 'data.items')).toBe('row');
    expect(pathFor({ kind: 'rule', index: 0 }, 'temp', 'list', 'data.items')).toBe('temp');
  });

  it('writes the widget value in a rule as value', () => {
    expect(pathFor({ kind: 'rule', index: 0 }, 'temp', 'value', 'temp')).toBe('value');
    expect(pathFor({ kind: 'path' }, 'temp', 'value', '')).toBe('temp');
  });
});

describe('previewOf', () => {
  it('shows the value at a path, or that it is missing', () => {
    expect(previewOf(doc, 'data.items[*].name')).toBe('["Milk","Bread"]');
    expect(previewOf(doc, 'nope')).toBe('Not found in this data');
    expect(previewOf(undefined, 'temp')).toBe('');
    expect(previewOf({ s: 'x'.repeat(200) }, 's')).toHaveLength(80);
  });
});

describe('checkContext', () => {
  it('passes the value or the first row', () => {
    expect(checkContext(doc, 'value', 'temp')).toEqual({ value: 21.5 });
    expect(checkContext(doc, 'list', 'data.items')).toEqual({ row: { name: 'Milk', price: 1.25 } });
    expect(checkContext(doc, 'keyValues', '')).toEqual({});
  });
});

describe('reading settings', () => {
  it('keeps items of the right shape', () => {
    expect(strings(['a', 1, 'b'])).toEqual(['a', 'b']);
    expect(strings(undefined)).toEqual([]);
    expect(pairs([{ label: 'A', path: 'a' }, { label: 3 }])).toEqual([
      { label: 'A', path: 'a' },
      { label: '', path: '' },
    ]);
    expect(rules([{ when: 'value > 1', tone: 'ok' }, { tone: 'bad' }, null])).toEqual([{ when: 'value > 1', tone: 'ok' }]);
  });
});
