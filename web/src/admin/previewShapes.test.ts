import { describe, expect, it } from 'vitest';
import { SHAPES, fitFrame } from './previewShapes';

const [tv, tablet, hub] = SHAPES;

describe('fitFrame', () => {
  it('shrinks to the width when the space is wide enough in height', () => {
    expect(fitFrame(tv!, 640, 2000)).toEqual({ scale: 0.5, width: 640, height: 360 });
  });

  it('shrinks to the height when the space is too low', () => {
    expect(fitFrame(tv!, 2000, 360)).toEqual({ scale: 0.5, width: 640, height: 360 });
  });

  it('keeps the shape of each screen', () => {
    for (const shape of [tv!, tablet!, hub!]) {
      const fit = fitFrame(shape, 700, 700);
      expect(fit.width / fit.height).toBeCloseTo(shape.width / shape.height, 1);
    }
  });

  it('does not enlarge a preview that has room to spare', () => {
    expect(fitFrame(hub!, 5000, 5000)).toEqual({ scale: 1, width: 1024, height: 600 });
  });

  it('stays visible before the space has been measured', () => {
    expect(fitFrame(tv!, 0, 0).scale).toBe(0.1);
  });
});
