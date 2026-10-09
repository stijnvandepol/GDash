import { describe, expect, it } from 'vitest';
import { iconFor } from './wmo';

describe('iconFor', () => {
  it('groups the WMO codes', () => {
    expect(iconFor(0)).toBe('sun');
    expect(iconFor(0, false)).toBe('moon');
    expect(iconFor(2)).toBe('partly');
    expect(iconFor(3)).toBe('cloud');
    expect(iconFor(48)).toBe('fog');
    expect(iconFor(53)).toBe('drizzle');
    expect(iconFor(63)).toBe('rain');
    expect(iconFor(81)).toBe('rain');
    expect(iconFor(75)).toBe('snow');
    expect(iconFor(86)).toBe('snow');
    expect(iconFor(96)).toBe('thunder');
  });

  it('draws a cloud for a code it does not know', () => {
    expect(iconFor(1234)).toBe('cloud');
  });
});
