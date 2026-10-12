import { spawn } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { existsSync } from 'node:fs';
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { baseUrl, port } from './env';

const START_TIMEOUT_MS = 30_000;
const STOP_TIMEOUT_MS = 10_000;
const POLL_INTERVAL_MS = 100;
// A feed host that can never resolve, so the seeded news source needs no
// internet and the suite never calls a real server.
const OFFLINE_FEED = 'https://feeds.example.invalid/rss.xml';

const here = path.dirname(fileURLToPath(import.meta.url));

/** The server binary: GDASH_E2E_BINARY, else the one `go build` leaves in the repository root. */
function findBinary(): string {
  const candidates = process.env.GDASH_E2E_BINARY
    ? [path.resolve(process.env.GDASH_E2E_BINARY)]
    : [path.resolve(here, '../../gdash'), path.resolve(here, '../../gdash.exe')];
  const found = candidates.find((candidate) => existsSync(candidate));
  if (!found) {
    throw new Error(
      `No GDash binary found at ${candidates.join(' or ')}. Run "go build ./cmd/gdash" in the repository root, or set GDASH_E2E_BINARY.`,
    );
  }
  return found;
}

/** The caller's environment without its GDASH_* settings, so a developer's own setup never reaches the test server. */
function serverEnvironment(dataDir: string): NodeJS.ProcessEnv {
  const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('GDASH_')));
  return {
    ...env,
    GDASH_ADDR: `127.0.0.1:${port()}`,
    GDASH_DATA_DIR: dataDir,
    GDASH_NEWS_FEED: OFFLINE_FEED,
    // The suite never asks GitHub for the latest release; tests that show
    // the update notice answer /api/system themselves.
    GDASH_UPDATE_CHECK: 'off',
  };
}

async function waitUntilServing(): Promise<void> {
  const deadline = Date.now() + START_TIMEOUT_MS;
  for (;;) {
    try {
      if ((await fetch(`${baseUrl()}/api/setup`)).ok) return;
    } catch {
      // Not listening yet.
    }
    if (Date.now() > deadline) throw new Error(`The server did not answer on ${baseUrl()} within ${START_TIMEOUT_MS / 1000}s.`);
    await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
  }
}

/**
 * Starts the built server on a fresh data directory, hands the tests its
 * one-time setup token and a generated admin password, and returns the
 * function that stops it and removes the data afterwards.
 */
export default async function globalSetup(): Promise<() => Promise<void>> {
  const dataDir = await mkdtemp(path.join(tmpdir(), 'gdash-e2e-'));
  const server = spawn(findBinary(), [], { env: serverEnvironment(dataDir), stdio: ['ignore', 'ignore', 'pipe'] });

  let log = '';
  let exited = false;
  const token = new Promise<string>((resolve, reject) => {
    server.stderr.setEncoding('utf8');
    server.stderr.on('data', (chunk: string) => {
      log += chunk;
      const match = /setup_token="?([^"\s]+)/.exec(log);
      if (match?.[1]) resolve(match[1]);
    });
    server.on('error', reject);
    server.on('exit', (code, signal) => {
      exited = true;
      reject(new Error(`The server stopped (exit code ${code}, signal ${signal}). Its log:\n${log}`));
    });
  });
  // After a normal shutdown nobody is left waiting for the token.
  token.catch(() => undefined);

  const stop = async () => {
    if (!exited) {
      const gone = new Promise<void>((resolve) => server.once('exit', () => resolve()));
      server.kill();
      await Promise.race([gone, new Promise((resolve) => setTimeout(resolve, STOP_TIMEOUT_MS))]);
    }
    await mkdir('test-results', { recursive: true });
    await writeFile(path.join('test-results', 'server.log'), log);
    await rm(dataDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
  };

  try {
    const timeout = new Promise<never>((_, reject) =>
      setTimeout(() => reject(new Error(`No setup token in the server log within ${START_TIMEOUT_MS / 1000}s. Log:\n${log}`)), START_TIMEOUT_MS),
    );
    process.env.GDASH_E2E_SETUP_TOKEN = await Promise.race([token, timeout]);
    await waitUntilServing();
  } catch (err) {
    await stop();
    throw err;
  }
  // 24 hex characters: longer than the 10 the server asks for, and new every run.
  process.env.GDASH_E2E_PASSWORD = randomBytes(12).toString('hex');
  return stop;
}
