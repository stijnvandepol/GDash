import { describe, expect, it } from 'vitest';
import { photoSource, photoUrls } from './photos';

const UPLOADED = '0123456789abcdef0123456789abcdef.jpg';
const IMMICH = '11111111-2222-4333-8444-555555555555';

describe('photoSource', () => {
  it('is the uploaded photos unless Immich is chosen', () => {
    expect(photoSource(undefined)).toBe('uploaded');
    expect(photoSource({ source: 'immich' })).toBe('immich');
    expect(photoSource({ source: 'elsewhere' })).toBe('uploaded');
  });
});

describe('photoUrls', () => {
  it('points each source at its own route', () => {
    expect(photoUrls([UPLOADED], 'uploaded')).toEqual([`/api/photos/${UPLOADED}`]);
    expect(photoUrls([IMMICH], 'immich')).toEqual([`/api/immich/assets/${IMMICH}`]);
  });

  it('puts nothing else from a snapshot in an address', () => {
    expect(photoUrls([IMMICH, '../settings', 7, UPLOADED], 'immich')).toEqual([`/api/immich/assets/${IMMICH}`]);
    expect(photoUrls([IMMICH, 'https://elsewhere/x.jpg'], 'uploaded')).toEqual([]);
    expect(photoUrls(undefined, 'uploaded')).toEqual([]);
  });
});
