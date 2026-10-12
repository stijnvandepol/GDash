import { describe, expect, it } from 'vitest';
import {
  buildCondition,
  checkContext,
  conditionPath,
  defaultLabel,
  findList,
  pairs,
  parseCondition,
  previewOf,
  rowPath,
  rules,
  strings,
  subjects,
  valueSettings,
  type Condition,
} from './customWidget';

const doc = { data: { items: [{ name: 'Milk', price: 1.25 }, { name: 'Bread', price: 2.5 }] }, temp: 21.5 };

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

describe('named values in the widget', () => {
  const lists = [{ name: 'Prices', path: 'data.items', columns: [{ name: 'Product', path: 'name' }, { name: 'Price', path: 'price' }] }];

  it('writes the path, unit and decimals of a picked value, clearing those it lacks', () => {
    expect(valueSettings({ name: 'Inside', path: 'temp', unit: '°C', decimals: 1 })).toEqual({ path: 'temp', unit: '°C', decimals: 1 });
    expect(valueSettings({ name: 'Inside', path: 'temp' })).toEqual({ path: 'temp', unit: '', decimals: '' });
  });

  it('finds the named list by its path and columns', () => {
    expect(findList(lists, 'data.items', ['name', 'price'])).toBe(lists[0]);
    expect(findList(lists, 'data.items', ['name'])).toBeUndefined();
    expect(findList(lists, 'other', ['name', 'price'])).toBeUndefined();
  });

  it('labels a new line with the name, or a badge with initials', () => {
    expect(defaultLabel('keyValues', 'Feels like')).toBe('Feels like');
    expect(defaultLabel('badges', 'Anna')).toBe('AN');
    expect(defaultLabel('badges', 'Stijn van de Pol')).toBe('SVD');
  });

  it('offers the value, each item column and the named values to a rule', () => {
    const values = [{ name: 'Inside', path: 'temp' }, { name: 'Odd', path: 'value.x' }];
    expect(subjects('value', values, undefined)).toEqual([
      { path: 'value', label: 'the value' },
      { path: 'temp', label: 'Inside' },
      { path: '$.value.x', label: 'Odd' },
    ]);
    expect(subjects('list', [], lists[0]).map((s) => s.path)).toEqual(['row.name', 'row.price']);
    expect(subjects('badges', [], undefined)).toEqual([]);
  });

  it('writes paths as a condition reads them', () => {
    expect(conditionPath('home.anna')).toBe('home.anna');
    expect(conditionPath('row.x')).toBe('$.row.x');
    expect(conditionPath('$[0]')).toBe('$[0]');
    expect(rowPath('$')).toBe('row');
    expect(rowPath('$.a')).toBe('row.a');
    expect(rowPath('[0]')).toBe('row[0]');
    expect(rowPath('name')).toBe('row.name');
  });
});

describe('the rule builder', () => {
  const cases: [Condition, string][] = [
    [{ subject: 'value', op: 'above', operand: '25' }, 'number(value) > 25'],
    [{ subject: 'value', op: 'below', operand: '-0,5' }, 'number(value) < -0.5'],
    [{ subject: 'home.state', op: 'equals', operand: 'away "now"' }, 'home.state == "away \\"now\\""'],
    [{ subject: 'row.stock', op: 'isNot', operand: '0' }, 'number(row.stock) != 0'],
    [{ subject: 'status', op: 'contains', operand: 'Error' }, 'contains(lower(status), "error")'],
    [{ subject: 'home.anna', op: 'on', operand: '' }, 'isOn(home.anna)'],
    [{ subject: 'home.anna', op: 'off', operand: '' }, 'not isOn(home.anna)'],
  ];

  it('writes each choice as a condition and reads it back', () => {
    for (const [condition, text] of cases) {
      expect(buildCondition(condition)).toBe(text);
      const back = parseCondition(text)!;
      expect(back.subject).toBe(condition.subject);
      expect(back.op).toBe(condition.op);
      expect(buildCondition(back)).toBe(text);
    }
  });

  it('writes nothing while a choice is incomplete', () => {
    expect(buildCondition({ subject: 'value', op: 'above', operand: '' })).toBe('');
    expect(buildCondition({ subject: 'value', op: 'above', operand: 'warm' })).toBe('');
    expect(buildCondition({ subject: '', op: 'on', operand: '' })).toBe('');
    expect(buildCondition({ subject: 'a', op: 'contains', operand: ' ' })).toBe('');
  });

  it('reads the simple conditions written by hand', () => {
    expect(parseCondition('value > 25')).toEqual({ subject: 'value', op: 'above', operand: '25' });
    expect(parseCondition('contains(status, "x")')).toEqual({ subject: 'status', op: 'contains', operand: 'x' });
  });

  it('leaves other conditions to the advanced field', () => {
    for (const when of ['value > 25 and value < 30', 'value >= 3', 'minutesSince(time) > 60', 'isOn(a) and isOn(b)', 'value == true', 'a > "b"', '']) {
      expect(parseCondition(when), when).toBeNull();
    }
  });
});
