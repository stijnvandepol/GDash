import { describe, expect, it } from 'vitest';
import type { Entry } from '../protocol';
import { FRAME_RELOAD_GAP_MS, frameScale, pageFrame, shouldReloadForFrames } from './webpage';

const OWN = 'http://192.168.1.2:8420';

function entry(pages: Record<string, unknown>): Entry {
  return { value: pages, fetchedAt: '', staleAfter: '', failing: false };
}

const grafana = { name: 'Grafana', url: 'http://192.168.1.10:3000/d/abc?kiosk', refreshMinutes: 15, zoom: 80, allowScripts: true };

describe('pageFrame', () => {
  it('frames the page of the widget with scripts in a sandbox', () => {
    expect(pageFrame(entry({ '1': grafana }), { page: 1 }, OWN)).toEqual({
      name: 'Grafana',
      src: 'http://192.168.1.10:3000/d/abc?kiosk',
      sandbox: 'allow-scripts allow-same-origin allow-forms',
      zoom: 80,
      reloadMs: 15 * 60_000,
    });
  });

  it('sandboxes a page without scripts completely and never reloads without an interval', () => {
    const frame = pageFrame(entry({ '2': { ...grafana, allowScripts: false, refreshMinutes: 0 } }), { page: 2 }, OWN);
    expect(frame?.sandbox).toBe('');
    expect(frame?.reloadMs).toBe(0);
  });

  it('never allows top navigation, popups, modals or downloads', () => {
    const sandbox = pageFrame(entry({ '1': grafana }), { page: 1 }, OWN)!.sandbox;
    for (const token of ['allow-top-navigation', 'allow-popups', 'allow-modals', 'allow-downloads']) {
      expect(sandbox).not.toContain(token);
    }
  });

  it('takes the widget’s zoom over the page’s, within 50 to 200', () => {
    expect(pageFrame(entry({ '1': grafana }), { page: 1, zoom: 150 }, OWN)?.zoom).toBe(150);
    expect(pageFrame(entry({ '1': grafana }), { page: 1, zoom: 500 }, OWN)?.zoom).toBe(80);
    expect(pageFrame(entry({ '1': { ...grafana, zoom: 10 } }), { page: 1 }, OWN)?.zoom).toBe(100);
  });

  it('shows nothing for a missing page or one that is not http or https', () => {
    expect(pageFrame(undefined, { page: 1 }, OWN)).toBeNull();
    expect(pageFrame(entry({ '1': grafana }), { page: 2 }, OWN)).toBeNull();
    expect(pageFrame(entry({ '1': grafana }), { page: '1' }, OWN)).toBeNull();
    expect(pageFrame(entry({ '1': grafana }), undefined, OWN)).toBeNull();
    for (const url of ['javascript:alert(1)', 'data:text/html,hi', 'file:///etc/passwd', 'not a url']) {
      expect(pageFrame(entry({ '1': { ...grafana, url } }), { page: 1 }, OWN)).toBeNull();
    }
  });

  it('refuses a page on the screen’s own origin', () => {
    expect(pageFrame(entry({ '1': { ...grafana, url: `${OWN}/d/hall` } }), { page: 1 }, OWN)).toBeNull();
    expect(pageFrame(entry({ '1': { ...grafana, url: 'http://192.168.1.2:3000/' } }), { page: 1 }, OWN)).not.toBeNull();
  });
});

describe('frameScale', () => {
  it('lays a zoomed-out page out larger and draws it smaller', () => {
    expect(frameScale(50)).toEqual({ size: '200%', scale: 0.5 });
    expect(frameScale(100)).toEqual({ size: '100%', scale: 1 });
    expect(frameScale(200)).toEqual({ size: '50%', scale: 2 });
  });
});

describe('shouldReloadForFrames', () => {
  it('reloads once, then not again within the gap', () => {
    expect(shouldReloadForFrames(null, 1_000)).toBe(true);
    expect(shouldReloadForFrames(1_000, 1_000 + FRAME_RELOAD_GAP_MS - 1)).toBe(false);
    expect(shouldReloadForFrames(1_000, 1_000 + FRAME_RELOAD_GAP_MS)).toBe(true);
  });

  it('reloads after the clock went back', () => {
    expect(shouldReloadForFrames(5_000, 1_000)).toBe(true);
  });
});
