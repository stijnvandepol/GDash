import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, openIntegration, test } from './fixtures';
import { garden, png } from './images';

// A stand-in for a camera on the LAN with a snapshot address behind basic
// auth, which can be switched off, and one for Immich with an album of two
// photos and a video.
const CAMERA_PASSWORD = 'e2e-camera-stand-in-password';
const IMMICH_KEY = 'e2e-immich-stand-in-key';
const ALBUM = '4b1c5d6e-7f80-4912-a3b4-c5d6e7f80912';
const PHOTOS = ['11111111-2222-4333-8444-555555555555', '66666666-7777-4888-9999-aaaaaaaaaaaa'];
const VIDEO = 'bbbbbbbb-cccc-4ddd-8eee-ffffffffffff';

let camera: Server;
let immich: Server;
let cameraUrl = '';
let immichUrl = '';
let cameraOff = false;

const listen = async (server: Server) => {
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
};

const close = async (server: Server) => {
  // GDash keeps its connections open between requests.
  server.closeAllConnections();
  await new Promise((resolve) => server.close(resolve));
};

test.beforeAll(async () => {
  const snapshot = garden();
  camera = createServer((req, res) => {
    const expected = `Basic ${Buffer.from(`admin:${CAMERA_PASSWORD}`).toString('base64')}`;
    if (req.headers.authorization !== expected) {
      res.writeHead(401).end();
    } else if (cameraOff) {
      res.writeHead(503).end();
    } else {
      // Many cameras send a generic type; GDash looks at the bytes.
      res.writeHead(200, { 'Content-Type': 'application/octet-stream' }).end(snapshot);
    }
  });
  const photo = png(160, 90, (x) => [200, 120 + x / 2, 80]);
  immich = createServer((req, res) => {
    const url = new URL(req.url ?? '/', 'http://immich');
    if (req.headers['x-api-key'] !== IMMICH_KEY) {
      res.writeHead(401).end();
    } else if (url.pathname === '/api/albums') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify([{ id: ALBUM, albumName: 'Holidays', assetCount: 3 }]));
    } else if (url.pathname === '/api/search/random' && req.method === 'POST') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify([...PHOTOS.map((id) => ({ id, type: 'IMAGE' })), { id: VIDEO, type: 'VIDEO' }]));
    } else if (PHOTOS.some((id) => url.pathname === `/api/assets/${id}/thumbnail`) && url.searchParams.get('size') === 'preview') {
      res.writeHead(200, { 'Content-Type': 'image/png' }).end(photo);
    } else {
      res.writeHead(404).end();
    }
  });
  cameraUrl = await listen(camera);
  immichUrl = await listen(immich);
});

test.afterAll(async () => {
  await close(camera);
  await close(immich);
});

test('a camera is added and tested under Integrations without showing its password again', async ({ adminPage: page }) => {
  const setup = await openIntegration(page, 'Camera snapshots');
  const section = setup.locator('section.card', { has: page.getByRole('heading', { name: 'Cameras', exact: true }) });
  await section.getByRole('button', { name: 'Add a camera' }).click();
  await section.getByLabel('Name', { exact: true }).fill('Front door');
  await section.getByRole('textbox', { name: /^Snapshot address/ }).fill(`${cameraUrl}/api/front_door/latest.jpg`);
  await section.getByRole('combobox', { name: /^Authentication/ }).selectOption('basic');
  await section.getByLabel('User name', { exact: true }).fill('admin');
  await section.getByRole('textbox', { name: /^Password/ }).fill(CAMERA_PASSWORD);
  await section.getByRole('button', { name: 'Save' }).click();
  await expect(section.getByText('Front door', { exact: true })).toBeVisible();

  await section.getByRole('button', { name: 'Test' }).click();
  const preview = section.getByRole('img', { name: 'The latest image of Front door' });
  await expect(preview).toBeVisible();
  await expect.poll(() => preview.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBe(320);
  const list = await page.request.get('/api/cameras');
  expect(await list.text()).not.toContain(CAMERA_PASSWORD);
});

test('an Immich album is connected under Integrations and chosen by name', async ({ adminPage: page }) => {
  const setup = await openIntegration(page, 'Immich');
  const section = setup.locator('section.card', { has: page.getByRole('heading', { name: 'Immich', exact: true }) });
  await section.getByLabel('Address').fill(immichUrl);
  await section.getByLabel('API key').fill('not-the-key');
  await section.getByRole('button', { name: 'Connect' }).click();
  await expect(section.getByText('Immich did not accept this API key')).toBeVisible();

  await section.getByLabel('API key').fill(IMMICH_KEY);
  await section.getByRole('button', { name: 'Connect' }).click();
  await section.getByRole('button', { name: 'Choose album' }).click();
  await section.getByLabel('Album').selectOption({ label: 'Holidays (3)' });
  await expect(section.getByText('Album: Holidays')).toBeVisible();
  const settings = await page.request.get('/api/settings');
  expect(await settings.text()).not.toContain(IMMICH_KEY);
});

test('a screen shows the camera, marks it stale when it fails, and shows the Immich album', async ({ adminPage: page }) => {
  test.setTimeout(60_000);
  const cameras = (await (await page.request.get('/api/cameras')).json()) as { cameras: { id: number; name: string }[] };
  const id = cameras.cameras.find((c) => c.name === 'Front door')?.id;
  const dashboard = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-camera',
      name: 'E2E Camera',
      widgets: [
        { id: 'door', type: 'camera', x: 0, y: 0, w: 14, h: 9, settings: { camera: id, seconds: 5, fit: 'cover' } },
        { id: 'album', type: 'photos', x: 14, y: 0, w: 10, h: 9, settings: { source: 'immich', seconds: 5 } },
      ],
    },
  });
  expect(dashboard.status(), await dashboard.text()).toBe(201);

  await page.goto('/preview/e2e-camera');
  const screen = page.locator('main.screen');
  const still = screen.locator('img[src^="blob:"]');
  await expect(still).toBeVisible({ timeout: 15_000 });
  expect(await still.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBe(320);
  await expect(screen.locator('.stale')).toHaveCount(0);

  const albumPhoto = screen.locator('img[src^="/api/immich/assets/"]');
  await expect(albumPhoto).toBeVisible({ timeout: 15_000 });
  await expect.poll(() => albumPhoto.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBe(160);
  // Neither the camera's nor Immich's address reaches the screen.
  const snapshot = await (await page.request.get('/api/dashboards/e2e-camera/preview')).text();
  expect(snapshot).not.toContain(cameraUrl);
  expect(snapshot).not.toContain(immichUrl);
  expect(snapshot).not.toContain(VIDEO);

  cameraOff = true;
  await expect(screen.locator('.stale')).toHaveText(/^\d{2}:\d{2}$/, { timeout: 20_000 });
  await expect(still).toBeVisible();
  cameraOff = false;
  await expect(screen.locator('.stale')).toHaveCount(0, { timeout: 20_000 });
});

test('a stranger on the network gets neither the camera image nor the album', async ({ playwright, baseURL }) => {
  const stranger = await playwright.request.newContext({ baseURL });
  expect((await stranger.get('/api/cameras/1/image')).status()).toBe(404);
  expect((await stranger.get(`/api/immich/assets/${PHOTOS[0]}`)).status()).toBe(404);
  await stranger.dispose();
});
