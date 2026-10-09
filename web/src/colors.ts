// Calendar colours, by the names internal/settings.CalendarColors uses.
// Mid tones, so a dot reads on light and on dark themes.
export const CALENDAR_COLORS: Record<string, string> = {
  blue: '#4f8cff',
  green: '#3fb96b',
  orange: '#f08c2e',
  purple: '#a66bff',
  red: '#e5534b',
  teal: '#2bb3b1',
  pink: '#e66aa6',
  yellow: '#d9b30f',
};

/** The shade for a colour name, or undefined for one Daycast does not know. */
export function calendarColor(name: unknown): string | undefined {
  return typeof name === 'string' && Object.prototype.hasOwnProperty.call(CALENDAR_COLORS, name)
    ? CALENDAR_COLORS[name]
    : undefined;
}
