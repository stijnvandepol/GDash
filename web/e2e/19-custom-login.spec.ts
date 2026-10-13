import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, openIntegration, test } from './fixtures';

// A stand-in for Pi-hole 6: POST /api/auth with the password gives a
// session ID, which /api/stats/summary wants in the X-FTL-SID header.
const PASSWORD = 'e2e-pihole-stand-in-password';
const SID = 'e2e-stand-in-session-0123';

let piHole: Server;
let piHoleUrl = '';
let logins = 0;

test.beforeAll(async () => {
  piHole = createServer((req, res) => {
    let body = '';
    req.on('data', (chunk: Buffer) => (body += chunk.toString()));
    req.on('end', () => {
      res.setHeader('Content-Type', 'application/json');
      if (req.method === 'POST' && req.url === '/api/auth') {
        const sent = JSON.parse(body || '{}') as { password?: string };
        if (sent.password !== PASSWORD) {
          res.writeHead(401).end(JSON.stringify({ session: { valid: false, sid: null } }));
          return;
        }
        logins++;
        res.end(JSON.stringify({ session: { valid: true, sid: SID, validity: 1800 }, took: 0.001 }));
      } else if (req.url === '/api/stats/summary' && req.headers['x-ftl-sid'] === SID) {
        res.end(JSON.stringify({ queries: { total: 48213, blocked: 5120, percent_blocked: 10.6 } }));
      } else {
        res.writeHead(401).end(JSON.stringify({ error: { key: 'unauthorized' } }));
      }
    });
  });
  await new Promise<void>((resolve) => piHole.listen(0, '127.0.0.1', resolve));
  piHoleUrl = `http://127.0.0.1:${(piHole.address() as AddressInfo).port}`;
});

test.afterAll(async () => {
  piHole.closeAllConnections();
  await new Promise((resolve) => piHole.close(resolve));
});

test('a custom API that logs in first, such as Pi-hole, is set up under Integrations and shown on a screen', async ({ adminPage: page }) => {
  const setup = await openIntegration(page, 'Custom APIs');
  const card = setup.locator('section.card').filter({ has: page.getByRole('heading', { name: 'Custom APIs', exact: true }) });
  await card.getByRole('button', { name: 'Add a custom API' }).click();
  await card.getByLabel('Name', { exact: true }).fill('Pi-hole');
  await card.getByRole('textbox', { name: /^Address/ }).fill(`${piHoleUrl}/api/stats/summary`);
  await card.getByRole('combobox', { name: /^Authentication/ }).selectOption({ label: 'Log in first (a session, such as Pi-hole)' });
  // Filled in as Pi-hole 6 wants it, on the API's own host.
  await expect(card.getByRole('textbox', { name: /^Login address/ })).toHaveValue(`${piHoleUrl}/api/auth`);
  await expect(card.getByRole('textbox', { name: /^Login body/ })).toHaveValue('{"password": "{secret}"}');
  await expect(card.getByRole('textbox', { name: /^Token in the answer/ })).toHaveValue('session.sid');
  await expect(card.getByRole('textbox', { name: /^Header name/ })).toHaveValue('X-FTL-SID');

  // A login on another host is refused: the password only goes where the data comes from.
  await card.getByRole('textbox', { name: /^Login address/ }).fill('http://elsewhere.example/api/auth');
  await card.getByRole('textbox', { name: /^Password/ }).fill(PASSWORD);
  await card.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(card.getByText('use a login address on the same host as the API address')).toBeVisible();

  await card.getByRole('textbox', { name: /^Login address/ }).fill(`${piHoleUrl}/api/auth`);
  await card.getByRole('textbox', { name: /^Password/ }).fill(PASSWORD);
  await card.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('Pi-hole is saved.')).toBeVisible();

  // Neither the password nor the session ever reaches the browser.
  const listed = await (await page.request.get('/api/custom-sources')).text();
  expect(listed).toContain('"tokenPath":"session.sid"');
  expect(listed).not.toContain(PASSWORD);
  expect(listed).not.toContain(SID);
  const { sources } = JSON.parse(listed) as { sources: { id: number; name: string }[] };
  const id = sources.find((s) => s.name === 'Pi-hole')!.id;

  const tested = await page.request.post(`/api/custom-sources/${id}/tests`);
  expect(await tested.text()).toContain('48213');
  expect(await tested.text()).not.toContain(SID);

  const dashboard = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-pihole',
      name: 'E2E Pi-hole',
      widgets: [{ id: 'dns', type: 'custom', x: 0, y: 0, w: 6, h: 4, settings: { source: id, mode: 'value', title: 'Blocked', path: 'queries.blocked' } }],
    },
  });
  expect(dashboard.status(), await dashboard.text()).toBe(201);
  await page.goto('/preview/e2e-pihole');
  // 5,120 or 5.120, in the language of the TVs.
  await expect(page.locator('main.screen')).toContainText(/5[,.]120/, { timeout: 15_000 });
  // The test and the screen's fetch share one session.
  expect(logins).toBe(1);
});

test('the Pi-hole preset logs in with the password and names its values', async ({ adminPage: page }) => {
  const setup = await openIntegration(page, 'Pi-hole');
  await setup.getByRole('textbox', { name: /^Address of Pi-hole/ }).fill(piHoleUrl);
  await setup.getByRole('textbox', { name: /^Password/ }).fill(PASSWORD);
  // Test logs in and asks Pi-hole before anything is saved.
  await setup.getByRole('button', { name: 'Test', exact: true }).click();
  await expect(setup.getByText('Pi-hole answered.')).toBeVisible({ timeout: 15_000 });
  await expect(setup.getByRole('listitem').filter({ hasText: 'Blocked share' })).toContainText('10.6 %');
  await setup.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(setup.getByRole('list', { name: 'Pi-hole set up' })).toContainText('Connected');

  const listed = await (await page.request.get('/api/custom-sources')).text();
  expect(listed).not.toContain(PASSWORD);
  expect(listed).not.toContain(SID);
  const { sources } = JSON.parse(listed) as { sources: { url: string; auth: { kind: string; login?: { url: string } } }[] };
  const made = sources.filter((s) => s.auth.kind === 'login');
  expect(made.map((s) => s.auth.login?.url)).toContain(`${piHoleUrl}/api/auth`);
});
