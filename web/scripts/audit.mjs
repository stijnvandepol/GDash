// Runs npm audit and fails on an advisory of high or critical severity,
// except the ones accepted below. npm audit has no such list of its own, and
// an advisory without a fixed version would otherwise keep CI red until the
// package's author releases one. Every entry says why it is accepted; remove
// it when a fixed version exists.
import { execFileSync } from 'node:child_process';

const ACCEPTED = {
  // picomatch <= 4.0.7, ReDoS on a crafted glob; no fixed version (checked
  // 18 November 2026). It only matches the globs of vite, vitest and
  // svelte-check at build time, which come from this repository, and is not
  // in the built web interface.
  'GHSA-fc5q-39xc-vjpf': 'picomatch',
};
const SEVERE = ['high', 'critical'];

let output;
try {
  output = execFileSync('npm', ['audit', '--json'], { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 });
} catch (err) {
  // npm audit exits non-zero when it finds anything; its report is on stdout.
  if (!err.stdout) throw err;
  output = err.stdout;
}

const report = JSON.parse(output);
if (!report.vulnerabilities) throw new Error(`npm audit gave no report: ${output.slice(0, 500)}`);

const found = new Map();
for (const vulnerability of Object.values(report.vulnerabilities)) {
  for (const via of vulnerability.via) {
    // A string names another package of this report, whose own advisories are listed there.
    if (typeof via === 'string' || !SEVERE.includes(via.severity)) continue;
    const id = via.url.split('/').pop();
    found.set(id, `${via.severity}: ${via.name} ${via.range}, ${via.title} (${via.url})`);
  }
}

const accepted = [...found.keys()].filter((id) => id in ACCEPTED);
const unaccepted = [...found.keys()].filter((id) => !(id in ACCEPTED));
for (const id of accepted) console.log(`accepted ${found.get(id)}`);
for (const id of Object.keys(ACCEPTED)) {
  if (!found.has(id)) console.log(`${id} (${ACCEPTED[id]}) is no longer reported; remove it from scripts/audit.mjs`);
}
for (const id of unaccepted) console.error(found.get(id));
if (unaccepted.length > 0) process.exit(1);
