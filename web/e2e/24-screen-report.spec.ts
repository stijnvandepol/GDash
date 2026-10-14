import type { Page } from '@playwright/test';
import { expect, openTab, test } from './fixtures';

// A screen tells the server about its own browser once it has drawn the
// dashboard; the Displays page shows that under "What the screen reports".

const DISPLAY = 'e2e-report';

interface Listed {
  displays: { id: string; screenReports?: { reason: string; visibility: { state: string }[] }[] }[];
}

async function reportsOf(page: Page): Promise<{ reason: string; visibility: { state: string }[] }[]> {
  const list = (await (await page.request.get('/api/displays')).json()) as Listed;
  return list.displays.find((d) => d.id === DISPLAY)?.screenReports ?? [];
}

test('a paired TV reports its browser, and the Displays page shows it', async ({ adminPage: page, browser, baseURL }) => {
  const created = await page.request.post('/api/displays', { data: { id: DISPLAY, name: 'E2E Report' } });
  expect(created.status()).toBe(201);

  // The kitchen TV: a Chromecast HD, here with the fallback for older Cast runtimes on.
  const tv = await browser.newContext({ baseURL, viewport: { width: 1280, height: 720 }, deviceScaleFactor: 1.5 });
  try {
    const asked = await tv.request.post('/api/pairing-requests', { data: { displayId: DISPLAY } });
    expect(asked.status()).toBe(201);
    const { code, pollToken } = (await asked.json()) as { code: string; pollToken: string };
    expect((await page.request.post(`/api/displays/${DISPLAY}/pairings`, { data: { code } })).status()).toBe(201);
    expect((await tv.request.post('/api/display-sessions', { data: { pollToken } })).status()).toBe(201);

    const screen = await tv.newPage();
    await screen.goto(`/d/${DISPLAY}?compat=1`);
    await expect(screen.locator('section.widget').first()).toBeVisible({ timeout: 15_000 });

    await expect.poll(async () => (await reportsOf(page)).map((r) => r.reason), { timeout: 20_000 }).toEqual(['connected']);

    // A screensaver covers the page: it says so at once.
    await screen.evaluate(() => {
      Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' });
      document.dispatchEvent(new Event('visibilitychange'));
    });
    await expect.poll(async () => (await reportsOf(page)).map((r) => r.reason)).toEqual(['hidden']);
    expect((await reportsOf(page))[0]!.visibility.map((e) => e.state)).toContain('hidden');
  } finally {
    await tv.close();
  }

  await page.reload();
  await openTab(page, 'Displays');
  const row = page.getByRole('listitem').filter({ hasText: 'E2E Report' });
  await row.getByText('What the screen reports').click();
  const report = row.getByRole('region', { name: 'Report of a paired screen' });
  await expect(report).toContainText(/Chrome \d+/);
  await expect(report).toContainText('1280 × 720 at 1.5× · the dashboard drawn at 67%');
  await expect(report).toContainText('The fallback for older browsers is on (?compat=1 in the address)');
  await expect(report).toContainText(/Visibility: .*hidden \d\d:\d\d:\d\d/);
});

test('the Displays page explains what an older TV browser lacks', async ({ adminPage: page }) => {
  const oldTv = {
    screen: 'cast',
    receivedAt: new Date().toISOString(),
    userAgent:
      'Mozilla/5.0 (Linux; Android 12.0; Build/STTL.240206.002) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 CrKey/1.56.500000 DeviceType/AndroidTV',
    viewport: { width: 1280, height: 720 },
    devicePixelRatio: 1.5,
    stageScale: 0.6667,
    features: {
      containerUnits: false,
      containerQueries: false,
      where: false,
      has: false,
      colorMix: false,
      aspectRatio: false,
      flexGap: true,
      inset: true,
      lineClamp: true,
      textWrap: false,
      nesting: false,
      cascadeLayers: false,
    },
    compat: true,
    fontsLoaded: true,
    firstRenderMs: 3100,
    errors: [{ message: 'TypeError: e.replaceAll is not a function', source: '/assets/colors-abc.js:1:2048' }],
    reason: 'connected',
    visibility: [],
  };
  await page.route('**/api/cast-devices', (route) =>
    route.fulfill({ json: { devices: [{ id: 'e2e-device', name: 'Tv Keuken', model: 'Chromecast', addr: '192.0.2.1:8009', hasScreen: true }] } }),
  );
  await page.route('**/api/displays', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    const response = await route.fetch();
    const body = (await response.json()) as { displays: unknown[] };
    const kitchen = {
      id: 'e2e-old-tv',
      name: 'E2E Old TV',
      castDeviceId: 'e2e-device',
      createdAt: '2026-10-12T10:00:00Z',
      dashboardId: '',
      activeFrom: '',
      activeUntil: '',
      schedule: [],
      rotation: [],
      rotateSeconds: 0,
      textScale: 100,
      screensaverMinutes: 15,
      pairedScreens: 0,
      cast: {
        state: 'showing',
        since: '2026-10-12T10:00:00Z',
        restartedAt: '2026-10-14T09:52:00Z',
        restartReason: 'hidden',
        hiddenAt: '2026-10-14T09:51:55Z',
      },
      screenReports: [oldTv],
    };
    await route.fulfill({ response, json: { displays: [...body.displays, kitchen] } });
  });
  const changes: unknown[] = [];
  await page.route('**/api/displays/e2e-old-tv', async (route) => {
    if (route.request().method() !== 'PATCH') return route.fallback();
    changes.push(route.request().postDataJSON());
    await route.fulfill({ json: {} });
  });
  await page.reload();
  await openTab(page, 'Displays');

  const row = page.getByRole('listitem').filter({ hasText: 'E2E Old TV' });
  await row.getByText('What the screen reports').click();
  const report = row.getByRole('region', { name: 'Report of Tv Keuken' });
  await expect(report).toContainText('Chrome 87 (Cast)');
  await expect(report).toContainText('does not support container units and container queries (Chrome 105 and later); GDash uses its fallback');
  await expect(report).toContainText("does not support :where() (Chrome 88 and later)");
  await expect(report).toContainText('TypeError: e.replaceAll is not a function');
  await expect(report).toContainText('/assets/colors-abc.js:1:2048');

  // Whether the restarts against the screensaver happen, and how often.
  await expect(row).toContainText(/Restarted against the screensaver at \d\d:\d\d, because the screen was hidden/);
  await expect(row).toContainText(/The screen was last hidden at \d\d:\d\d/);
  const every = row.getByLabel('Keep E2E Old TV in front of the screensaver');
  await expect(every).toHaveValue('15');
  await every.selectOption({ label: 'Every 5 minutes' });
  await expect.poll(() => changes).toEqual([{ screensaverMinutes: 5 }]);
});
