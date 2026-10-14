// The OLED rating of every theme, worked out from its colours when the web
// interface is built; see ThemeRating in build/oledRating.ts, which an
// ambient module cannot import by its relative path.
declare module 'virtual:oled-ratings' {
  export const OLED_RATINGS: Record<string, { rating: 'oled-friendly' | 'ok' | 'bright'; level: number }>;
}
