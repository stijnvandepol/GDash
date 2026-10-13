import { describe, expect, it } from 'vitest';
import { DEFAULT_SECONDS, cameraId, readShot, refreshSeconds, shotTime } from './camera';

describe('cameraId', () => {
  it('takes a whole number from 1 up', () => {
    expect(cameraId({ camera: 3 })).toBe(3);
    for (const camera of [0, -1, 1.5, '3', undefined]) expect(cameraId({ camera })).toBeUndefined();
    expect(cameraId(undefined)).toBeUndefined();
  });
});

describe('refreshSeconds', () => {
  it('uses the setting, never below 5 seconds', () => {
    expect(refreshSeconds({ seconds: 30 })).toBe(30);
    expect(refreshSeconds({ seconds: 2 })).toBe(DEFAULT_SECONDS);
    expect(refreshSeconds({})).toBe(DEFAULT_SECONDS);
  });
});

describe('readShot', () => {
  it('reads when the image arrived and whether the camera failed since', () => {
    const shot = readShot(new Headers({ 'Last-Modified': 'Tue, 13 Oct 2026 12:02:00 GMT', 'X-Image-Stale': 'true' }));
    expect(shot).toEqual({ takenAt: new Date(Date.UTC(2026, 9, 13, 12, 2)), stale: true });
  });

  it('treats a fresh image without a date as not stale', () => {
    expect(readShot(new Headers())).toEqual({ takenAt: null, stale: false });
    expect(readShot(new Headers({ 'Last-Modified': 'yesterday' })).takenAt).toBeNull();
  });
});

describe('shotTime', () => {
  it('shows hours and minutes in the screen language', () => {
    const at = new Date(2026, 9, 13, 14, 2);
    expect(shotTime(at, 'nl-NL')).toBe('14:02');
    expect(shotTime(at, 'de-DE')).toBe('14:02');
  });
});
