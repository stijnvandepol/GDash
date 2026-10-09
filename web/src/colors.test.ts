import { describe, expect, it } from 'vitest';
import { calendarColor } from './colors';

describe('calendarColor', () => {
  it('gives the shade of a known colour', () => {
    expect(calendarColor('blue')).toBe('#4f8cff');
  });

  it('gives nothing for unknown names, also ones every object has', () => {
    for (const name of ['magenta', 'toString', '__proto__', '', 42, undefined]) {
      expect(calendarColor(name)).toBeUndefined();
    }
  });
});
