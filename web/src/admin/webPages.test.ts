import { describe, expect, it } from 'vitest';
import { describeCheck, describeRefresh, emptyWebPage, toWebPageInput } from './webPages';

describe('describeRefresh', () => {
  it('words the interval', () => {
    expect(describeRefresh(0)).toBe('Never reloads');
    expect(describeRefresh(1)).toBe('Reloads every minute');
    expect(describeRefresh(15)).toBe('Reloads every 15 minutes');
    expect(describeRefresh(60)).toBe('Reloads every hour');
    expect(describeRefresh(1440)).toBe('Reloads every 24 hours');
    expect(describeRefresh(90)).toBe('Reloads every 90 minutes');
  });
});

describe('describeCheck', () => {
  it('passes a page that may be framed', () => {
    expect(describeCheck({ status: 200 }).ok).toBe(true);
  });

  it('names the header that forbids framing and how to fix it', () => {
    const deny = describeCheck({ status: 200, blocked: 'x-frame-options', header: 'DENY' });
    expect(deny.ok).toBe(false);
    expect(deny.message).toContain('X-Frame-Options (DENY)');
    expect(deny.message).toContain('use_x_frame_options: false');
    expect(deny.message).toContain('allow_embedding = true');
    const ancestors = describeCheck({ status: 200, blocked: 'frame-ancestors', header: "frame-ancestors 'self'" });
    expect(ancestors.ok).toBe(false);
    expect(ancestors.message).toContain("frame-ancestors 'self'");
  });

  it('warns about an error status', () => {
    const missing = describeCheck({ status: 404 });
    expect(missing.ok).toBe(false);
    expect(missing.message).toContain('404');
  });
});

describe('web page input', () => {
  it('starts with scripts allowed at 100 percent', () => {
    expect(emptyWebPage()).toEqual({ name: '', url: '', refreshMinutes: 0, zoom: 100, allowScripts: true });
  });

  it('copies a page without its id', () => {
    const page = { id: 3, name: 'Radar', url: 'https://example.com/', refreshMinutes: 10, zoom: 80, allowScripts: false };
    expect(toWebPageInput(page)).toEqual({ name: 'Radar', url: 'https://example.com/', refreshMinutes: 10, zoom: 80, allowScripts: false });
  });
});
