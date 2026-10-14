import { describe, expect, it } from 'vitest';
import { compatForced, meetsQuery, replaceAll } from './compat';

describe('meetsQuery', () => {
  const tile = { width: 668, height: 330 };

  it('compares the content size as the container query would', () => {
    expect(meetsQuery('height<=420px', tile)).toBe(true);
    expect(meetsQuery('height<=260px', tile)).toBe(false);
    expect(meetsQuery('height>=330px', tile)).toBe(true);
    expect(meetsQuery('height>330px', tile)).toBe(false);
    expect(meetsQuery('width<668px', tile)).toBe(false);
    expect(meetsQuery('width=668px', tile)).toBe(true);
  });

  it('needs every condition of a compound query', () => {
    expect(meetsQuery('width>=420px&height>=170px', tile)).toBe(true);
    expect(meetsQuery('width<=419px&height>=330px', tile)).toBe(false);
  });

  it('never meets a condition it cannot read', () => {
    expect(meetsQuery('aspect-ratio>1', tile)).toBe(false);
    expect(meetsQuery('', tile)).toBe(false);
  });
});

describe('replaceAll', () => {
  it('replaces every occurrence of a string, also one with regular expression characters', () => {
    expect(replaceAll.call('<!><p><!>', '<!>', '<!---->')).toBe('<!----><p><!---->');
    expect(replaceAll.call('a.b.c', '.', '-')).toBe('a-b-c');
  });

  it('takes a global regular expression and refuses another', () => {
    expect(replaceAll.call('/* a */x/* b */', /\/\*.*?\*\//g, '')).toBe('x');
    expect(() => replaceAll.call('x', /x/, '')).toThrow(TypeError);
  });

  it('passes a replacement function each match', () => {
    expect(replaceAll.call('ab', /[ab]/g, (m: string) => m.toUpperCase())).toBe('AB');
  });
});

describe('compatForced', () => {
  it('is only on for compat=1', () => {
    expect(compatForced('?compat=1')).toBe(true);
    expect(compatForced('?textScale=150&compat=1')).toBe(true);
    expect(compatForced('?compat=0')).toBe(false);
    expect(compatForced('')).toBe(false);
  });
});
