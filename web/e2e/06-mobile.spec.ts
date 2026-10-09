import type { Page } from '@playwright/test';
import { ensureAccount, expect, openTab, test } from './fixtures';

// A phone in portrait.
test.use({ viewport: { width: 375, height: 812 } });

async function expectNoHorizontalScroll(page: Page): Promise<void> {
  const { scrollWidth, innerWidth } = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    innerWidth: window.innerWidth,
  }));
  expect(scrollWidth, `the page is ${scrollWidth}px wide in a ${innerWidth}px window`).toBeLessThanOrEqual(innerWidth);
}

test.describe('mobile width', () => {
  test('the login page fits', async ({ page, request }) => {
    await ensureAccount(request);
    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible();
    await expectNoHorizontalScroll(page);
  });

  for (const tab of ['Displays', 'Dashboards', 'Settings'] as const) {
    test(`the ${tab} page fits`, async ({ adminPage: page }) => {
      await openTab(page, tab);
      await expectNoHorizontalScroll(page);
    });
  }

  test('the dashboard editor fits', async ({ adminPage: page }) => {
    await openTab(page, 'Dashboards');
    await page.getByRole('button', { name: 'Edit' }).first().click();
    await expect(page.getByLabel('Dashboard name')).toBeVisible();
    await expectNoHorizontalScroll(page);
  });
});
