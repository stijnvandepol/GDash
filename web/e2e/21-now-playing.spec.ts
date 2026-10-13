import { crc32, deflateSync } from 'node:zlib';
import type { Page, Route } from '@playwright/test';
import { expect, openTab, test } from './fixtures';

// What plays comes from Cast devices on the network. The suite never lets
// the server look for them: no dashboard with a Now playing widget is
// saved, and the snapshots that the preview page shows are answered here
// with the data hub entry in them.

const KITCHEN = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const GROUP = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
const BEDROOM = 'dddddddddddddddddddddddddddddddd';
const TV = 'cccccccccccccccccccccccccccccccc';
const COVER = '/api/now-playing/artwork/0123456789abcdef0123456789abcdef';
const COVER_PNG = coverPng(64);

/** A square PNG with a diagonal gradient, to stand in for an album cover. */
function coverPng(size: number): Buffer {
  const chunk = (type: string, data: Buffer) => {
    const body = Buffer.concat([Buffer.from(type, 'ascii'), data]);
    const length = Buffer.alloc(4);
    length.writeUInt32BE(data.length);
    const crc = Buffer.alloc(4);
    crc.writeUInt32BE(crc32(body));
    return Buffer.concat([length, body, crc]);
  };
  const header = Buffer.alloc(13);
  header.writeUInt32BE(size, 0);
  header.writeUInt32BE(size, 4);
  header.set([8, 2, 0, 0, 0], 8);
  const rows: number[] = [];
  for (let y = 0; y < size; y++) {
    rows.push(0);
    for (let x = 0; x < size; x++) rows.push(220 - x * 2, 60 + y * 2, 120 + (x + y));
  }
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', header),
    chunk('IDAT', deflateSync(Buffer.from(rows))),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

/** The Now playing entry: a group plays a song, a speaker is paused. */
function nowPlaying(at: Date) {
  return {
    [GROUP]: {
      name: 'Whole house',
      group: true,
      app: 'Spotify',
      title: 'One More Time',
      artist: 'Daft Punk',
      album: 'Discovery',
      image: COVER,
      state: 'playing',
      position: 63,
      duration: 320,
      at: at.toISOString(),
    },
    [BEDROOM]: {
      name: 'Bedroom',
      app: 'YouTube Music',
      title: 'Clair de Lune',
      artist: 'Claude Debussy',
      state: 'paused',
      position: 12,
      duration: 300,
      at: new Date(at.getTime() - 60_000).toISOString(),
    },
  };
}

/** Answers the preview of a stored dashboard with a Now playing widget and value. */
async function showNowPlaying(page: Page, dashboardId: string, value: () => unknown, size = { w: 12, h: 7 }): Promise<void> {
  await page.route(`**/api/dashboards/${dashboardId}/preview*`, async (route: Route) => {
    const response = await route.fetch();
    const snapshot = await response.json();
    snapshot.dashboard.widgets = [{ id: 'music', type: 'nowplaying', x: 0, y: 0, ...size, source: 'nowplaying.default' }];
    const now = new Date().toISOString();
    snapshot.data['nowplaying.default'] = { value: value(), fetchedAt: now, staleAfter: now, failing: false };
    snapshot.version = Date.now();
    // An earlier spec may have chosen another display language.
    snapshot.language = 'en';
    await route.fulfill({ response, json: snapshot });
  });
  await page.route('**/api/now-playing/artwork/*', (route) => route.fulfill({ contentType: 'image/png', body: COVER_PNG }));
}

test('a screen shows what plays on the Cast speakers', async ({ adminPage: page }) => {
  const made = await page.request.post('/api/dashboards', {
    data: { id: 'e2e-now-playing', name: 'E2E Now playing', widgets: [{ id: 'clock', type: 'clock', x: 0, y: 0, w: 4, h: 3 }] },
  });
  expect(made.status(), await made.text()).toBe(201);
  let value: unknown = nowPlaying(new Date());
  await showNowPlaying(page, 'e2e-now-playing', () => value);

  await page.goto('/preview/e2e-now-playing');
  const widget = page.locator('main.screen section.widget');
  await expect(widget.locator('.title')).toHaveText('One More Time', { timeout: 15_000 });
  await expect(widget.locator('.source')).toContainText('Spotify on Whole house');
  await expect(widget.locator('.artist')).toHaveText('Daft Punk · Discovery');
  await expect(widget.locator('.others li')).toHaveText([/Bedroom\s*Clair de Lune · Claude Debussy/]);
  await expect(widget.locator('.others li.paused')).toHaveCount(1);
  // The cover loads from GDash, and the time moves on by itself.
  await expect(widget.locator('.art img')).toHaveAttribute('src', COVER);
  await expect.poll(() => widget.locator('.art img').evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);
  const first = await widget.locator('.times span').first().textContent();
  await expect(widget.locator('.times span').first()).not.toHaveText(first ?? '', { timeout: 5_000 });

  // When everything stops, the widget says so calmly.
  value = {};
  await expect(widget.locator('.nothing')).toHaveText('Nothing playing', { timeout: 15_000 });
});

test('covers are not served for what does not play', async ({ adminPage: page }) => {
  const response = await page.request.get(COVER);
  expect(response.status()).toBe(404);
});

test('the editor adds a Now playing widget that follows chosen speakers, never a display TV', async ({ adminPage: page }) => {
  await page.route('**/api/cast-devices', (route) =>
    route.fulfill({
      json: {
        devices: [
          { id: KITCHEN, name: 'Kitchen speaker', model: 'Google Nest Mini', addr: '192.0.2.10:8009', hasScreen: false },
          { id: GROUP, name: 'Whole house', model: 'Google Cast Group', addr: '192.0.2.10:32187', hasScreen: false },
          { id: TV, name: 'Living room TV', model: 'Chromecast', addr: '192.0.2.20:8009', hasScreen: true },
        ],
      },
    }),
  );
  await page.route('**/api/displays', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    const response = await route.fetch();
    const body = (await response.json()) as { displays: unknown[] };
    body.displays.push({
      id: 'e2e-living-room', name: 'E2E Living room', castDeviceId: TV, createdAt: '2026-10-12T08:00:00Z', pairedScreens: 0,
      dashboardId: '', activeFrom: '', activeUntil: '', schedule: [], rotation: [], rotateSeconds: 0, textScale: 100,
    });
    await route.fulfill({ response, json: body });
  });
  const previews: string[] = [];
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().includes('/api/dashboards/e2e-music/preview')) previews.push(request.postData() ?? '');
  });

  await openTab(page, 'Dashboards');
  await page.getByLabel('Name').fill('E2E Music');
  await page.getByRole('button', { name: 'Create and edit' }).click();
  const palette = page.getByRole('complementary', { name: 'Add widgets' });
  await palette.getByRole('searchbox', { name: 'Search widgets' }).fill('speakers');
  await expect(palette.locator('button.item')).toHaveCount(1);
  await palette.getByRole('button', { name: 'Now playing', exact: true }).click();

  const settings = page.getByRole('complementary', { name: 'Widget settings' });
  await expect(settings.getByRole('heading', { name: 'Selected Now playing' })).toBeVisible();
  await expect(settings.getByLabel('Any device that is playing')).toBeChecked();
  await settings.getByLabel('Only these devices').check();
  await expect(settings.getByText('Tick at least one device, or choose any device.')).toBeVisible();
  await expect(settings.getByRole('checkbox', { name: /Living room TV/ })).toBeDisabled();
  await expect(settings.getByText('Shows the dashboard of E2E Living room')).toBeVisible();
  await settings.getByRole('checkbox', { name: /Kitchen speaker/ }).check();
  await expect(settings.getByText('Tick at least one device, or choose any device.')).toHaveCount(0);

  // The preview shows the unsaved widget, which follows the kitchen speaker.
  await page.getByRole('button', { name: 'Preview', exact: true }).click();
  await expect.poll(() => previews.some((body) => body.includes(`"settings":{"devices":["${KITCHEN}"]}`))).toBe(true);
  // An unsaved widget has nothing to show yet: it waits, in the household's language.
  await expect(page.frameLocator('iframe').locator('main.screen .now > .muted')).toBeVisible();
});
