import { describe, expect, it } from 'vitest';
import { coordinate, roundCoordinate } from './coordinates';

describe('coordinate', () => {
  it('rounds half away from zero like the server', () => {
    expect(coordinate(51.7655)).toBe('51.766');
    expect(coordinate(-51.7655)).toBe('-51.766');
    expect(coordinate(5.5)).toBe('5.5');
    expect(coordinate(51.7654321)).toBe('51.765');
  });

  it('writes zero without a sign', () => {
    expect(coordinate(-0.0004)).toBe('0');
    expect(coordinate(0)).toBe('0');
    expect(Object.is(roundCoordinate(-0.0004), 0)).toBe(true);
  });

  it('leaves a value that is already rounded alone', () => {
    expect(roundCoordinate(51.77)).toBe(51.77);
    expect(roundCoordinate(-33.869)).toBe(-33.869);
  });
});
