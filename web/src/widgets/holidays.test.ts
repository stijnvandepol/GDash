import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { display } from '../state.svelte';
import type { Snapshot } from '../protocol';
import { dayRange, holidayName, holidaySettings, holidayWhen, readHolidays, upcomingHolidays, type Holiday } from './holidays';

// Tuesday 13 October 2026, late in the evening: the day must not shift.
const now = new Date(2026, 9, 13, 23, 40);

const autumnNorth = { kind: 'autumn', name: 'Herfstvakantie', start: '2026-10-10', end: '2026-10-18' };
const autumnMiddle = { kind: 'autumn', name: 'Herfstvakantie', start: '2026-10-17', end: '2026-10-25' };
const christmas = { kind: 'christmas', name: 'Kerstvakantie', start: '2026-12-19', end: '2027-01-03' };
const summer2026 = { kind: 'summer', name: 'Zomervakantie', start: '2026-07-18', end: '2026-08-30' };
// What the server sends, as of a day before: its list can be a day old.
const value = {
  public: [
    { kind: 'christmas', date: '2026-12-25' },
    { kind: 'boxingDay', date: '2026-12-26' },
    { kind: 'kingsDay', date: '2027-04-27' },
    { kind: 'carnival', date: '2027-02-08' },
  ],
  north: [autumnNorth, christmas],
  middle: [summer2026, autumnMiddle, christmas, { kind: 'autumn', start: 'soon' }],
  south: [],
};

function speak(language: string) {
  display.snapshot = { language } as Snapshot;
}

beforeEach(() => {
  vi.stubGlobal('navigator', { language: 'en-US' });
});

afterEach(() => {
  display.snapshot = null;
  vi.unstubAllGlobals();
});

describe('holidaySettings', () => {
  it('fills in what is not set, as the server does', () => {
    expect(holidaySettings(undefined)).toEqual({ region: 'middle', show: 'both', maxItems: 5 });
    expect(holidaySettings({ region: 'zuid', show: 'all', maxItems: 0 })).toEqual({ region: 'middle', show: 'both', maxItems: 5 });
    expect(holidaySettings({ region: 'south', show: 'public', maxItems: 3 })).toEqual({ region: 'south', show: 'public', maxItems: 3 });
  });
});

describe('readHolidays', () => {
  it('reads the region and the public holidays, and skips what it does not know', () => {
    const both = readHolidays(value, 'north', 'both');
    expect(both?.map((h) => `${h.type}:${h.kind}`)).toEqual(['school:autumn', 'school:christmas', 'public:christmas', 'public:boxingDay', 'public:kingsDay']);
    expect(readHolidays(value, 'middle', 'school')).toHaveLength(3);
    expect(readHolidays(value, 'south', 'school')).toEqual([]);
  });

  it('needs the region only for school holidays', () => {
    const publicOnly = { public: value.public };
    expect(readHolidays(publicOnly, 'north', 'public')).toHaveLength(3);
    // The server adds a region once it has read the school holidays.
    expect(readHolidays(publicOnly, 'north', 'both')).toBeUndefined();
    expect(readHolidays({ north: [] }, 'north', 'both')).toBeUndefined();
  });

  it('is undefined while there is no usable data', () => {
    expect(readHolidays(undefined, 'middle', 'both')).toBeUndefined();
    expect(readHolidays([], 'middle', 'both')).toBeUndefined();
    expect(readHolidays({ public: 'soon', middle: [] }, 'middle', 'both')).toBeUndefined();
  });
});

describe('upcomingHolidays', () => {
  it('drops what has ended and lists the running holiday first', () => {
    const list = upcomingHolidays(readHolidays(value, 'middle', 'both')!, now, 10);
    expect(list.map((h) => `${h.kind}:${h.daysToStart}`)).toEqual(['autumn:4', 'christmas:67', 'christmas:73', 'boxingDay:74', 'kingsDay:196']);
    // The same start: the school holiday comes before the public one.
    const sameDay: Holiday[] = [
      { type: 'public', kind: 'christmas', start: '2026-12-25', end: '2026-12-25' },
      { type: 'school', kind: 'christmas', name: 'Kerstvakantie', start: '2026-12-25', end: '2027-01-03' },
    ];
    expect(upcomingHolidays(sameDay, now, 10)[0]?.type).toBe('school');
  });

  it('keeps at most max', () => {
    expect(upcomingHolidays(readHolidays(value, 'north', 'both')!, now, 2).map((h) => h.kind)).toEqual(['autumn', 'christmas']);
  });
});

describe('holidayWhen', () => {
  const upcoming = (h: Holiday, at = now) => upcomingHolidays([h], at, 1)[0]!;
  const school = (h: typeof autumnMiddle): Holiday => ({ type: 'school', ...h }) as Holiday;

  it('says until when a running holiday lasts', () => {
    expect(holidayWhen(upcoming(school(autumnNorth)), 'en-GB')).toBe('Now, until Sunday');
    expect(holidayWhen(upcoming(school(autumnNorth), new Date(2026, 9, 18, 9)), 'en-GB')).toBe('Last day today');
    expect(holidayWhen(upcoming(school(christmas), new Date(2026, 11, 20, 9)), 'en-GB')).toBe('Now, until 3 Jan');
  });

  it('counts the days to a holiday that starts soon, and gives the dates of one further off', () => {
    expect(holidayWhen(upcoming(school(autumnMiddle)), 'en-GB')).toBe('Starts in 4 days (17–25 Oct)');
    expect(holidayWhen(upcoming(school(autumnMiddle), new Date(2026, 9, 16, 8)), 'en-GB')).toBe('Starts tomorrow (17–25 Oct)');
    expect(holidayWhen(upcoming(school(christmas)), 'en-GB')).toBe('19 Dec – 3 Jan');
  });

  it('gives the day of a public holiday', () => {
    const kingsDay: Holiday = { type: 'public', kind: 'kingsDay', start: '2027-04-27', end: '2027-04-27' };
    expect(holidayWhen(upcoming(kingsDay), 'en-GB')).toBe('Tue 27 Apr');
    expect(holidayWhen(upcoming(kingsDay, new Date(2027, 3, 26, 20)), 'en-GB')).toBe('Tomorrow');
    expect(holidayWhen(upcoming(kingsDay, new Date(2027, 3, 27, 8)), 'en-GB')).toBe('Today');
  });

  it('speaks the language of the screen', () => {
    speak('nl');
    expect(holidayWhen(upcoming(school(autumnNorth)), 'nl-NL')).toBe('Nu, tot en met zondag');
    expect(holidayWhen(upcoming(school(autumnMiddle)), 'nl-NL')).toBe('Begint over 4 dagen (17–25 okt)');
    speak('de');
    expect(holidayWhen(upcoming(school(autumnMiddle)), 'de-DE')).toBe('Beginnt in 4 Tagen (17.–25. Okt.)');
    speak('fr');
    expect(holidayWhen(upcoming(school(autumnNorth)), 'fr-FR')).toBe('En cours, jusqu’à dimanche');
  });
});

describe('dayRange', () => {
  it('names the month once within a month and both across months', () => {
    expect(dayRange('2026-10-17', '2026-10-25', 'en-GB')).toBe('17–25 Oct');
    expect(dayRange('2026-12-19', '2027-01-03', 'nl-NL')).toBe('19 dec – 3 jan');
    expect(dayRange('2026-12-19', 'later', 'en-GB')).toBe('2026-12-19 – later');
  });
});

describe('holidayName', () => {
  it('translates the known holidays and keeps the Dutch name of others', () => {
    expect(holidayName({ type: 'public', kind: 'kingsDay', start: '2027-04-27', end: '2027-04-27' })).toBe('King’s Day');
    expect(holidayName({ type: 'school', kind: 'autumn', name: 'Herfstvakantie', start: '', end: '' })).toBe('Autumn holiday');
    expect(holidayName({ type: 'school', kind: 'other', name: 'Pinkstervakantie', start: '', end: '' })).toBe('Pinkstervakantie');
    speak('de');
    expect(holidayName({ type: 'public', kind: 'ascension', start: '', end: '' })).toBe('Christi Himmelfahrt');
    speak('nl');
    expect(holidayName({ type: 'school', kind: 'autumn', name: 'Herfstvakantie', start: '', end: '' })).toBe('Herfstvakantie');
  });
});
