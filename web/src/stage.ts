// The stage is the canvas a dashboard is drawn on. At 100 % it is 1920 px
// wide, the size layouts are made for; the browser scales it to the screen.

const DESIGN_WIDTH = 1920;
// Screens narrower than 4:3 or wider than 21:9 get bars instead of a grid
// stretched out of shape.
const MIN_ASPECT = 4 / 3;
const MAX_ASPECT = 21 / 9;
// The text sizes a display can have, in percent (internal/displays.TextScales).
const TEXT_SCALES = [100, 125, 150, 200];

export interface Stage {
  /** Logical size in CSS pixels, before scaling to the screen. */
  width: number;
  height: number;
  /** Factor from logical to screen pixels. */
  scale: number;
  /** Text size as a factor: 1.5 for 150 %. */
  textScale: number;
}

/**
 * The stage for a screen. It takes the screen's shape, so a 16:10 Nest Hub
 * or a 4:3 tablet has no black bars, and a larger text size makes it
 * narrower, so the same layout is drawn larger. Spacing that is defined in
 * logical pixels must be divided by textScale to keep its look.
 */
export function stageFor(screenWidth: number, screenHeight: number, textScalePercent: unknown): Stage {
  const percent = typeof textScalePercent === 'number' && TEXT_SCALES.includes(textScalePercent) ? textScalePercent : 100;
  const textScale = percent / 100;
  const aspect = Math.min(MAX_ASPECT, Math.max(MIN_ASPECT, screenWidth / Math.max(1, screenHeight)));
  const width = DESIGN_WIDTH / textScale;
  const height = width / aspect;
  return { width, height, scale: Math.min(screenWidth / width, screenHeight / height), textScale };
}
