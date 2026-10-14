// What a pasted calendar address is, so the Calendars setup can explain
// the common mistakes before and after a test: GDash reads iCal (ICS)
// addresses, and Google, Outlook and iCloud each also offer links that
// open a web page instead. The server decides whether an address works;
// this only words the advice.

export type CalendarService = 'google' | 'outlook' | 'icloud' | 'other';

export interface CalendarAddressCheck {
  service: CalendarService;
  /** The address cannot work as it is; says what to copy instead. */
  problem?: string;
  /** The address may work, with a condition worth knowing. */
  note?: string;
}

const GOOGLE_HOSTS = ['calendar.google.com', 'www.google.com'];
const OUTLOOK_HOSTS = ['outlook.live.com', 'outlook.office365.com', 'outlook.office.com'];

const GOOGLE_SECRET =
  'In Google Calendar on a computer, open Settings, choose the calendar under “Settings for my calendars”, and copy the “Secret address in iCal format” under “Integrate calendar”.';
const OUTLOOK_PUBLISH =
  'In Outlook on the web, open Settings, Calendar, Shared calendars, publish the calendar under “Publish a calendar”, and copy the ICS link.';
const ICLOUD_PUBLIC = 'In the Calendar app, share the calendar, turn on “Public Calendar” and copy the link it shows (webcal://…).';

/** Which service an address is from, and whether it is the kind GDash reads. */
export function checkCalendarAddress(raw: string): CalendarAddressCheck {
  const url = parse(raw);
  if (!url) return { service: 'other' };
  const host = url.hostname.toLowerCase();
  const path = url.pathname;
  if (GOOGLE_HOSTS.includes(host) && (host !== 'www.google.com' || path.startsWith('/calendar'))) return checkGoogle(url);
  if (OUTLOOK_HOSTS.includes(host)) return checkOutlook(path);
  if (host === 'icloud.com' || host.endsWith('.icloud.com')) return checkIcloud(host, path);
  return { service: 'other' };
}

function checkGoogle(url: URL): CalendarAddressCheck {
  const path = url.pathname;
  if (path.startsWith('/calendar/ical/')) {
    if (path.includes('/public/')) {
      return {
        service: 'google',
        note: 'This is the public address: it only works when the calendar is public. For a private calendar, use the secret address.',
      };
    }
    return { service: 'google' };
  }
  if (path.startsWith('/calendar/embed')) {
    return { service: 'google', problem: `This is the address for embedding the calendar in a web page, not its iCal address. ${GOOGLE_SECRET}` };
  }
  const kind = url.searchParams.has('cid') ? 'a link to add the calendar' : 'the address of Google Calendar in the browser';
  return { service: 'google', problem: `This is ${kind}, not an iCal address. ${GOOGLE_SECRET}` };
}

function checkOutlook(path: string): CalendarAddressCheck {
  const lower = path.toLowerCase();
  if (lower.endsWith('.ics')) return { service: 'outlook' };
  if (lower.endsWith('.html')) {
    return { service: 'outlook', problem: 'This is the HTML link of a published calendar, which opens a web page. Copy the ICS link beside it.' };
  }
  return { service: 'outlook', problem: `This is the address of Outlook in the browser, not a calendar link. ${OUTLOOK_PUBLISH}` };
}

function checkIcloud(host: string, path: string): CalendarAddressCheck {
  if (path.startsWith('/published/')) return { service: 'icloud' };
  if (host === 'www.icloud.com' || host === 'icloud.com') {
    return { service: 'icloud', problem: `This is the address of iCloud in the browser, not a calendar link. ${ICLOUD_PUBLIC}` };
  }
  return { service: 'icloud' };
}

/** More advice when a test failed with this error code of the server. */
export function explainCalendarFailure(raw: string, code: string): string {
  const check = checkCalendarAddress(raw);
  if (check.problem) return check.problem;
  switch (check.service) {
    case 'google':
      if (code === 'calendar_not_found') {
        return check.note
          ? 'Google answers this for the public address of a calendar that is not public. Use the secret address in iCal format instead.'
          : 'The secret address was reset, or the calendar was removed. Copy it again from the calendar’s settings.';
      }
      if (code === 'not_a_calendar') return GOOGLE_SECRET;
      return '';
    case 'outlook':
      return code === 'calendar_not_found' ? 'The calendar is no longer published, or was published again with a new link. Copy the ICS link again.' : '';
    case 'icloud':
      return code === 'calendar_not_found' ? 'Public Calendar was turned off, or the calendar was removed. Turn it on again and copy the new link.' : '';
    default:
      return code === 'not_a_calendar' ? 'Look in the calendar app for a link named iCal, ICS, webcal or “subscribe”.' : '';
  }
}

function parse(raw: string): URL | null {
  const text = raw.trim().replace(/^webcal:\/\//i, 'https://');
  try {
    const url = new URL(text);
    return url.protocol === 'https:' || url.protocol === 'http:' ? url : null;
  } catch {
    return null;
  }
}
