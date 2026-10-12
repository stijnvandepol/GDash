import { describe, expect, it } from 'vitest';
import type { CustomSource } from './api';
import {
  describeInterval,
  describeNamed,
  describeSize,
  hostOf,
  parseSample,
  sampleText,
  secretPlaceholder,
  suggestName,
  toInput,
} from './customSources';

const source: CustomSource = {
  id: 3,
  name: 'Prices',
  url: 'https://api.example.com/v1/prices?currency=EUR',
  method: 'GET',
  body: '',
  auth: { kind: 'header', name: 'X-API-Key', secretSet: true },
  headers: [{ name: 'X-Tenant', value: '', secret: true }],
  intervalSeconds: 900,
  sample: { price: 1 },
  values: [],
  lists: [],
  unreadable: false,
};

describe('toInput', () => {
  it('leaves every secret empty, so saving keeps the stored ones', () => {
    const input = toInput(source);
    expect(input.auth).toEqual({ kind: 'header', name: 'X-API-Key', secret: '' });
    expect(input.headers).toEqual([{ name: 'X-Tenant', value: '', secret: true }]);
    expect(input.headers[0]).not.toBe(source.headers[0]);
  });
});

describe('secretPlaceholder', () => {
  it('only offers to keep a secret of the same kind', () => {
    expect(secretPlaceholder(source, 'header')).toContain('keep');
    expect(secretPlaceholder(source, 'query')).toBe('');
    expect(secretPlaceholder(undefined, 'header')).toBe('');
  });
});

describe('parseSample', () => {
  it('reads JSON and treats empty as no sample', () => {
    expect(parseSample(' {"a": [1]} ')).toEqual({ sample: { a: [1] } });
    expect(parseSample('  ')).toEqual({ sample: null });
    expect(parseSample('{"a":')).toHaveProperty('error');
    expect(parseSample(`"${'x'.repeat(50 * 1024)}"`)).toHaveProperty('error');
  });

  it('round-trips through the text in the form', () => {
    expect(parseSample(sampleText(source.sample))).toEqual({ sample: source.sample });
    expect(sampleText(null)).toBe('');
  });
});

describe('describeInterval and hostOf', () => {
  it('name the refresh and the host', () => {
    expect(describeInterval(900)).toBe('Every 15 minutes');
    expect(describeInterval(45)).toBe('Every 45 seconds');
    expect(hostOf(source.url)).toBe('api.example.com');
    expect(hostOf('not a url')).toBe('not a url');
    expect(describeSize(312)).toBe('312 bytes');
    expect(describeSize(14 * 1024 + 100)).toBe('14 KB');
  });
});

describe('suggestName', () => {
  it('names a value after its last key, in words', () => {
    expect(suggestName('current.temperature_2m', [])).toBe('Temperature 2m');
    expect(suggestName('main.feelsLike', [])).toBe('Feels like');
    expect(suggestName('weather[0]', [])).toBe('Weather 1');
    expect(suggestName('weather[0].description', [])).toBe('Description');
    expect(suggestName('$', [])).toBe('Item');
    expect(suggestName('["wind-speed"]', [])).toBe('Wind speed');
  });

  it('adds a number to a name that is taken, ignoring case', () => {
    expect(suggestName('a.temp', ['Temp'])).toBe('Temp 2');
    expect(suggestName('a.temp', ['temp', 'TEMP 2'])).toBe('Temp 3');
    expect(suggestName(`a.${'x'.repeat(60)}`, [`X${'x'.repeat(39)}`])).toHaveLength(40);
  });
});

describe('describeNamed', () => {
  it('counts the named values and lists', () => {
    expect(describeNamed({ values: [], lists: [] })).toBe('No values named yet');
    expect(describeNamed({ values: [{ name: 'A', path: 'a' }], lists: [] })).toBe('1 value');
    expect(describeNamed({ values: [{ name: 'A', path: 'a' }, { name: 'B', path: 'b' }], lists: [{ name: 'L', path: 'l', columns: [] }] })).toBe(
      '2 values · 1 list',
    );
  });
});
