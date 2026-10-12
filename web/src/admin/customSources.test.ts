import { describe, expect, it } from 'vitest';
import type { CustomSource } from './api';
import { describeInterval, describeSize, hostOf, parseSample, sampleText, secretPlaceholder, toInput } from './customSources';

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
