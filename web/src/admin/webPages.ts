import type { WebPage, WebPageCheck, WebPageInput } from './types';

/** internal/webpage allows this many pages. */
export const MAX_WEB_PAGES = 20;

export function emptyWebPage(): WebPageInput {
  return { name: '', url: '', refreshMinutes: 0, zoom: 100, allowScripts: true };
}

export function toWebPageInput(page: WebPage): WebPageInput {
  return { name: page.name, url: page.url, refreshMinutes: page.refreshMinutes, zoom: page.zoom, allowScripts: page.allowScripts };
}

/** How often a page reloads, for the list under Integrations. */
export function describeRefresh(minutes: number): string {
  if (minutes <= 0) return 'Never reloads';
  if (minutes === 1) return 'Reloads every minute';
  if (minutes % 60 === 0) return minutes === 60 ? 'Reloads every hour' : `Reloads every ${minutes / 60} hours`;
  return `Reloads every ${minutes} minutes`;
}

/** What a Check found, as one line for the admin: whether a TV can show the page, and how to fix it when not. */
export function describeCheck(check: WebPageCheck): { ok: boolean; message: string } {
  const header = check.header ? ` (${check.header})` : '';
  if (check.blocked === 'x-frame-options') {
    return {
      ok: false,
      message: `This page forbids being shown in a frame with X-Frame-Options${header}, so a TV shows an empty tile. ${FIXES}`,
    };
  }
  if (check.blocked === 'frame-ancestors') {
    return {
      ok: false,
      message: `This page only lets certain sites show it in a frame, and GDash is not one of them (Content-Security-Policy${header}). ${FIXES}`,
    };
  }
  if (check.status >= 400) {
    return {
      ok: false,
      message: `The page answered with status ${check.status}. A TV would show that answer; check the address, or whether the page needs a login.`,
    };
  }
  return { ok: true, message: 'This page may be shown in a frame. If it needs a login, the TV’s browser must be logged in too.' };
}

const FIXES =
  'For Home Assistant, add use_x_frame_options: false under http: in configuration.yaml; for Grafana, set allow_embedding = true. Other pages may have a share or embed link that allows it.';
