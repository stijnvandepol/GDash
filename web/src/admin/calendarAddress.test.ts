import { describe, expect, it } from 'vitest';
import { checkCalendarAddress, explainCalendarFailure } from './calendarAddress';

const SECRET = 'https://calendar.google.com/calendar/ical/family.smith%40gmail.com/private-0123456789abcdef/basic.ics';
const PUBLIC = 'https://calendar.google.com/calendar/ical/nl.dutch%23holiday%40group.v.calendar.google.com/public/basic.ics';

describe('checkCalendarAddress', () => {
  it('accepts the iCal addresses of Google, Outlook and iCloud', () => {
    expect(checkCalendarAddress(SECRET)).toEqual({ service: 'google' });
    expect(checkCalendarAddress('https://outlook.office365.com/owa/calendar/abc@example.com/def/calendar.ics')).toEqual({ service: 'outlook' });
    expect(checkCalendarAddress('https://outlook.live.com/owa/calendar/00000000-0000/abc/cid-X/calendar.ics')).toEqual({ service: 'outlook' });
    expect(checkCalendarAddress('webcal://p52-caldav.icloud.com/published/2/MTIzNDU2Nzg5')).toEqual({ service: 'icloud' });
  });

  it('warns that a public Google address needs a public calendar', () => {
    const check = checkCalendarAddress(PUBLIC);
    expect(check.service).toBe('google');
    expect(check.problem).toBeUndefined();
    expect(check.note).toContain('only works when the calendar is public');
  });

  it('explains the Google links that open a web page', () => {
    for (const url of [
      'https://calendar.google.com/calendar/embed?src=family.smith%40gmail.com&ctz=Europe%2FAmsterdam',
      'https://calendar.google.com/calendar/r',
      'https://calendar.google.com/calendar/u/0/r/week',
      'https://calendar.google.com/calendar/u/0?cid=ZmFtaWx5LnNtaXRoQGdtYWlsLmNvbQ',
      'https://www.google.com/calendar/render',
    ]) {
      const check = checkCalendarAddress(url);
      expect(check.service, url).toBe('google');
      expect(check.problem, url).toContain('Secret address in iCal format');
    }
    expect(checkCalendarAddress('https://calendar.google.com/calendar/embed?src=x').problem).toContain('embedding');
    expect(checkCalendarAddress('https://calendar.google.com/calendar/u/0?cid=x').problem).toContain('a link to add the calendar');
  });

  it('explains the Outlook and iCloud links that open a web page', () => {
    expect(checkCalendarAddress('https://outlook.office365.com/owa/calendar/abc@example.com/def/calendar.html').problem).toContain('ICS link');
    expect(checkCalendarAddress('https://outlook.live.com/calendar/0/view/month').problem).toContain('Publish a calendar');
    expect(checkCalendarAddress('https://www.icloud.com/calendar/').problem).toContain('Public Calendar');
  });

  it('leaves other addresses to the test', () => {
    expect(checkCalendarAddress('https://cloud.example.org/remote.php/dav/calendars/me/family?export')).toEqual({ service: 'other' });
    expect(checkCalendarAddress('not an address')).toEqual({ service: 'other' });
    expect(checkCalendarAddress('ftp://calendar.google.com/calendar/r')).toEqual({ service: 'other' });
    // A Google search page is no calendar link of Google's.
    expect(checkCalendarAddress('https://www.google.com/search?q=ical')).toEqual({ service: 'other' });
  });
});

describe('explainCalendarFailure', () => {
  it('tells a public address of a private Google calendar from a reset secret one', () => {
    expect(explainCalendarFailure(PUBLIC, 'calendar_not_found')).toContain('not public');
    expect(explainCalendarFailure(SECRET, 'calendar_not_found')).toContain('was reset');
  });

  it('repeats what is wrong with the address itself first', () => {
    expect(explainCalendarFailure('https://calendar.google.com/calendar/embed?src=x', 'not_a_calendar')).toContain('embedding');
  });

  it('points to the provider’s steps after other failures', () => {
    expect(explainCalendarFailure('https://outlook.office365.com/owa/calendar/a/b/calendar.ics', 'calendar_not_found')).toContain('published');
    expect(explainCalendarFailure('webcal://p52-caldav.icloud.com/published/2/x', 'calendar_not_found')).toContain('Public Calendar');
    expect(explainCalendarFailure('https://example.org/page', 'not_a_calendar')).toContain('iCal, ICS, webcal');
    expect(explainCalendarFailure(SECRET, 'fetch_failed')).toBe('');
  });
});
