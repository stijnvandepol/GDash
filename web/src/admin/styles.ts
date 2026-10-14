// What the editor says about each theme and typeface. The themes themselves
// are in ../themes.css and the presets files; internal/display lists the same
// names, and themes.test.ts keeps the three in step.

/** The sections of the style picker. */
export const GROUPS = [
  { id: 'essentials', name: 'Essentials', note: 'Calm and readable in any room.' },
  { id: 'character', name: 'Character', note: 'Wood, gold, glass and paper, each with a typeface of its own.' },
  { id: 'screen', name: 'Film and series', note: 'Inspired by a film or a series, without its names or logos.' },
  { id: 'games', name: 'Games', note: 'Inspired by video games, without their names, characters or logos.' },
] as const;

export type GroupId = (typeof GROUPS)[number]['id'];

export interface ThemeInfo {
  name: string;
  group: GroupId;
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
  | 'im-fell'
  | 'courier-prime'
  | 'oswald'
  | 'jersey';

export const THEMES: Record<string, ThemeInfo> = {
  'minimal-dark': { name: 'Minimal dark', group: 'essentials', note: 'Graphite with a signal orange. Calm on any TV.', font: 'geist' },
  'minimal-light': { name: 'Minimal light', group: 'essentials', note: 'Light grey and white for bright rooms.', font: 'geist' },
  glass: { name: 'Glass', group: 'essentials', note: 'Translucent tiles over slate green.', font: 'geist' },
  'warm-light': { name: 'Warm light', group: 'essentials', note: 'Cream and brown, softer than white.', font: 'geist' },
  'high-contrast': { name: 'High contrast', group: 'essentials', note: 'White on black with clear borders.', font: 'geist' },
  night: { name: 'Night', group: 'essentials', note: 'Dim amber on black for a bedroom.', font: 'geist' },
  frost: { name: 'Frost', group: 'character', note: 'Frosted white tiles on a pastel wallpaper, round and airy.', font: 'inter' },
  'frost-night': { name: 'Frost night', group: 'character', note: 'Frost after dark, like a phone in dark mode.', font: 'inter' },
  walnut: { name: 'Walnut', group: 'character', note: 'Dark wood with a straight grain and brass.', font: 'lora' },
  swiss: { name: 'Swiss', group: 'character', note: 'Square tiles, heavy black rules, one red.', font: 'space-grotesk' },
  tonal: { name: 'Tonal', group: 'character', note: 'Soft tonal greens and very round tiles.', font: 'outfit' },
  candy: { name: 'Candy', group: 'character', note: 'Hot pink and white, the roundest of all.', font: 'outfit' },
  gilded: { name: 'Gilded', group: 'character', note: 'Black and gold with a classic serif.', font: 'cormorant' },
  riviera: { name: 'Riviera', group: 'character', note: 'Cream and terracotta in an Italian serif.', font: 'bodoni' },
  galaxy: { name: 'Galaxy', group: 'screen', note: 'A field of stars and the yellow of an opening crawl.', font: 'news-cycle' },
  terminal: { name: 'Terminal', group: 'screen', note: 'Green phosphor and scan lines, like a ship’s computer.', font: 'jetbrains-mono' },
  parchment: { name: 'Parchment', group: 'screen', note: 'Aged paper and ink, for a wizard’s kitchen.', font: 'im-fell' },
  neon: { name: 'Neon', group: 'screen', note: 'A synthwave night in magenta and cyan.', font: 'outfit' },
  'upside-down': { name: 'Upside down', group: 'screen', note: 'Red letters glowing on black, in a small town in the eighties.', font: 'bodoni' },
  noir: { name: 'Noir', group: 'screen', note: 'Black and white with film grain, typed out.', font: 'courier-prime' },
  mars: { name: 'Mars', group: 'screen', note: 'Rust-red dust and a low sun.', font: 'space-grotesk' },
  // Film, series and game presets of presets-film.css.
  hyperspace: { name: 'Hyperspace', group: 'screen', note: 'Star lines streaking out at the jump to light speed, in the spirit of space-opera sagas.', font: 'oswald' },
  'dark-side': { name: 'Dark side', group: 'screen', note: 'Black armour and crimson light, square tiles with glowing red edges.', font: 'oswald' },
  'desert-planet': { name: 'Desert planet', group: 'screen', note: 'Dunes under a dusk-purple sky, wide-set letters and spice orange.', font: 'outfit' },
  'wizard-school': { name: 'Wizard school', group: 'screen', note: 'A castle hall by candlelight: burgundy, night blue and gold leaf.', font: 'lora' },
  'middle-realm': { name: 'Middle realm', group: 'screen', note: 'An elven wood in deep green and pale gold, with a flowing serif.', font: 'cormorant' },
  'rain-city': { name: 'Rain city', group: 'screen', note: 'Rain through teal and orange signs in a city of the future.', font: 'space-grotesk' },
  'time-machine': { name: 'Time machine', group: 'screen', note: 'Brushed steel, amber readouts and two trails of fire, as in the eighties.', font: 'jetbrains-mono' },
  'secret-agent': { name: 'Agent 00', group: 'screen', note: 'Black tie, white letters set wide and rings like a barrel, in the spirit of spy films.', font: 'inter' },
  'shiny-hunt': { name: 'Shiny hunt', group: 'games', note: 'Pastel and full of sparkles, for the rare monster that shines.', font: 'outfit' },
  'pocket-console': { name: 'Pocket console', group: 'games', note: 'The four greens of a handheld from 1989, in a pixel face.', font: 'jersey' },
  'block-world': { name: 'Block world', group: 'games', note: 'Grass and dirt in square blocks, like a world built by hand.', font: 'jersey' },
  'quest-menu': { name: 'Quest menu', group: 'games', note: 'Deep blue menu windows over an old map, as in a fantasy adventure.', font: 'news-cycle' },
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
  'courier-prime': 'Courier Prime',
  oswald: 'Oswald',
  jersey: 'Jersey 10',
};

/**
 * The dark counterpart worth offering first for "After sunset": the same
 * look in the dark, or the closest dim theme.
 */
const NIGHT_PAIRS: Record<string, string> = { frost: 'frost-night', 'minimal-light': 'minimal-dark', 'warm-light': 'night' };

/**
 * The themes to offer after sunset for a dashboard in theme, in the server's
 * order with the counterpart of theme first. The theme itself is left out
 * (that is what "Keep this style" does) unless it is the one chosen.
 */
export function nightChoices(theme: string, themes: string[], chosen: string): string[] {
  const first = NIGHT_PAIRS[theme];
  const rest = themes.filter((id) => id !== first && (id !== theme || id === chosen));
  return first && themes.includes(first) ? [first, ...rest] : rest;
}

export function themeName(id: string): string {
  return THEMES[id]?.name ?? id;
}

/** The typeface a dashboard shows: its own choice, or its theme's. */
export function fontName(theme: string, font: string): string {
  const id = (font || THEMES[theme]?.font || 'geist') as FontId;
  return FONTS[id] ?? font;
}
