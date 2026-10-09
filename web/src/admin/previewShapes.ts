// The screens the editor can preview a dashboard on, and how a preview is
// scaled to fit the dialog.

export interface Shape {
  id: string;
  label: string;
  /** Size of the preview window in CSS pixels. Only the shape matters to the layout. */
  width: number;
  height: number;
}

export const SHAPES: Shape[] = [
  { id: 'tv', label: '16:9 TV', width: 1280, height: 720 },
  { id: 'tablet', label: '16:10 tablet', width: 1280, height: 800 },
  { id: 'hub', label: 'Nest Hub (1024 × 600)', width: 1024, height: 600 },
];

// Text sizes the server offers (internal/displays.TextScales).
export const TEXT_SIZES: [number, string][] = [
  [100, '100 % (TV)'],
  [125, '125 %'],
  [150, '150 % (small screen)'],
  [200, '200 %'],
];

// A frame is never drawn smaller than this, so it stays visible while the
// dialog is still being measured.
const MIN_SCALE = 0.1;

export interface Fit {
  /** Factor from the shape's size to what is drawn; never above 1, to keep text sharp. */
  scale: number;
  /** The drawn size, which the frame's wrapper takes. */
  width: number;
  height: number;
}

/** The largest size of shape that fits in the space, with its shape kept. */
export function fitFrame(shape: Shape, maxWidth: number, maxHeight: number): Fit {
  const scale = Math.max(MIN_SCALE, Math.min(1, maxWidth / shape.width, maxHeight / shape.height));
  return { scale, width: Math.round(shape.width * scale), height: Math.round(shape.height * scale) };
}
