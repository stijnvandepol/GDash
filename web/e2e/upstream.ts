import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { solarAnswer } from './solar-stand-in';

// A stand-in for Buienradar, MeteoAlarm and Open-Meteo's solar forecast,
// so the Rain, Weather warnings and Solar forecast widgets show data
// without the internet. The server is pointed
// here with GDASH_TEST_UPSTREAM (cmd/gdash/config.go); the paths are the
// ones cmd/gdash/warnings.go names. Open-Meteo's path answers 404: the
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

function answer(path: string, now: number): { type: string; body: string } | undefined {
  if (path === '/buienradar/data/raintext') return { type: 'text/plain', body: raintext(now) };
  if (path === '/meteoalarm/api/v1/warnings/feeds-netherlands') return { type: 'application/json', body: JSON.stringify(feed(now)) };
  if (path === '/open-meteo-solar/v1/forecast') return { type: 'application/json', body: JSON.stringify(solarAnswer(now)) };
  return undefined;
}

/** Starts the stand-in on a free port of the loopback address. */
export async function startUpstream(): Promise<{ url: string; close: () => Promise<void> }> {
  const solarQueries: string[] = [];
  const server: Server = createServer((req, res) => {
    const url = new URL(req.url ?? '/', 'http://upstream');
    if (url.pathname === '/open-meteo-solar/requests') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(solarQueries));
      return;
    }
    if (url.pathname === '/open-meteo-solar/v1/forecast') solarQueries.push(url.search);
    const found = answer(url.pathname, Date.now());
    if (!found) {
      res.writeHead(404).end();
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
