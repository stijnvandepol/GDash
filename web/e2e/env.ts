// Values shared by the global setup, the config and the tests.

const DEFAULT_PORT = 8435;

export const ADMIN_USERNAME = 'e2e-admin';

/** The port of the test server, on the loopback address only. */
export function port(): number {
  return Number(process.env.GDASH_E2E_PORT ?? DEFAULT_PORT);
}

export function baseUrl(): string {
  return `http://127.0.0.1:${port()}`;
}

/** Set by the global setup: read in the workers, which start after it. */
function required(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is not set; the global setup has not run.`);
  return value;
}

export const setupToken = () => required('GDASH_E2E_SETUP_TOKEN');
export const adminPassword = () => required('GDASH_E2E_PASSWORD');
