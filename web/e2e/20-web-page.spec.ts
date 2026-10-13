import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, openIntegration, openTab, test } from './fixtures';

// A page on the LAN, such as a Grafana panel, stood in for by a server on
// another port: another origin than GDash's.
let standIn: Server;
let origin = '';

test.beforeAll(async () => {
  standIn = createServer((req, res) => {
    if (req.url === '/deny') res.setHeader('X-Frame-Options', 'DENY');
    res.setHeader('Content-Type', 'text/html; charset=utf-8');
    res.end('<!doctype html><title>Stand-in</title><body style="background:#e8f0ff;font:48px sans-serif"><h1>Stand-in panel</h1></body>');
  });
  await new Promise<void>((resolve) => standIn.listen(0, '127.0.0.1', resolve));
  origin = `http://127.0.0.1:${(standIn.address() as AddressInfo).port}`;
});

test.afterAll(async () => {
  await new Promise((resolve) => standIn.close(resolve));
});

test('a web page from Integrations shows in a sandboxed frame that only its origin may fill', async ({ adminPage: page }) => {
  const setup = await openIntegration(page, 'Web pages');
  const card = setup.locator('section.card').filter({ has: page.getByRole('heading', { name: 'Web pages', exact: true }) });

  async function addPage(name: string, url: string) {
    await card.getByRole('button', { name: 'Add a web page' }).click();
    await card.getByLabel('Name', { exact: true }).fill(name);
    await card.getByRole('textbox', { name: /^Address/ }).fill(url);
    await card.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.getByText(`${name} is saved.`)).toBeVisible();
  }

  // GDash's own pages are refused: a frame there could leave its sandbox.
  await card.getByRole('button', { name: 'Add a web page' }).click();
  await card.getByLabel('Name', { exact: true }).fill('Itself');
  await card.getByRole('textbox', { name: /^Address/ }).fill(new URL('/d/hall', page.url()).href);
  await card.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(card.getByText('a page of GDash itself cannot be shown in a frame')).toBeVisible();
  await card.getByRole('button', { name: 'Cancel' }).click();

  await addPage('Stand-in', `${origin}/panel?kiosk`);
  await addPage('Refuses frames', `${origin}/deny`);

  const standInRow = card.getByRole('listitem').filter({ hasText: 'Stand-in' }).first();
  await standInRow.getByRole('button', { name: 'Check' }).click();
  await expect(card.getByRole('status')).toHaveText(/may be shown in a frame/);
  const denyRow = card.getByRole('listitem').filter({ hasText: 'Refuses frames' }).first();
  await denyRow.getByRole('button', { name: 'Check' }).click();
  await expect(card.getByRole('status')).toContainText('X-Frame-Options (DENY)');
  await expect(card.getByRole('status')).toContainText('use_x_frame_options: false');

  // The editor picks a page by name; it has no field for an address.
  await openTab(page, 'Dashboards');
  await page.getByLabel('Name').fill('E2E Web');
  await page.getByRole('button', { name: 'Create and edit' }).click();
  const search = page.getByRole('complementary', { name: 'Add widgets' }).getByRole('searchbox', { name: 'Search widgets' });
  await search.fill('web page');
  await search.press('Enter');
  const settings = page.getByRole('complementary', { name: 'Widget settings' });
  await expect(settings.getByRole('heading', { name: 'Selected Web page' })).toBeVisible();
  await settings.getByRole('combobox', { name: /^Web page/ }).selectOption({ label: 'Stand-in' });
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();

  // The preview lists the pages' origins in frame-src, and nothing else.
  const response = await page.goto('/preview/e2e-web');
  const frameSrc = (response?.headers()['content-security-policy'] ?? '').split(';').map((d) => d.trim()).find((d) => d.startsWith('frame-src'));
  expect(frameSrc).toBe(`frame-src ${origin}`);
  const frame = page.locator('main.screen iframe');
  await expect(frame).toHaveAttribute('sandbox', 'allow-scripts allow-same-origin allow-forms', { timeout: 15_000 });
  await expect(frame).toHaveAttribute('referrerpolicy', 'no-referrer');
  await expect(frame).toHaveAttribute('allow', '');
  await expect(frame).toHaveAttribute('src', `${origin}/panel?kiosk`);
  await expect(page.frameLocator('main.screen iframe').getByRole('heading', { name: 'Stand-in panel' })).toBeVisible();

  // Someone without a session gets no origins at all.
  const anonymous = await page.context().browser()!.newContext();
  const anonymousResponse = await (await anonymous.newPage()).goto(new URL('/preview/e2e-web', page.url()).href);
  expect(anonymousResponse?.headers()['content-security-policy']).toContain("frame-src 'none'");
  await anonymous.close();
});
