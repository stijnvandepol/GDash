import { describe, expect, it } from 'vitest';
import { groupSecret, isPendingLogin, qrImageSource, recoveryCodesText } from './twoStep';

describe('isPendingLogin', () => {
  it('tells a pending login from a user', () => {
    expect(isPendingLogin({ pending: 'abc' })).toBe(true);
    expect(isPendingLogin({ id: 1, username: 'stijn' })).toBe(false);
  });
});

describe('qrImageSource', () => {
  it('makes an SVG image address that keeps every character', () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg"><path d="M4 4h7v1h-7z"/></svg>';
    const source = qrImageSource(svg);
    expect(source.startsWith('data:image/svg+xml;charset=utf-8,')).toBe(true);
    expect(source).not.toContain('<');
    expect(source).not.toContain('"');
    expect(decodeURIComponent(source.slice(source.indexOf(',') + 1))).toBe(svg);
  });
});

describe('groupSecret', () => {
  it('splits the secret into groups of four', () => {
    expect(groupSecret('GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ')).toBe('GEZD GNBV GY3T QOJQ GEZD GNBV GY3T QOJQ');
    expect(groupSecret('ABCDEF')).toBe('ABCD EF');
    expect(groupSecret('')).toBe('');
  });
});

describe('recoveryCodesText', () => {
  it('names the account and the day, and lists every code on its own line', () => {
    const text = recoveryCodesText('stijn', ['aaaa-bbbb-cccc-dddd', 'eeee-ffff-gggg-hhhh'], new Date('2026-10-12T09:00:00Z'));
    const lines = text.split('\n');
    expect(lines[0]).toBe('GDash recovery codes for stijn');
    expect(lines[1]).toBe('Made on 2026-10-12.');
    expect(lines).toContain('aaaa-bbbb-cccc-dddd');
    expect(lines).toContain('eeee-ffff-gggg-hhhh');
    expect(text.endsWith('\n')).toBe(true);
  });
});
