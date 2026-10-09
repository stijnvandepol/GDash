import { describe, expect, it } from 'vitest';
import { stageFor } from './stage';

describe('stageFor', () => {
  it('draws a 16:9 TV at the design size', () => {
    const s = stageFor(1920, 1080, 100);
    expect([s.width, s.height, s.scale]).toEqual([1920, 1080, 1]);
  });

  it('fills a 16:10 Nest Hub Max and a 4:3 tablet without bars', () => {
    const hub = stageFor(1280, 800, 100);
    expect(hub.height).toBe(1200);
    expect(hub.width * hub.scale).toBeCloseTo(1280);
    expect(hub.height * hub.scale).toBeCloseTo(800);
    const tablet = stageFor(1024, 768, 100);
    expect(tablet.height * tablet.scale).toBeCloseTo(768);
  });

  it('makes everything larger at a larger text size', () => {
    const normal = stageFor(1024, 600, 100);
    const large = stageFor(1024, 600, 150);
    expect(large.width).toBe(1280);
    expect(large.scale / normal.scale).toBeCloseTo(1.5);
  });

  it('ignores a text size it does not offer, and very odd screens get bars', () => {
    expect(stageFor(1920, 1080, 130).textScale).toBe(1);
    expect(stageFor(1920, 1080, undefined).textScale).toBe(1);
    const tall = stageFor(600, 1000, 100);
    expect(tall.height).toBe(1440); // clamped to 4:3
  });
});
