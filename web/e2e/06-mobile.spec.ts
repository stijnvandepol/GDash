import type { Page } from '@playwright/test';
import { ensureAccount, expect, openIntegration, openTab, test } from './fixtures';

// A phone in portrait.
test.use({ viewport: { width: 375, height: 812 } });

async function expectNoHorizontalScroll(page: Page): Promise<void> {
  const { scrollWidth, innerWidth, sticking } = await page.evaluate(() => {
    const width = window.innerWidth;
    // The innermost elements that reach past the window, to name the culprit.
    const out = [...document.querySelectorAll('body *')].filter((e) => {
      const r = e.getBoundingClientRect();
      return r.width > 0 && r.right > width && ![...e.children].some((c) => c.getBoundingClientRect().right > width);
    });
    return {
      scrollWidth: document.documentElement.scrollWidth,
      innerWidth: width,
      sticking: out.slice(0, 5).map((e) => `${e.tagName.toLowerCase()}.${[...e.classList].join('.')} "${(e.textContent ?? '').trim().slice(0, 40)}"`),
    };
  });
  expect(
    scrollWidth,
    `the page is ${scrollWidth}px wide in a ${innerWidth}px window; reaching past it: ${sticking.join(', ') || 'none found'}`,
  ).toBeLessThanOrEqual(innerWidth);
}

test.describe('mobile width', () => {
  test('the login page fits', async ({ page, request }) => {
    await ensureAccount(request);
    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible();
    await expectNoHorizontalScroll(page);
  });

  for (const tab of ['Displays', 'Dashboards', 'Integrations', 'Settings'] as const) {
    test(`the ${tab} page fits`, async ({ adminPage: page }) => {
      await openTab(page, tab);
      await expectNoHorizontalScroll(page);
    });
  }

  test('an open setup on the Integrations page fits', async ({ adminPage: page }) => {
    for (const name of ['Custom APIs', 'HomeWizard P1 meter', 'Homey']) {
      await openIntegration(page, name);
      await expectNoHorizontalScroll(page);
    }
  });

  test('the dashboard editor fits', async ({ adminPage: page }) => {
    await openTab(page, 'Dashboards');
    await page.getByRole('button', { name: 'Edit', exact: true }).first().click();
    await expect(page.getByLabel('Dashboard name')).toBeVisible();
    await expectNoHorizontalScroll(page);
  });
});
