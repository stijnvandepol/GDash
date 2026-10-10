import { test as base, expect, type APIRequestContext, type Page } from '@playwright/test';
import { ADMIN_USERNAME, adminPassword, setupToken } from './env';

type Tab = 'Displays' | 'Dashboards' | 'Settings';

/** Creates the administrator account when the server has none yet. */
export async function ensureAccount(request: APIRequestContext): Promise<void> {
  const state = await request.get('/api/setup');
  expect(state.ok()).toBe(true);
  if (!((await state.json()) as { needed: boolean }).needed) return;
  const created = await request.post('/api/setup', {
    data: { setupToken: setupToken(), username: ADMIN_USERNAME, password: adminPassword() },
  });
  expect(created.status()).toBe(201);
}

export const test = base.extend<{ adminPage: Page }>({
  // Every context answers the Cast search itself: the real one sends
  // multicast queries onto the network the tests run on, and takes seconds.
  context: async ({ context }, use) => {
    await context.route('**/api/cast-devices', (route) => route.fulfill({ json: { devices: [] } }));
    await use(context);
  },

  // A page with an administrator session, opened on the web interface.
  adminPage: async ({ page, context, request }, use) => {
    await ensureAccount(request);
    const login = await context.request.post('/api/sessions', {
      data: { username: ADMIN_USERNAME, password: adminPassword() },
    });
    expect(login.status()).toBe(201);
    await page.goto('/');
    await expect(tabButton(page, 'Displays')).toBeVisible();
    await use(page);
  },
});

export { expect };

function tabButton(page: Page, tab: Tab) {
  return page.getByRole('navigation').getByRole('button', { name: tab, exact: true });
}

/** Opens a tab of the web interface and waits for its first heading. */
export async function openTab(page: Page, tab: Tab): Promise<void> {
  await tabButton(page, tab).click();
  const heading = tab === 'Settings' ? 'Weather location' : tab;
  await expect(page.getByRole('heading', { name: heading, exact: true })).toBeVisible();
}
