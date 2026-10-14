import { readFile } from 'node:fs/promises';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { KNMI_KEY } from './env';
import { solarAnswer } from './solar-stand-in';

// A stand-in for Buienradar, MeteoAlarm and Open-Meteo's solar forecast,
// so the Rain, Weather warnings and Solar forecast widgets show data
// without the internet, for a municipality's waste calendar, which a test
// pastes into the Waste collection setup, for the secret iCal address of a
// Google calendar, which a test pastes into Calendars, and for the ANWB's traffic list,
// the Overpass API and the KNMI, which answer with the Go tests' recorded
// answers. The
// server is pointed here with GDASH_TEST_UPSTREAM (cmd/gdash/config.go);
// the paths are the ones cmd/gdash/warnings.go, cmd/gdash/traffic.go and
// cmd/gdash/news.go name; the news path stands in for the feeds of the
// news catalogue, so the tests choose real catalogue sources without the
// internet. Open-Meteo's path answers 404: the
// tests use a place in the Netherlands, which Buienradar covers. Every
// answer is built around the current time, so there is always something
// ahead. /open-meteo-solar/requests lists the queries the solar forecast
// was asked with.

const STEP_MS = 5 * 60_000;
const HOUR_MS = 3_600_000;

/** Buienradar's values for the next two hours: dry for 20 minutes, a moderate shower, then dry. */
const SHOWER = [0, 0, 0, 0, 100, 115, 125, 130, 120, 105, 90, 77, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0];

const dutchClock = new Intl.DateTimeFormat('en-GB', { timeZone: 'Europe/Amsterdam', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });

/** The shower as Buienradar writes it: "VVV|HH:MM" per five minutes in Dutch time. */
function raintext(now: number): string {
  const first = Math.ceil(now / STEP_MS) * STEP_MS;
  return SHOWER.map((value, i) => `${String(value).padStart(3, '0')}|${dutchClock.format(first + i * STEP_MS)}`).join('\r\n');
}

/** One CAP alert in MeteoAlarm's JSON shape, in English and Dutch. */
function alert(id: string, area: [string, string], level: string, kind: string, texts: { en: string; nl: string }, onset: number, expires: number) {
  const info = (language: string, headline: string) => ({
    area: [{ areaDesc: area[0], geocode: [{ value: area[1], valueName: 'EMMA_ID' }] }],
    event: headline,
    headline,
    language,
    onset: new Date(onset).toISOString(),
    expires: new Date(expires).toISOString(),
    parameter: [
      { value: level, valueName: 'awareness_level' },
      { value: kind, valueName: 'awareness_type' },
    ],
  });
  return {
    alert: {
      identifier: id,
      info: [info('en-GB', texts.en), info('nl-NL', texts.nl)],
      msgType: 'Alert',
      sent: new Date(onset - HOUR_MS).toISOString(),
      status: 'Actual',
    },
  };
}

/** An orange wind warning in effect and a yellow rain warning tonight for Noord-Brabant, and a red one for Zeeland. */
function feed(now: number): unknown {
  return {
    warnings: [
      alert('e2e-wind', ['Noord-Brabant', 'NL013'], '3; orange; Severe', '1; Wind', { en: 'Severe gusts of wind', nl: 'Zware windstoten' }, now - HOUR_MS, now + 6 * HOUR_MS),
      alert('e2e-rain', ['Noord-Brabant', 'NL013'], '2; yellow; Moderate', '10; Rain', { en: 'A lot of rain', nl: 'Veel regen' }, now + 3 * HOUR_MS, now + 9 * HOUR_MS),
      alert('e2e-storm', ['Zeeland', 'NL010'], '4; red; Extreme', '3; Thunderstorm', { en: 'Very heavy thunderstorms', nl: 'Zeer zware onweersbuien' }, now - HOUR_MS, now + 2 * HOUR_MS),
    ],
  };
}

/** Stand-in headlines for two sources of the news catalogue, by catalogue id (internal/news/catalogue.json). */
export const NEWS_STAND_INS: Record<string, string[]> = {
  'nos-algemeen': ['Kitchen headline from the NOS stand-in', 'Second NOS stand-in headline'],
  tweakers: ['Study headline from the Tweakers stand-in', 'Second Tweakers stand-in headline'],
};

const escapeXml = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;');

/** The RSS of a catalogue source, its headlines a few minutes old; GDash asks for it at /news/<id> (cmd/gdash/news.go). */
function newsFeed(id: string, now: number): string | undefined {
  const titles = NEWS_STAND_INS[id];
  if (!titles) return undefined;
  const items = titles
    .map((title, i) => `<item><title>${escapeXml(title)}</title><pubDate>${new Date(now - (i + 1) * STEP_MS).toUTCString()}</pubDate></item>`)
    .join('');
  return `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>${escapeXml(id)}</title>${items}</channel></rss>`;
}

/** A day from now as an iCal date, YYYYMMDD, in the time zone of the machine, which the server shares. */
function icalDay(now: number, days: number): string {
  const d = new Date(now);
  d.setDate(d.getDate() + days);
  return `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`;
}

/** A municipality's waste calendar: GFT tomorrow, PMD in three days, paper in nine, Christmas trees in twenty and bulky waste in a month. */
function wasteCalendar(now: number): string {
  const pickups: [number, string][] = [
    [1, 'GFT'],
    [3, 'Plastic\\, blik en drinkpakken (PMD)'],
    [9, 'Papier en karton'],
    [20, 'Kerstbomen'],
    [30, 'Grofvuil op afspraak'],
  ];
  const events = pickups.flatMap(([days, title]) => [
    'BEGIN:VEVENT',
    `UID:e2e-waste-${days}@afval.example`,
    `DTSTART;VALUE=DATE:${icalDay(now, days)}`,
    `DTEND;VALUE=DATE:${icalDay(now, days + 1)}`,
    `SUMMARY:${title}`,
    'END:VEVENT',
  ]);
  return ['BEGIN:VCALENDAR', 'VERSION:2.0', 'PRODID:-//E2E Afvalkalender//NL', ...events, 'END:VCALENDAR', ''].join('\r\n');
}

/** Where the Google stand-in serves a secret iCal address, %40 and all, as Google writes it. */
export const GOOGLE_CALENDAR_PATH = '/calendar/ical/family.e2e%40gmail.com/private-0123456789abcdef/basic.ics';

/**
 * A family calendar as Google's secret iCal address serves it: its time
 * zone, a weekly lesson since last year with its reminder, a dentist
 * tomorrow at half past eight Dutch time with notes, and a meeting from
 * years back.
 */
function googleCalendar(now: number): string {
  const lessonStart = icalDay(now, -365);
  return [
    'BEGIN:VCALENDAR',
    'PRODID:-//Google Inc//Google Calendar 70.9054//EN',
    'VERSION:2.0',
    'CALSCALE:GREGORIAN',
    'METHOD:PUBLISH',
    'X-WR-CALNAME:Gezin',
    'X-WR-TIMEZONE:Europe/Amsterdam',
    'BEGIN:VTIMEZONE',
    'TZID:Europe/Amsterdam',
    'BEGIN:DAYLIGHT',
    'TZOFFSETFROM:+0100',
    'TZOFFSETTO:+0200',
    'DTSTART:19700329T020000',
    'RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU',
    'END:DAYLIGHT',
    'BEGIN:STANDARD',
    'TZOFFSETFROM:+0200',
    'TZOFFSETTO:+0100',
    'DTSTART:19701025T030000',
    'RRULE:FREQ=YEARLY;BYMONTH=10;BYDAY=-1SU',
    'END:STANDARD',
    'END:VTIMEZONE',
    'BEGIN:VEVENT',
    `DTSTART;TZID=Europe/Amsterdam:${lessonStart}T190000`,
    `DTEND;TZID=Europe/Amsterdam:${lessonStart}T200000`,
    'RRULE:FREQ=WEEKLY',
    'UID:e2e-lesson@google.com',
    'SUMMARY:Zwemles',
    'BEGIN:VALARM',
    'ACTION:DISPLAY',
    'TRIGGER:-P0DT0H30M0S',
    'END:VALARM',
    'END:VEVENT',
    'BEGIN:VEVENT',
    `DTSTART;TZID=Europe/Amsterdam:${icalDay(now, 1)}T083000`,
    `DTEND;TZID=Europe/Amsterdam:${icalDay(now, 1)}T091500`,
    'UID:e2e-dentist@google.com',
    'DESCRIPTION:Bring the insurance card. A long note that Google folds at se',
    ' venty-five octets.',
    'SUMMARY:Tandarts',
    'END:VEVENT',
    'BEGIN:VEVENT',
    'DTSTART:20150612T080000Z',
    'DTEND:20150612T090000Z',
    'UID:e2e-long-ago@google.com',
    'SUMMARY:Overleg 2015',
    'END:VEVENT',
    'END:VCALENDAR',
    '',
  ].join('\r\n');
}

function answer(path: string, now: number): { type: string; body: string } | undefined {
  if (path === GOOGLE_CALENDAR_PATH) return { type: 'text/calendar; charset=UTF-8', body: googleCalendar(now) };
  // What Google answers for its embed and browser addresses: a web page.
  if (path === '/calendar/embed') return { type: 'text/html; charset=utf-8', body: '<!doctype html><html><head><title>Google Calendar</title></head></html>' };
  if (path.startsWith('/news/')) {
    const body = newsFeed(path.slice('/news/'.length), now);
    return body === undefined ? undefined : { type: 'application/rss+xml', body };
  }
  if (path === '/buienradar/data/raintext') return { type: 'text/plain', body: raintext(now) };
  if (path === '/waste/afvalkalender.ics') return { type: 'text/calendar', body: wasteCalendar(now) };
  if (path === '/meteoalarm/api/v1/warnings/feeds-netherlands') return { type: 'application/json', body: JSON.stringify(feed(now)) };
  if (path === '/open-meteo-solar/v1/forecast') return { type: 'application/json', body: JSON.stringify(solarAnswer(now)) };
  return undefined;
}

const here = path.dirname(fileURLToPath(import.meta.url));
const recorded = (pkg: string, file: string) => readFile(path.resolve(here, '../../internal', pkg, 'testdata', file), 'utf8');

/** The recorded answers: jams on the A2 near Born, two fixed cameras on the A2, and one hour of a KNMI station. */
async function recordedAnswers(): Promise<(path: string, authorization: string | undefined) => { status: number; body: string }> {
  const [incidents, cameras, locations, observation] = await Promise.all([
    recorded('traffic', 'incidents.json'),
    recorded('speedcameras', 'overpass.json'),
    recorded('knmi', 'locations.json'),
    recorded('knmi', 'observation.json'),
  ]);
  return (path, authorization) => {
    if (path === '/anwb/traffic') return { status: 200, body: incidents };
    if (path === '/overpass/api/interpreter') return { status: 200, body: cameras };
    if (!path.startsWith('/knmi/edr/')) return { status: 404, body: '' };
    if (authorization !== KNMI_KEY) return { status: 401, body: '{"error":"Unauthorized"}' };
    if (path === '/knmi/edr/locations') return { status: 200, body: locations };
    // Every station measured the same hour.
    if (path.startsWith('/knmi/edr/locations/')) return { status: 200, body: observation };
    return { status: 404, body: '' };
  };
}

/** Starts the stand-in on a free port of the loopback address. */
export async function startUpstream(): Promise<{ url: string; close: () => Promise<void> }> {
  const replay = await recordedAnswers();
  const solarQueries: string[] = [];
  const server: Server = createServer((req, res) => {
    const url = new URL(req.url ?? '/', 'http://upstream');
    const pathname = url.pathname;
    if (pathname === '/open-meteo-solar/requests') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(solarQueries));
      return;
    }
    if (pathname === '/open-meteo-solar/v1/forecast') solarQueries.push(url.search);
    const found = answer(pathname, Date.now());
    if (!found) {
      const fixed = replay(pathname, req.headers.authorization);
      // Read a POST body (Overpass) before answering, so the client sees no reset.
      req.resume();
      req.on('end', () => {
        res.writeHead(fixed.status, { 'Content-Type': 'application/json' });
        res.end(fixed.body);
      });
      return;
    }
    res.writeHead(200, { 'Content-Type': found.type });
    res.end(found.body);
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  return {
    url: `http://127.0.0.1:${(server.address() as AddressInfo).port}`,
    close: async () => {
      server.closeAllConnections();
      await new Promise((resolve) => server.close(resolve));
    },
  };
}
