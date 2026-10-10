// What the editor says about each theme and typeface. The themes themselves
// are in ../themes.css and ../presets.css; internal/display lists the same
// names, and themes.test.ts keeps the three in step.

export interface ThemeInfo {
  name: string;
  /** One line about the look, for the style picker. */
  note: string;
  /** The typeface the theme uses unless a dashboard picks another. */
  font: FontId;
}

export type FontId =
  | 'geist'
  | 'inter'
  | 'outfit'
  | 'space-grotesk'
  | 'lora'
  | 'cormorant'
  | 'bodoni'
  | 'news-cycle'
  | 'jetbrains-mono'
  | 'im-fell';

export const THEMES: Record<string, ThemeInfo> = {
  'minimal-dark': { name: 'Minimal dark', note: 'Graphite with a signal orange. Calm on any TV.', font: 'geist' },
  'minimal-light': { name: 'Minimal light', note: 'Light grey and white for bright rooms.', font: 'geist' },
  glass: { name: 'Glass', note: 'Translucent tiles over slate green.', font: 'geist' },
  'warm-light': { name: 'Warm light', note: 'Cream and brown, softer than white.', font: 'geist' },
  'high-contrast': { name: 'High contrast', note: 'White on black with clear borders.', font: 'geist' },
  night: { name: 'Night', note: 'Dim amber on black for a bedroom.', font: 'geist' },
  frost: { name: 'Frost', note: 'Frosted white tiles on a pastel wallpaper, round and airy.', font: 'inter' },
  walnut: { name: 'Walnut', note: 'Dark wood with a straight grain and brass.', font: 'lora' },
  swiss: { name: 'Swiss', note: 'Square tiles, heavy black rules, one red.', font: 'space-grotesk' },
  tonal: { name: 'Tonal', note: 'Soft tonal greens and very round tiles.', font: 'outfit' },
  gilded: { name: 'Gilded', note: 'Black and gold with a classic serif.', font: 'cormorant' },
  riviera: { name: 'Riviera', note: 'Cream and terracotta in an Italian serif.', font: 'bodoni' },
  galaxy: { name: 'Galaxy', note: 'A field of stars and the yellow of an opening crawl.', font: 'news-cycle' },
  terminal: { name: 'Terminal', note: 'Green phosphor and scan lines.', font: 'jetbrains-mono' },
  parchment: { name: 'Parchment', note: 'Aged paper and ink, for a wizard’s kitchen.', font: 'im-fell' },
  neon: { name: 'Neon', note: 'A synthwave night in magenta and cyan.', font: 'outfit' },
};

export const FONTS: Record<FontId, string> = {
  geist: 'Geist',
  inter: 'Inter',
  outfit: 'Outfit',
  'space-grotesk': 'Space Grotesk',
  lora: 'Lora',
  cormorant: 'Cormorant',
  bodoni: 'Bodoni',
  'news-cycle': 'News Cycle',
  'jetbrains-mono': 'JetBrains Mono',
  'im-fell': 'IM Fell',
};

export function themeName(id: string): string {
  return THEMES[id]?.name ?? id;
}

/** The typeface a dashboard shows: its own choice, or its theme's. */
export function fontName(theme: string, font: string): string {
  const id = (font || THEMES[theme]?.font || 'geist') as FontId;
  return FONTS[id] ?? font;
}
