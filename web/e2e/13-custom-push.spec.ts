import { expect, openIntegration, test } from './fixtures';

// A household's system, such as a HomeyScript, that sends its data to GDash
// instead of serving an API.
test('data that a system sends to GDash shows on a screen', async ({ adminPage: page, request, baseURL }) => {
  const setup = await openIntegration(page, 'Custom APIs');
  const card = setup.locator('section.card').filter({ has: page.getByRole('heading', { name: 'Custom APIs', exact: true }) });
  await card.getByRole('button', { name: 'Add a custom API' }).click();
  await card.getByLabel('Your system sends it to GDash').check();
  await expect(card.getByLabel('Address', { exact: true })).toHaveCount(0);
  await card.getByLabel('Name', { exact: true }).fill('Homey push');
  await card.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('Homey push is saved.')).toBeVisible();

  // The instructions open by themselves, with the address to send to.
  const instructions = card.locator('.push');
  await expect(instructions.getByRole('heading', { name: 'Send data to Homey push' })).toBeVisible();
  const address = await instructions.getByLabel('Address to send to').inputValue();
  expect(address).toMatch(new RegExp(`^${baseURL}/api/custom-sources/\\d+/data$`));
  await expect(instructions).toContainText('Nothing received yet.');
  await instructions.getByRole('button', { name: 'Make an API token' }).click();
  const token = await instructions.getByLabel('New API token').inputValue();
  expect(token).not.toBe('');
  await expect(instructions.getByLabel('HomeyScript (Homey Pro)')).toContainText(token);

  // The system sends with the token alone, no session.
  const sent = await request.put(address, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    data: { home: { Anna: true, Bram: false }, power: 1200 },
  });
  expect(sent.status(), await sent.text()).toBe(204);
  const again = await request.put(address, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    data: { home: { Anna: false } },
  });
  expect(again.status()).toBe(429);

  const row = card.getByRole('listitem').filter({ hasText: 'Homey push' }).first();
  await row.getByRole('button', { name: 'Last data received' }).click();
  await expect(card.getByText(/^The last data received, at/)).toBeVisible();
  await expect(card.getByRole('button', { name: /^power/ })).toBeVisible();
  await expect(row).toContainText('Last data received just now.');

  const id = Number(address.match(/custom-sources\/(\d+)\/data$/)![1]);
  const pairs = [
    { label: 'AN', path: 'home.Anna' },
    { label: 'BR', path: 'home.Bram' },
  ];
  const dashboard = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-push',
      name: 'E2E Push',
      widgets: [{ id: 'home', type: 'custom', x: 0, y: 0, w: 8, h: 5, source: 'custom.default', settings: { source: id, mode: 'badges', title: 'Home', pairs } }],
    },
  });
  expect(dashboard.status(), await dashboard.text()).toBe(201);
  await page.goto('/preview/e2e-push');
  await expect(page.locator('.badge')).toHaveText(['AN'], { timeout: 15_000 });
  await expect(page.locator('main.screen')).not.toContainText('BR');
});
