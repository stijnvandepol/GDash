# Contributing to GDash

Thank you for wanting to help. GDash is a small project that one person
maintains in their spare time, so a few ground rules keep it pleasant for
everyone. By taking part you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Before you start

- **Small fixes** (a typo, a clear bug, a missing test, a doc correction): open
  a pull request straight away.
- **Anything bigger** (a new widget or data source, a change to the database,
  the API, the pairing or login flow, a new dependency): open an issue first
  and wait for an answer. A large pull request that nobody asked for is likely
  to be declined, and that is a waste of your time. GDash aims to stay small,
  boring and safe to run on a home network; "yes, but not in this form" is a
  normal answer.
- **Security problems** are not issues: see [SECURITY.md](SECURITY.md).
- **Questions** about using GDash are welcome as an issue too, once you have
  looked at [docs/install.md](docs/install.md), [docs/guide.md](docs/guide.md)
  and the troubleshooting section.

## Build and run

You need Go 1.26 (the version in `go.mod`) and Node.js 24.

```bash
cd web && npm ci && npm run build && cd ..   # the web interface is embedded in the binary
go build -o gdash ./cmd/gdash
./gdash                                      # reads the setup token from its log
```

`cd web && npm run dev` serves the web interface with hot reload and proxies
`/api` to a GDash on port 8420. The data goes to `data/` (set `GDASH_DATA_DIR`
for another folder); never point a development build at the data of the
GDash that runs your TV.

Cast devices are found with mDNS, which does not reach into Docker Desktop on
Windows or macOS; run the binary directly there. You do not need a TV to work
on GDash: a browser can be a display (`/d/<display>`, pair it with the code it
shows).

## Test

Run what CI runs before you open a pull request:

```bash
go test ./...                      # CI also uses -race, which needs cgo (a C compiler)
go vet ./...
gofmt -l .                         # prints nothing when formatted
cd web && npm run check            # Svelte and TypeScript, zero warnings allowed
cd web && npm test                 # unit tests (vitest)
```

CI also runs `staticcheck`, `govulncheck`, `npm audit` and a secret scan of the
whole history; the exact commands are in
[.github/workflows/ci.yml](.github/workflows/ci.yml) and in the "Project
specifics" section of [CODE_GUIDELINES.md](CODE_GUIDELINES.md).

### Browser tests

The Playwright tests start the built binary on `127.0.0.1:8435` with a
temporary data directory and click through the web interface in Chromium. They
never touch your own data or any device on your network. Build first:

```bash
cd web && npm run build && cd .. && go build ./cmd/gdash
cd web && npx playwright install chromium    # once
cd web && npm run e2e                        # about 30 seconds
```

`GDASH_E2E_BINARY` points to another binary (for example a Linux build you
copied somewhere) and `GDASH_E2E_PORT` to another port. Screenshots and traces
of failed tests, and the server's log, end up in `web/test-results/`.
The suite reaches no outside service: the global setup starts a stand-in for
the school holiday feed (`web/e2e/school-holidays.ts`) and points the server
at it with `GDASH_SCHOOL_HOLIDAYS_URL`, and the news feed it seeds is an
address that never resolves.

### The Home Assistant app

`homeassistant/` holds a Home Assistant app. CI lints it with the Home
Assistant app linter. If you change it, keep `version` in
`homeassistant/gdash/config.yaml` and `ARG GDASH_VERSION` in its Dockerfile
the same.

## Code

The rules are in [CODE_GUIDELINES.md](CODE_GUIDELINES.md), including the "Known
oddities" that look wrong but are on purpose. The short version:

- Read the surrounding code first and match it. Search before you add a second
  version of something.
- Build the smallest thing that solves the problem. Handle or return every
  error; validate everything that comes from outside.
- New behaviour comes with tests, and a bug fix with a test that fails without
  it. Never change a test, skip one or loosen a check to make CI pass.
- Code, comments, documentation and the web interface are in English. The texts
  that appear on the TV have English and Dutch versions.
- Standard library first. A new dependency needs a reason in the pull request;
  `scripts/third-party-licenses.sh` must be run and `THIRD_PARTY_LICENSES`
  committed with it, or CI fails.
- SQL lives only in `internal/store`. Migrations are never
  edited after they have been merged; add a new one.
- `web/dist` is build output; only `.gitkeep` is committed.

## Commits and pull requests

- One concern per commit and one purpose per pull request. A refactor and a
  behaviour change do not travel together.
- Commit messages follow what `git log` shows: a short lower-case summary with
  a type, such as `feat: ...`, `fix: ...`, `docs: ...`, `test(e2e): ...`,
  `build(deps): ...` or `refactor(web): ...`. The body says why; the diff
  already shows what.
- Never commit secrets, local configuration, generated files, or real personal
  data such as your home IP addresses, device names or calendar addresses. Use
  `192.168.1.10`-style examples.
- Update the documentation and the [CHANGELOG](CHANGELOG.md) (under
  "Unreleased") when behaviour that people see changes.
- Fill in the pull request template. Say how you tested it, and what could
  break.

Maintainers review every pull request and may ask for changes or decline it.
Releases are made by the maintainer: a version tag builds a draft release that
is published by hand.

## License

GDash is free software under the [GNU Affero General Public License
v3.0](LICENSE). By contributing you agree that your contribution is licensed
under the same license. There is no contributor license agreement.
