import type { Page } from '@playwright/test';
import { expect, openTab, test } from './fixtures';

// axe only sees buttons at rest. A button whose own colours do not cover
// the hover state inherits the global one (a dark fill) and can lose its
// text; this happened twice. Each visible button is hovered and its text
// measured against the background it then has.

/** The buttons on the page whose text is below 4.5:1 while hovered. */
async function lowContrastOnHover(page: Page): Promise<string[]> {
  const found: string[] = [];
  for (const button of await page.locator('button:visible').all()) {
    try {
      await button.hover({ timeout: 1000 });
    } catch {
      continue; // Covered by something else, such as the tab bar.
    }
    const result = await button.evaluate((el) => {
      const parse = (css: string) => {
        const [r = 0, g = 0, b = 0, a = 1] = (css.match(/[\d.]+/g) ?? []).map(Number);
        const lin = (x: number) => ((x /= 255) <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4);
        return { lum: 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b), alpha: a };
      };
      // The nearest background that is not (nearly) transparent.
      let node: Element | null = el;
      let background = 'rgb(255, 255, 255)';
      while (node) {
        const css = getComputedStyle(node).backgroundColor;
        if (parse(css).alpha > 0.5) {
          background = css;
          break;
        }
        node = node.parentElement;
      }
      const fg = parse(getComputedStyle(el).color).lum;
      const bg = parse(background).lum;
      const text = (el as HTMLElement).innerText.trim();
      return { text, ratio: (Math.max(fg, bg) + 0.05) / (Math.min(fg, bg) + 0.05) };
    });
    if (result.text && result.ratio < 4.5) found.push(`"${result.text}" ${result.ratio.toFixed(2)}:1`);
  }
  return found;
}

for (const colorScheme of ['light', 'dark'] as const) {
  test(`buttons keep their contrast on hover in ${colorScheme} mode`, async ({ adminPage: page }) => {
    // No transitions, so the colours are measured at their end.
    await page.emulateMedia({ colorScheme, reducedMotion: 'reduce' });
    const found: string[] = [];

    for (const tab of ['Displays', 'Dashboards'] as const) {
      await openTab(page, tab);
      found.push(...(await lowContrastOnHover(page)));
    }
    await page.getByRole('button', { name: 'Edit', exact: true }).first().click();
    found.push(...(await lowContrastOnHover(page)));

    await openTab(page, 'Integrations');
    found.push(...(await lowContrastOnHover(page)));

    await openTab(page, 'Settings');
    for (const part of ['Screens', 'Account']) {
      await page.getByRole('navigation', { name: 'Settings' }).getByRole('button', { name: part }).click();
      found.push(...(await lowContrastOnHover(page)));
    }
    expect(found, found.join('\n')).toEqual([]);
  });
}
