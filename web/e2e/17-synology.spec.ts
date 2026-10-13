import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, openIntegration, test } from './fixtures';
import { png } from './images';

// A stand-in for Synology Photos with one album shared by link with a
// password: a photo, a Live Photo and a video. It answers the DSM Web API
// calls the Photos page makes for a share link.
const PASSPHRASE = 'E2eShare42';
const PASSWORD = 'e2e-share-stand-in-password';
const SESSION = 'e2e-sharing-session';

let nas: Server;
let nasUrl = '';

test.beforeAll(async () => {
  const photo = png(200, 120, (x, y) => [60 + y, 90 + x / 4, 160]);
  nas = createServer((req, res) => {
    const url = new URL(req.url ?? '/', 'http://nas');
    let body = '';
    req.on('data', (chunk: Buffer) => (body += chunk.toString()));
    req.on('end', () => {
      if (url.pathname !== '/mo/sharing/webapi/entry.cgi') return void res.writeHead(404).end();
      const form = new URLSearchParams(req.method === 'POST' ? body : url.search);
      const loggedIn = (req.headers.cookie ?? '').includes(`sharing_sid=${SESSION}`);
      const json = (value: unknown, headers: Record<string, string> = {}) =>
        res.writeHead(200, { 'Content-Type': 'application/json', ...headers }).end(JSON.stringify(value));
      const thumbnail = (id: number, xl: string) => ({ cache_key: `${id}_1700000000`, unit_id: id, xl, m: 'ready' });
      switch (form.get('api')) {
        case 'SYNO.Core.Sharing.Login':
          if (form.get('sharing_id') !== PASSPHRASE || form.get('password') !== JSON.stringify(PASSWORD)) {
            return json({ success: false, error: { code: 105 } });
          }
          return json({ success: true, data: {} }, { 'Set-Cookie': `sharing_sid=${SESSION}; Path=/` });
        case 'SYNO.Foto.Browse.Item':
          if (!loggedIn || req.headers['x-syno-sharing'] !== PASSPHRASE) return json({ success: false, error: { code: 119 } });
          return json({
            success: true,
            data: {
              list: [
                { id: 21, type: 'photo', additional: { thumbnail: thumbnail(21, 'ready') } },
                { id: 22, type: 'live', additional: { thumbnail: thumbnail(22, 'broken') } },
                { id: 23, type: 'video', additional: { thumbnail: thumbnail(23, 'ready') } },
              ],
            },
          });
        case 'SYNO.Foto.Thumbnail':
          if (!loggedIn || form.get('passphrase') !== PASSPHRASE) return json({ success: false, error: { code: 119 } });
          return void res.writeHead(200, { 'Content-Type': 'image/png' }).end(photo);
        default:
          return json({ success: false, error: { code: 102 } });
      }
    });
  });
  await new Promise<void>((resolve) => nas.listen(0, '127.0.0.1', resolve));
  nasUrl = `http://127.0.0.1:${(nas.address() as AddressInfo).port}`;
});

test.afterAll(async () => {
  nas.closeAllConnections();
  await new Promise((resolve) => nas.close(resolve));
});

test('a Synology Photos album is connected by its share link and password', async ({ adminPage: page }) => {
  const setup = await openIntegration(page, 'Synology Photos');
  const section = setup.locator('section.card', { has: page.getByRole('heading', { name: 'Synology Photos', exact: true }) });
  await section.getByRole('textbox', { name: /^Share link/ }).fill(`${nasUrl}/mo/sharing/${PASSPHRASE}`);
  await section.getByRole('textbox', { name: /^Password of the link/ }).fill('not-the-password');
  await section.getByRole('button', { name: 'Connect' }).click();
  await expect(section.getByText('Synology did not open the album')).toBeVisible();

  await section.getByRole('textbox', { name: /^Password of the link/ }).fill(PASSWORD);
  await section.getByRole('button', { name: 'Connect' }).click();
  await expect(page.getByText('GDash can show 2 photos')).toBeVisible();
  await expect(section.getByText(/^Shared album on 127\.0\.0\.1:\d+$/)).toBeVisible();
  const settings = await (await page.request.get('/api/settings')).text();
  expect(settings).not.toContain(PASSPHRASE);
  expect(settings).not.toContain(PASSWORD);
});

test('a screen shows the photos of the shared album through GDash', async ({ adminPage: page }) => {
  const dashboard = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-synology',
      name: 'E2E Synology',
      widgets: [{ id: 'album', type: 'photos', x: 0, y: 0, w: 12, h: 8, settings: { source: 'synology', seconds: 5 } }],
    },
  });
  expect(dashboard.status(), await dashboard.text()).toBe(201);

  await page.goto('/preview/e2e-synology');
  const photo = page.locator('main.screen img[src^="/api/synology/photos/"]');
  await expect(photo).toBeVisible({ timeout: 15_000 });
  await expect.poll(() => photo.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBe(200);
  const snapshot = await (await page.request.get('/api/dashboards/e2e-synology/preview')).text();
  expect(snapshot).not.toContain(nasUrl);
  expect(snapshot).not.toContain(PASSPHRASE);
  // The video is not in the album GDash shows.
  expect(snapshot).not.toContain('23-23_');

  const stranger = await page.context().browser()!.newContext({ baseURL: page.url() });
  const token = /\/api\/synology\/photos\/([^"]+)/.exec((await photo.getAttribute('src')) ?? '')?.[1];
  expect((await stranger.request.get(`/api/synology/photos/${token}`)).status()).toBe(404);
  await stranger.close();
});
