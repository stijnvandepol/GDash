import { describe, expect, it } from 'vitest';
import type { Features } from './compat';
import { buildReport, createErrorLog, plainText, rejectionMessage, scriptSource } from './screenReport';

const FEATURES: Features = {
  containerUnits: false,
  containerQueries: false,
  where: true,
  has: false,
  colorMix: false,
  aspectRatio: true,
  flexGap: true,
  inset: true,
  lineClamp: true,
  textWrap: false,
  nesting: false,
  cascadeLayers: true,
};

describe('plainText', () => {
  it('keeps one line without control characters', () => {
    expect(plainText('TypeError: boom\n    at f (App.js:1:2)\t\u001b[31m', 300)).toBe('TypeError: boom at f (App.js:1:2) [31m');
  });

  it('cuts long text with an ellipsis, by characters', () => {
    expect(plainText('ééééé', 3)).toBe('éé…');
    expect(plainText('abc', 3)).toBe('abc');
  });
});

describe('scriptSource', () => {
  it('keeps only the path, never the address with its query', () => {
    expect(scriptSource('http://192.168.1.2:8420/assets/App-x.js?launch=secret', 1, 200)).toBe('/assets/App-x.js:1:200');
  });

  it('is empty without a file or for something that is not an address', () => {
    expect(scriptSource('', 0, 0)).toBe('');
    expect(scriptSource('not a url', 1, 1)).toBe('');
  });
});

describe('createErrorLog', () => {
  it('keeps the first five errors, in one line each', () => {
    const log = createErrorLog();
    for (let i = 1; i <= 7; i++) log.add(`Error ${i}\nstack`, '/assets/App.js:1:1');
    expect(log.list().map((e) => e.message)).toEqual(['Error 1 stack', 'Error 2 stack', 'Error 3 stack', 'Error 4 stack', 'Error 5 stack']);
  });

  it('names an error without a message', () => {
    const log = createErrorLog();
    log.add('  ', '');
    expect(log.list()).toEqual([{ message: 'Unknown error', source: '' }]);
  });
});

describe('rejectionMessage', () => {
  it('names an error and anything else a promise was rejected with', () => {
    expect(rejectionMessage(new TypeError('x is undefined'))).toBe('TypeError: x is undefined');
    expect(rejectionMessage('offline')).toBe('Unhandled rejection: offline');
  });
});

describe('buildReport', () => {
  it('rounds the measurements and keeps them within the limits the server takes', () => {
    const report = buildReport({
      userAgent: 'Chrome/87 CrKey/1.56',
      width: 1280.4,
      height: 719.6,
      devicePixelRatio: 1.5,
      stageScale: 0.666666,
      features: FEATURES,
      compat: true,
      fontsLoaded: true,
      firstRenderMs: 2412.7,
      errors: [],
    });
    expect(report).toEqual({
      userAgent: 'Chrome/87 CrKey/1.56',
      viewport: { width: 1280, height: 720 },
      devicePixelRatio: 1.5,
      stageScale: 0.6667,
      features: FEATURES,
      compat: true,
      fontsLoaded: true,
      firstRenderMs: 2413,
      errors: [],
    });
  });

  it('keeps a missing render time and caps a very long one', () => {
    const base = { userAgent: '', width: 0, height: 0, devicePixelRatio: 0, stageScale: 1, features: FEATURES, compat: false, fontsLoaded: false, errors: [] };
    expect(buildReport({ ...base, firstRenderMs: null })).toMatchObject({ userAgent: 'unknown', viewport: { width: 1, height: 1 }, devicePixelRatio: 1, firstRenderMs: null });
    expect(buildReport({ ...base, firstRenderMs: 9e9 }).firstRenderMs).toBe(3_600_000);
  });
});
