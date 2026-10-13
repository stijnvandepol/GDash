import { describe, expect, it } from 'vitest';
import type { CustomSource } from './api';
import {
  TOKEN_PLACEHOLDER,
  describeInterval,
  describeNamed,
  describePush,
  describeSize,
  hostOf,
  isPushLate,
  parseSample,
  pushExamples,
  pushInput,
  pushURL,
  sampleText,
  secretPlaceholder,
  suggestName,
  toInput,
  newLogin,
  withLogin,
} from './customSources';

const source: CustomSource = {
  id: 3,
  name: 'Prices',
  mode: 'fetch',
  url: 'https://api.example.com/v1/prices?currency=EUR',
  method: 'GET',
  body: '',
  auth: { kind: 'header', name: 'X-API-Key', secretSet: true },
  headers: [{ name: 'X-Tenant', value: '', secret: true }],
  intervalSeconds: 900,
  sample: { price: 1 },
  values: [],
  lists: [],
  preset: '',
  unreadable: false,
};

describe('toInput', () => {
  it('leaves every secret empty, so saving keeps the stored ones', () => {
    const input = toInput(source);
    expect(input.auth).toEqual({ kind: 'header', name: 'X-API-Key', secret: '' });
    expect(input.headers).toEqual([{ name: 'X-Tenant', value: '', secret: true }]);
    expect(input.headers[0]).not.toBe(source.headers[0]);
  });

  it('keeps the preset, as a PUT without it would make the source an own API', () => {
    expect(toInput({ ...source, preset: 'tibber' }).preset).toBe('tibber');
    expect(toInput(source).preset).toBe('');
  });
});

describe('secretPlaceholder', () => {
  it('only offers to keep a secret of the same kind', () => {
    expect(secretPlaceholder(source, 'header')).toContain('keep');
    expect(secretPlaceholder(source, 'query')).toBe('');
    expect(secretPlaceholder(undefined, 'header')).toBe('');
  });
});

describe('parseSample', () => {
  it('reads JSON and treats empty as no sample', () => {
    expect(parseSample(' {"a": [1]} ')).toEqual({ sample: { a: [1] } });
    expect(parseSample('  ')).toEqual({ sample: null });
    expect(parseSample('{"a":')).toHaveProperty('error');
    expect(parseSample(`"${'x'.repeat(50 * 1024)}"`)).toHaveProperty('error');
  });

  it('round-trips through the text in the form', () => {
    expect(parseSample(sampleText(source.sample))).toEqual({ sample: source.sample });
    expect(sampleText(null)).toBe('');
  });
});

describe('describeInterval and hostOf', () => {
  it('name the refresh and the host', () => {
    expect(describeInterval(900)).toBe('Every 15 minutes');
    expect(describeInterval(45)).toBe('Every 45 seconds');
    expect(hostOf(source.url)).toBe('api.example.com');
    expect(hostOf('not a url')).toBe('not a url');
    expect(describeSize(312)).toBe('312 bytes');
    expect(describeSize(14 * 1024 + 100)).toBe('14 KB');
  });
});

describe('suggestName', () => {
  it('names a value after its last key, in words', () => {
    expect(suggestName('current.temperature_2m', [])).toBe('Temperature 2m');
    expect(suggestName('main.feelsLike', [])).toBe('Feels like');
    expect(suggestName('weather[0]', [])).toBe('Weather 1');
    expect(suggestName('weather[0].description', [])).toBe('Description');
    expect(suggestName('$', [])).toBe('Item');
    expect(suggestName('["wind-speed"]', [])).toBe('Wind speed');
  });

  it('adds a number to a name that is taken, ignoring case', () => {
    expect(suggestName('a.temp', ['Temp'])).toBe('Temp 2');
    expect(suggestName('a.temp', ['temp', 'TEMP 2'])).toBe('Temp 3');
    expect(suggestName(`a.${'x'.repeat(60)}`, [`X${'x'.repeat(39)}`])).toHaveLength(40);
  });
});

describe('describeNamed', () => {
  it('counts the named values and lists', () => {
    expect(describeNamed({ values: [], lists: [] })).toBe('No values named yet');
    expect(describeNamed({ values: [{ name: 'A', path: 'a' }], lists: [] })).toBe('1 value');
    expect(describeNamed({ values: [{ name: 'A', path: 'a' }, { name: 'B', path: 'b' }], lists: [{ name: 'L', path: 'l', columns: [] }] })).toBe(
      '2 values · 1 list',
    );
  });
});

describe('pushing', () => {
  it('drops the request of a source that is sent to GDash', () => {
    const input = pushInput({ ...toInput(source), mode: 'push', body: '{}' });
    expect(input).toMatchObject({ mode: 'push', url: '', method: 'GET', body: '', headers: [] });
    expect(input.auth).toEqual({ kind: 'none', name: '', secret: '' });
    expect(input.sample).toEqual({ price: 1 });
  });

  it('fills the address and token into every example', () => {
    const url = pushURL('http://192.168.1.2:8420', 7);
    expect(url).toBe('http://192.168.1.2:8420/api/custom-sources/7/data');
    for (const example of Object.values(pushExamples(url, TOKEN_PLACEHOLDER))) {
      expect(example).toContain(url);
      expect(example).toContain(TOKEN_PLACEHOLDER);
    }
  });

  it('keeps the HomeyScript free of syntax an older Homey may not know', () => {
    // The script is the body of an async function on the Homey; it must at least parse.
    const AsyncFunction = Object.getPrototypeOf(async () => {}).constructor;
    expect(() => new AsyncFunction('Homey', 'fetch', pushExamples('http://gdash', 'token').homeyScript)).not.toThrow();
    expect(pushExamples('http://gdash', TOKEN_PLACEHOLDER).homeyScript).toContain("TOKEN === 'YOUR_API_TOKEN'");
    const { homeyScript } = pushExamples('http://gdash', 'token');
    expect(homeyScript).not.toContain('?.');
    expect(homeyScript).not.toContain('??');
    expect(homeyScript).toContain("method: 'PUT'");
  });

  it('marks data as late after two intervals and 30 seconds', () => {
    const at = '2026-10-13T08:00:00Z';
    const after = (seconds: number) => Date.parse(at) + seconds * 1000;
    expect(isPushLate(at, 60, after(150))).toBe(false);
    expect(isPushLate(at, 60, after(151))).toBe(true);
    expect(isPushLate(undefined, 60, after(1000))).toBe(false);
    expect(describePush(undefined, 60)).toBe('Nothing received yet.');
    expect(describePush(at, 60, after(60))).toBe('Last data received 1 minute ago.');
    expect(describePush(at, 60, after(240))).toContain('late');
  });
});

describe('logging in first', () => {
  const login = { url: 'http://pi.hole/api/auth', body: '{"password": "{secret}"}', tokenPath: 'session.sid', place: 'header' as const, validityPath: 'session.validity' };

  it('starts from the API address, filled in as Pi-hole 6 wants it', () => {
    expect(newLogin('http://pi.hole:8080/api/stats/summary')).toEqual({ ...login, url: 'http://pi.hole:8080/api/auth' });
    expect(newLogin('').url).toBe('');
  });

  it('copies the login of a stored source and never its password', () => {
    const stored: CustomSource = { ...source, auth: { kind: 'login', name: 'X-FTL-SID', secretSet: true, login } };
    const input = toInput(stored);
    expect(input.auth).toEqual({ kind: 'login', name: 'X-FTL-SID', secret: '', login });
    expect(input.auth.login).not.toBe(login);
  });

  it('sends a login only with its own kind, and no header name for a bearer token', () => {
    const base = toInput(source);
    expect(withLogin({ ...base, auth: { ...base.auth, login } }).auth).toEqual({ kind: 'header', name: 'X-API-Key', secret: '' });
    const bearer = withLogin({ ...base, auth: { kind: 'login', name: 'X-FTL-SID', secret: 'pw', login: { ...login, place: 'bearer' } } });
    expect(bearer.auth.name).toBe('');
    expect(bearer.auth.login?.place).toBe('bearer');
  });
});