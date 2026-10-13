import { expect, openIntegration, test } from './fixtures';

// A Homey Pro (Early 2019) has no API keys: its HomeyScript sends a
// snapshot of every device and who is home to GDash instead.
const SNAPSHOT = {
  at: '2026-10-13T08:00:00.000Z',
  devices: [
    { id: 'p1', name: 'Boiler', zone: 'Attic', available: true, caps: { measure_temperature: { title: 'Water', value: 58.5, units: '°C' } } },
    { id: 'p2', name: 'Garage door', zone: 'Garage', available: true, caps: { alarm_contact: { title: 'Contact', value: false, units: null } } },
  ],
  users: [
    { id: 'u-eva', name: 'Eva Jansen', present: true, asleep: false },
    { id: 'u-finn', name: 'Finn', present: false, asleep: false },
  ],
};

test('a Homey that sends its data shows on a screen', async ({ adminPage: page, request, baseURL }) => {
  const setup = await openIntegration(page, 'Homey');
  const section = setup.locator('section.card', { has: page.getByRole('heading', { name: 'Homey', exact: true }) });
  // An earlier spec may have connected a Homey with a key.
  const change = section.getByRole('button', { name: 'Change', exact: true });
  if (await change.isVisible()) await change.click();
  await section.getByLabel('Homey Pro (2016–2019): your Homey sends its data (HomeyScript)').check();
  await expect(section.getByRole('textbox', { name: 'Address' })).toHaveCount(0);
  await section.getByRole('button', { name: 'Use HomeyScript' }).click();
  await expect(section.getByText('Your Homey sends its data')).toBeVisible();
  await expect(section).toContainText('Nothing received yet.');

  // The steps open by themselves, with the script to paste.
  await expect(section.getByRole('heading', { name: 'Let your Homey send its data' })).toBeVisible();
  await section.getByRole('button', { name: 'Make an API token' }).click();
  const token = await section.getByLabel('New API token').inputValue();
  const script = section.getByLabel('HomeyScript', { exact: true });
  await expect(script).toContainText(`${baseURL}/api/homey/snapshot`);
  await expect(script).toContainText(token);

  const sent = await request.put('/api/homey/snapshot', {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    data: SNAPSHOT,
  });
  expect(sent.status(), await sent.text()).toBe(204);
  await expect(section).toContainText('Last data received just now.', { timeout: 10_000 });

  // The editor's pickers list what the Homey sent.
  const capabilities = await page.request.get('/api/homey/capabilities');
  expect(await capabilities.text()).toContain('"key":"p1/measure_temperature","zone":"Attic"');

  const dashboard = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-homey-push',
      name: 'E2E Homey push',
      widgets: [
        { id: 'values', type: 'homey', x: 0, y: 0, w: 8, h: 5, settings: { capabilities: ['p1/measure_temperature'] } },
        {
          id: 'home',
          type: 'homeypresence',
          x: 8,
          y: 0,
          w: 8,
          h: 5,
          settings: {
            users: [
              { id: 'u-eva', label: 'EJ' },
              { id: 'u-finn', label: 'FI' },
            ],
          },
        },
      ],
    },
  });
  expect(dashboard.status(), await dashboard.text()).toBe(201);

  await page.goto('/preview/e2e-homey-push');
  const screen = page.locator('main.screen');
  await expect(screen.locator('.badge')).toHaveText(['EJ'], { timeout: 15_000 });
  await expect(screen).toContainText('Boiler');
  await expect(screen).toContainText(/58[.,]5/);
  // Neither the garage door nor who is away reaches this screen.
  const preview = await (await page.request.get('/api/dashboards/e2e-homey-push/preview')).text();
  for (const hidden of ['Garage door', 'u-finn', 'Finn']) expect(preview).not.toContain(hidden);
});
