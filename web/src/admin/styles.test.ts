import { describe, expect, it } from 'vitest';
import { THEMES, nightChoices } from './styles';

const themes = Object.keys(THEMES);

describe('nightChoices', () => {
  it('offers the dark counterpart first', () => {
    expect(nightChoices('frost', themes, '')[0]).toBe('frost-night');
    expect(nightChoices('minimal-light', themes, '')[0]).toBe('minimal-dark');
    expect(nightChoices('warm-light', themes, '')[0]).toBe('night');
  });

  it('keeps the server order after it, and never lists the counterpart twice', () => {
    const choices = nightChoices('frost', themes, '');
    expect(choices.filter((id) => id === 'frost-night')).toHaveLength(1);
    expect(choices.slice(1)).toEqual(themes.filter((id) => id !== 'frost' && id !== 'frost-night'));
  });

  it('leaves the theme itself out, "Keep this style" being that choice', () => {
    expect(nightChoices('walnut', themes, '')).toEqual(themes.filter((id) => id !== 'walnut'));
  });

  it('still lists the theme when it is the chosen one, so the select can show it', () => {
    expect(nightChoices('walnut', themes, 'walnut')).toContain('walnut');
  });

  it('does not offer a counterpart this server does not know', () => {
    expect(nightChoices('frost', ['frost', 'walnut'], '')).toEqual(['walnut']);
  });
});
