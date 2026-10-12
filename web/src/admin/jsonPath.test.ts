import { describe, expect, it } from 'vitest';
import { formatPath, parsePath, relativeTo, resolvePath } from './jsonPath';

// The same answer as internal/custom/path_test.go, so both sides agree.
const answer = {
  data: {
    items: [
      { name: 'Milk', price: 1.25, tags: ['dairy'] },
      { name: 'Bread', price: 2.5, tags: [] },
      { name: 'Eggs', price: null },
    ],
  },
  'weird key': { 'a.b': 7 },
  '2m': 21,
  ok: true,
};

describe('resolvePath', () => {
  it.each([
    ['data.items[0].name', 'Milk', true],
    ['data.items[-1].name', 'Eggs', true],
    ['data.items[1].price', 2.5, true],
    ['data.items[2].price', null, true],
    ['data.items[3].name', undefined, false],
    ['data.items[-4]', undefined, false],
    ['data.items[*].name', ['Milk', 'Bread', 'Eggs'], true],
    ['data.items[*].tags[*]', ['dairy'], true],
    ['data.items[*].missing', [], true],
    ['["weird key"]["a.b"]', 7, true],
    ["$['2m']", 21, true],
    ['$.ok', true, true],
    ['ok', true, true],
    ['nothing', undefined, false],
    ['data.items.name', undefined, false],
    ['ok[0]', undefined, false],
  ])('%s', (path, value, found) => {
    expect(resolvePath(answer, path)).toEqual({ found, value });
  });

  it('starts from a list at the top', () => {
    expect(resolvePath([{ id: 1 }, { id: 2 }], '[1].id').value).toBe(2);
    expect(resolvePath([{ id: 1 }], '$[0].id').value).toBe(1);
  });

  it('does not find keys of the object prototype', () => {
    expect(resolvePath({}, 'constructor').found).toBe(false);
  });
});

describe('parsePath', () => {
  it.each([
    '', 'data.', '.data', 'data..x', 'data[', 'data[x]', 'data[1', 'data[*', '["x]', '["x"', '2m', 'data-x',
    'data items', '$x', 'data[1.5]', 'data[99999999999]', '["\\q"]', 'a'.repeat(201), 'a' + '[0]'.repeat(32),
  ])('refuses %s as the server does', (bad) => {
    expect(parsePath(bad)).toBeNull();
  });

  it('accepts 32 steps', () => {
    expect(parsePath('a' + '[0]'.repeat(31))?.segments).toHaveLength(32);
  });
});

describe('formatPath', () => {
  it('writes names with dots and other keys in quotes', () => {
    expect(formatPath(['data', 'items', 0, 'name'])).toBe('data.items[0].name');
    expect(formatPath(['weird key', 'a.b'])).toBe('["weird key"]["a.b"]');
    expect(formatPath([0, 'id'])).toBe('[0].id');
    expect(formatPath(['say "hi"\\'])).toBe('["say \\"hi\\"\\\\"]');
    expect(formatPath([])).toBe('$');
  });

  it('writes paths that resolve back to the value', () => {
    for (const steps of [['data', 'items', 1, 'price'], ['weird key', 'a.b'], ['2m']]) {
      expect(resolvePath(answer, formatPath(steps)).found).toBe(true);
    }
  });
});

describe('relativeTo', () => {
  it('gives the part below an item of the list', () => {
    expect(relativeTo('data.items', 'data.items[0].name')).toBe('name');
    expect(relativeTo('data.items', 'data.items[2]["odd key"]')).toBe('["odd key"]');
    expect(relativeTo('data.items', 'data.items[1]')).toBe('$');
  });

  it('refuses a path outside the list', () => {
    expect(relativeTo('data.items', 'data.other[0].name')).toBeNull();
    expect(relativeTo('data.items', 'data.itemsX[0]')).toBeNull();
    expect(relativeTo('', 'a[0]')).toBeNull();
  });
});
