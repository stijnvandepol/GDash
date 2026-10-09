import AxeBuilder from '@axe-core/playwright';
import type { Page } from '@playwright/test';
import { ensureAccount, expect, openTab, test } from './fixtures';

const BLOCKING_IMPACTS = ['serious', 'critical'];

interface Finding {
  rule: string;
  html: string;
  detail: string;
}

// Violations the suite found and the app does not fix yet. The footer links
// (#2f6fde on #f4f5f7) have a contrast of 4.32:1 and WCAG AA asks for 4.5:1.
// Both are on every page; the test fails when an entry stops matching, so
// remove it together with the fix.
const KNOWN_FINDINGS: Pick<Finding, 'rule' | 'html'>[] = [
  { rule: 'color-contrast', html: '<a href="https://www.gnu.org/licenses/agpl-3.0.html" rel="noopener">AGPL-3.0</a>' },
  { rule: 'color-contrast', html: '<a href="https://github.com/stijnvandepol/nookboard" rel="noopener">Source code</a>' },
];

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

const isKnown = (f: Pick<Finding, 'rule' | 'html'>) => KNOWN_FINDINGS.some((k) => k.rule === f.rule && k.html === f.html);

/** Fails on a serious or critical violation that is not a known one, and on a known one that is gone. */
async function expectNoNewBlockingViolations(page: Page): Promise<void> {
  const findings = await blockingFindings(page);
  const unexpected = findings.filter((f) => !isKnown(f)).map((f) => f.detail);
  expect(unexpected, unexpected.join('\n')).toEqual([]);

  const fixed = KNOWN_FINDINGS.filter((k) => !findings.some((f) => f.rule === k.rule && f.html === k.html)).map((k) => `${k.rule} ${k.html}`);
  expect(fixed, 'These known violations no longer occur: remove them from KNOWN_FINDINGS.').toEqual([]);
}

test.describe('accessibility', () => {
  test('the login page has no new serious or critical violations', async ({ page, request }) => {
    await ensureAccount(request);
    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible();
    await expectNoNewBlockingViolations(page);
  });

  for (const tab of ['Displays', 'Dashboards', 'Settings'] as const) {
    test(`the ${tab} page has no new serious or critical violations`, async ({ adminPage: page }) => {
      await openTab(page, tab);
      await expectNoNewBlockingViolations(page);
    });
  }
});
