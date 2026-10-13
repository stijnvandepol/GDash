import { describe, expect, it } from 'vitest';
import { anwbStatusText, parseRoads } from './traffic';

describe('roads typed in the editor', () => {
  it('reads roads however they are typed', () => {
    expect(parseRoads('a2, A50 n279;A2')).toEqual({ roads: ['A2', 'A50', 'N279'], invalid: [] });
  });
  it('names what is not an A or N road', () => {
    expect(parseRoads('A2, E25, Ring')).toEqual({ roads: ['A2'], invalid: ['E25', 'RING'] });
    expect(parseRoads('  ')).toEqual({ roads: [], invalid: [] });
  });
});

describe('the ANWB status', () => {
  it('says what goes on', () => {
    expect(anwbStatusText({ state: 'idle' })).toMatch(/^Not asked yet/);
    expect(anwbStatusText({ state: 'ok' })).toMatch(/at most once every 3 minutes/);
    expect(anwbStatusText({ state: 'failing', httpStatus: 502, nextAttemptAt: '2026-10-13T08:46:00Z' })).toMatch(
      /^The last request failed \(HTTP 502\); screens keep the last traffic\. GDash asks again at \d\d:46\.$/,
    );
    expect(anwbStatusText({ state: 'stopped', httpStatus: 403 })).toBe('Stopped: the ANWB refused several requests in a row (HTTP 403).');
  });
});
