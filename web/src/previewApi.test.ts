import { describe, expect, it } from 'vitest';
import { isPreviewMessage, previewOutcome, previewRequest, type PreviewDraft } from './previewApi';

const draft: PreviewDraft = { name: 'Hall', theme: 'glass', font: '', nightTheme: '', widgets: [{ id: 'clock', type: 'clock', x: 0, y: 0, w: 8, h: 4 }] };

const snapshot = {
  type: 'snapshot',
  version: 7,
  serverTime: '2026-10-08T17:00:00Z',
  dashboard: { id: 'hall', name: 'Hall', theme: 'glass', columns: 24, rows: 14, widgets: [] },
  data: {},
};

describe('isPreviewMessage', () => {
  it('accepts a text size with or without a draft', () => {
    expect(isPreviewMessage({ type: 'gdash.preview', textScale: 150, draft: null })).toBe(true);
    expect(isPreviewMessage({ type: 'gdash.preview', textScale: 100, draft })).toBe(true);
  });

  it('refuses anything else', () => {
    expect(isPreviewMessage(null)).toBe(false);
    expect(isPreviewMessage('gdash.preview')).toBe(false);
    expect(isPreviewMessage({ type: 'other', textScale: 100, draft: null })).toBe(false);
    expect(isPreviewMessage({ type: 'gdash.preview', textScale: '150', draft: null })).toBe(false);
    expect(isPreviewMessage({ type: 'gdash.preview', textScale: 100 })).toBe(false);
    expect(isPreviewMessage({ type: 'gdash.preview', textScale: 100, draft: { name: 'x' } })).toBe(false);
    expect(isPreviewMessage({ type: 'gdash.preview', textScale: 100, draft: { ...draft, nightTheme: undefined } })).toBe(false);
  });
});

describe('previewRequest', () => {
  it('reads the stored dashboard without a draft', () => {
    const { url, init } = previewRequest('hall', 125, null);
    expect(url).toBe('/api/dashboards/hall/preview?textScale=125');
    expect(init.method).toBeUndefined();
    expect(init.cache).toBe('no-store');
  });

  it('posts the draft, so nothing is saved', () => {
    const { url, init } = previewRequest('hall', 100, draft);
    expect(url).toBe('/api/dashboards/hall/preview?textScale=100');
    expect(init.method).toBe('POST');
    expect(JSON.parse(String(init.body))).toEqual(draft);
    expect(JSON.parse(String(previewRequest('hall', 100, { ...draft, nightTheme: 'night' }).init.body))).toMatchObject({ nightTheme: 'night' });
  });

  it('keeps an odd id inside its path segment', () => {
    expect(previewRequest('a/b?c', 100, null).url).toBe('/api/dashboards/a%2Fb%3Fc/preview?textScale=100');
  });
});

describe('previewOutcome', () => {
  it('passes a snapshot on', () => {
    expect(previewOutcome(200, snapshot)).toEqual({ kind: 'snapshot', snapshot });
  });

  it('does not trust a 200 that is not a snapshot', () => {
    expect(previewOutcome(200, { dashboard: {} })).toEqual({ kind: 'error' });
    expect(previewOutcome(200, null)).toEqual({ kind: 'error' });
  });

  it('tells a missing session, an unknown dashboard and other failures apart', () => {
    expect(previewOutcome(401, { error: { code: 'not_logged_in' } })).toEqual({ kind: 'login' });
    expect(previewOutcome(404, null)).toEqual({ kind: 'missing' });
    expect(previewOutcome(500, null)).toEqual({ kind: 'error' });
  });

  it('gives the first field error as the reason a layout is refused', () => {
    const body = { error: { code: 'invalid_input', message: 'Some fields are not valid.', fields: [{ field: 'widgets[1]', message: 'overlaps another widget' }] } };
    expect(previewOutcome(422, body)).toEqual({ kind: 'invalid', reason: 'widgets[1]: overlaps another widget' });
  });

  it('falls back to the message, or to nothing, when there are no field errors', () => {
    expect(previewOutcome(422, { error: { message: 'Not valid.' } })).toEqual({ kind: 'invalid', reason: 'Not valid.' });
    expect(previewOutcome(422, 'garbage')).toEqual({ kind: 'invalid', reason: '' });
  });
});
