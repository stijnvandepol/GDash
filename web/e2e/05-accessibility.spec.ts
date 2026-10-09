import AxeBuilder from '@axe-core/playwright';
import type { Page } from '@playwright/test';
import { ensureAccount, expect, openTab, test } from './fixtures';

const BLOCKING_IMPACTS = ['serious', 'critical'];

interface Finding {
  rule: string;
  html: string;
  detail: string;
}

async function blockingFindings(page: Page): Promise<Finding[]> {
  const { violations } = await new AxeBuilder({ page }).analyze();
  return violations
    .filter((v) => v.impact && BLOCKING_IMPACTS.includes(v.impact))
    .flatMap((v) =>
      v.nodes.map((n) => ({
        rule: v.id,
        html: n.html,
        detail: `${v.impact} ${v.id}: ${v.help}\n  ${n.target.join(' ')}\n  ${n.html}\n  ${(n.failureSummary ?? '').split('\n').join('\n  ')}`,
      })),
    );
}

/** Fails on any serious or critical violation, listing each one. */
async function expectNoBlockingViolations(page: Page): Promise<void> {
  const findings = (await blockingFindings(page)).map((f) => f.detail);
  expect(findings, findings.join('\n')).toEqual([]);
}

// Both colour schemes: the web interface follows the system's light or dark
// mode, and contrast has to hold in each.
for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`accessibility in ${colorScheme} mode`, () => {
    test.beforeEach(async ({ page }) => {
      await page.emulateMedia({ colorScheme });
    });

    test('the login page has no serious or critical violations', async ({ page, request }) => {
      await ensureAccount(request);
      await page.goto('/');
      await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible();
      await expectNoBlockingViolations(page);
    });

    for (const tab of ['Displays', 'Dashboards'] as const) {
      test(`the ${tab} page has no serious or critical violations`, async ({ adminPage: page }) => {
        await page.emulateMedia({ colorScheme });
        await openTab(page, tab);
        await expectNoBlockingViolations(page);
      });
    }

    test('every part of Settings has no serious or critical violations', async ({ adminPage: page }) => {
      await page.emulateMedia({ colorScheme });
      await openTab(page, 'Settings');
      for (const part of ['Data sources', 'Screens', 'Automations', 'Account']) {
        await page.getByRole('navigation', { name: 'Settings' }).getByRole('button', { name: part }).click();
        await expectNoBlockingViolations(page);
      }
    });
  });
}
