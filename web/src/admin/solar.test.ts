import { describe, expect, it } from 'vitest';
import { copySolar, describeArray, direction, newSolar } from './solar';

describe('direction', () => {
  it('names the azimuth convention of Open-Meteo and Forecast.Solar', () => {
    expect(direction(0)).toBe('south');
    expect(direction(-90)).toBe('east');
    expect(direction(90)).toBe('west');
    expect(direction(180)).toBe('north');
    expect(direction(-180)).toBe('north');
    expect(direction(-45)).toBe('south-east');
    expect(direction(130)).toBe('north-west');
    expect(direction(-20)).toBe('south');
  });
});

describe('describeArray', () => {
  it('says size, direction and tilt', () => {
    expect(describeArray({ tilt: 35, azimuth: -45, kwp: 4.2 })).toBe('4.2 kWp facing south-east at 35°');
  });
});

describe('copySolar', () => {
  it('copies deep enough that the form never edits what the page shows', () => {
    const panels = { ...newSolar(), location: { name: 'Oss', latitude: 51.77, longitude: 5.52 } };
    const copy = copySolar(panels);
    copy.arrays[0]!.tilt = 10;
    copy.location!.name = 'Cabin';
    expect(panels.arrays[0]!.tilt).toBe(35);
    expect(panels.location.name).toBe('Oss');
  });
});
