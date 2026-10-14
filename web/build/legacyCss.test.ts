import { describe, expect, it } from 'vitest';
import { legacyStylesheet, queryToken, replaceContainerUnits } from './legacyCss';

describe('replaceContainerUnits', () => {
  it('writes every container unit over the tile properties', () => {
    expect(replaceContainerUnits('min(172px,34cqw,64cqh)')).toBe('min(172px,calc(34 * var(--cqw)),calc(64 * var(--cqh)))');
    expect(replaceContainerUnits('3cqmin 1.5cqmax .5cqi 2cqb')).toBe(
      'calc(3 * var(--cqmin)) calc(1.5 * var(--cqmax)) calc(.5 * var(--cqw)) calc(2 * var(--cqh))',
    );
    expect(replaceContainerUnits('calc(-1*clamp(2px,2cqh,10px))')).toBe('calc(-1*clamp(2px,calc(2 * var(--cqh)),10px))');
  });

  it('leaves other values alone', () => {
    expect(replaceContainerUnits('12px solid var(--cqw-like)')).toBe('12px solid var(--cqw-like)');
    expect(replaceContainerUnits('url(a2cqw.png)')).toBe('url(a2cqw.png)');
  });
});

describe('queryToken', () => {
  it('reads the range syntax the minifier writes and the min-/max- syntax', () => {
    expect(queryToken('(height<=420px)')).toBe('height<=420px');
    expect(queryToken('(width>=420px) and (height>=170px)')).toBe('width>=420px&height>=170px');
    expect(queryToken('(min-height: 300px)')).toBe('height>=300px');
    expect(queryToken('(max-width: 440px)')).toBe('width<=440px');
  });

  it('refuses a query the fallback cannot evaluate', () => {
    expect(() => queryToken('(orientation: portrait)')).toThrow(/not supported/);
    expect(() => queryToken('(width > 30em)')).toThrow(/not supported/);
    expect(() => queryToken('(width>=400px) or (height>=300px)')).toThrow(/not supported/);
  });
});

describe('legacyStylesheet', () => {
  it('unfolds container queries behind the tile attribute', () => {
    const css = '.a.s1{font-size:min(34px,7cqw)}@container (height>=300px){.t.s1{display:block}.u.s1,.v.s1{color:red}}.b.s1{color:blue}';
    expect(legacyStylesheet(css)).toBe(
      '.a.s1{font-size:min(34px,calc(7 * var(--cqw)))}[data-cq~="height>=300px"] .t.s1{display:block}[data-cq~="height>=300px"] .u.s1,[data-cq~="height>=300px"] .v.s1{color:red}.b.s1{color:blue}',
    );
  });

  it('writes the scoping :where() of Svelte as a plain class', () => {
    expect(legacyStylesheet('.next.s1 div:where(.s1){gap:4px}')).toBe('.next.s1 div.s1{gap:4px}');
  });

  it('rewrites container units in custom properties too', () => {
    expect(legacyStylesheet('.badge.s2{--size:clamp(56px,min(26cqw,48cqh),140px)}')).toBe(
      '.badge.s2{--size:clamp(56px,min(calc(26 * var(--cqw)),calc(48 * var(--cqh))),140px)}',
    );
  });

  it('drops named containers and font faces, which an older browser already has', () => {
    const css = '@font-face{font-family:X;src:url(x.woff2)}@container page (width>=700px){.d{display:grid}}.e{width:5cqw}';
    expect(legacyStylesheet(css)).toBe('.e{width:calc(5 * var(--cqw))}');
  });

  it('is only a comment for a stylesheet that needs no fallback', () => {
    expect(legacyStylesheet('@font-face{font-family:X;src:url(x.woff2)}.a{color:red}')).toMatch(/^\/\*[^*]*\*\/\n$/);
  });

  it('fails the build for a query or rule it cannot unfold', () => {
    expect(() => legacyStylesheet('@container (aspect-ratio > 1){.a{color:red}}')).toThrow(/not supported/);
    expect(() => legacyStylesheet('@container (width>=400px){@media print{.a{color:red}}}')).toThrow(/@media/);
  });
});
