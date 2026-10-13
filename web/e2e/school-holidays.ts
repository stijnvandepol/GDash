import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';

// A stand-in for the government's school holiday feed
// (opendata.rijksoverheid.nl), in its shape, with dates around the day the
// suite runs: the server reads it through GDASH_SCHOOL_HOLIDAYS_URL, so no
// test reaches the internet and the dates never run out.

/** The days from today that the stand-in's holidays start and end. */
export const SCHOOL_HOLIDAYS = {
  // The north is on its autumn holiday, which ends within the week.
  north: { start: -3, end: 5 },
  // The middle and the south start theirs in four days.
  middle: { start: 4, end: 12 },
  christmas: { start: 60, end: 75 },
} as const;

function day(offset: number): string {
  const d = new Date();
  d.setDate(d.getDate() + offset);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

// The last day as a time at noon, so it is the same day in any time zone.
function region(name: string, span: { start: number; end: number }) {
  return { region: name, startdate: `${day(span.start)}T00:00:00.000Z`, enddate: `${day(span.end)}T12:00:00.000Z` };
}

// The feed's text fields carry the whitespace of the XML they came from.
function feed() {
  return [
    {
      id: 'e2e',
      type: 'schoolvakanties',
      content: [
        {
          title: 'Overzicht schoolvakanties',
          schoolyear: '\n   e2e \n ',
          vacations: [
            {
              type: '\n   Herfstvakantie \n ',
              compulsorydates: 'false',
              regions: [region('noord', SCHOOL_HOLIDAYS.north), region('midden', SCHOOL_HOLIDAYS.middle), region('zuid', SCHOOL_HOLIDAYS.middle)],
            },
            { type: '\n   Kerstvakantie \n ', compulsorydates: 'true', regions: [region('heel Nederland', SCHOOL_HOLIDAYS.christmas)] },
          ],
        },
      ],
      license: 'CC0 1.0 Universal',
    },
  ];
}

/** Starts the stand-in on a free loopback port; returns its address and how to stop it. */
export async function startSchoolHolidays(): Promise<{ url: string; close: () => Promise<void> }> {
  const server = createServer((req, res) => {
    if (req.method !== 'GET') {
      res.writeHead(405).end();
      return;
    }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(feed()));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}/v1/infotypes/schoolholidays?output=json`;
  const close = async () => {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  };
  return { url, close };
}
