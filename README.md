# GDash

Your day on every TV in the house. Self-hosted, casts to the Chromecast or
Google TV you already own. No extra hardware, no subscription, no cloud.

> **Status: beta (0.5).** It runs every day on a TV at home, but has not
> been used anywhere else yet. Run it only on a network you trust.

![GDash on a TV: Home Assistant values, clock, sunrise and sunset, air quality and weather in the Glass theme](docs/images/tv-glass.jpg)

## What works

- **Weather** from [Open-Meteo](https://open-meteo.com) (no API key): now, and
  a forecast for up to seven days, sunrise and sunset, the air quality and
  the pollen count (Europe). The last good value is kept when the
  internet drops, and stale data is marked.
- **Dutch or English on the TV**: days, dates, weather and other texts on the
  screens follow the household's choice; the web interface is in English.
- **Home Assistant**: show sensors and switches, such as the temperature
  inside or whether a door is open, with a long-lived access token. Read
  only; the token is stored encrypted. GDash works without Home
  Assistant too.
- **Announcements**: an automation, such as a Home Assistant rest_command,
  posts "The washing machine is done" with an API token and every TV, or the
  ones you choose, shows it over the dashboard for a while.
- **Notes and countdowns**: a short text for the household, and the days
  left until a date such as a holiday or a birthday.
- **Calendar** from secret iCal addresses (Google, Outlook, iCloud,
  Nextcloud): the coming week with recurrences, exceptions and moved
  occurrences, without OAuth. Only paired screens receive it.
- **News headlines** from up to five RSS or Atom feeds (default: NOS), merged
  newest first. Fetched once by the server for all TVs, with conditional
  requests; only plain-text titles reach the screen.
- **A web interface** at `/` to set up the first account, log in, manage
  displays and set the location, news feeds and calendars. Calendar addresses
  are stored encrypted and never shown again.
- **A dashboard editor**: drag and resize widgets on the TV's 16:9 grid, make
  several dashboards, pick one of six themes (from Minimal dark to High contrast and Night) and
  choose a dashboard per display. Saving updates the TVs within
  seconds. A display only receives the data its own widgets show.
- **Displays**: link a display to a Cast device found on your network, and
  GDash keeps the dashboard on that TV. It restarts the cast every 15
  minutes, because Google TV covers a running cast with its screensaver after
  about 19 minutes. No Home Assistant, no HTTPS and no Cast developer account
  needed.
- **Schedules**: show another dashboard during part of the day or week, such
  as the calendar and the school run on weekday mornings and a calm clock at
  night.
  Displays switch by themselves within a minute.
- **Active hours and burn-in protection**: outside a display's hours GDash
  stops casting and the page goes dark, so the TV can sleep. The dashboard
  shifts a few pixels every three minutes to protect OLED screens.
- **Browser displays**: any browser, kiosk or Raspberry Pi can open
  `/d/<display>` instead. The page shows a code; enter it in the web
  interface to pair the screen.
- **Paired screens only**: a dashboard only streams to a screen paired with
  that display. A TV opened by GDash pairs itself with a single-use token.
  Unpairing stops a screen within 20 seconds.

Progress and test results: [docs/progress.md](docs/progress.md). The research
behind the design (Dutch) is in
[GoogleHome-Dashboard/docs/onderzoek](https://github.com/stijnvandepol/GoogleHome-Dashboard/tree/main/docs/onderzoek).

## Run it

To install a release with Docker, as a Linux service or on Windows, see
[docs/install.md](docs/install.md); how to use it is in
[docs/guide.md](docs/guide.md). To build from source you need Go 1.26
and Node.js 24:

```bash
cd web && npm ci && npm run build && cd ..
go build -o gdash ./cmd/gdash
./gdash
```

On first start GDash logs a **setup token**. Open `http://<server>:8420/`,
enter the token and create your account. Under **Settings**, choose the
location for the weather and add news feeds and calendars. Then add a
display and pick your TV.

With Docker: run `docker compose up -d` and read the token with
`docker compose logs gdash`.

Back up the whole data directory: `secret.key` next to the database is
needed to read the stored calendar addresses.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `GDASH_ADDR` | `:8420` | Listen address |
| `GDASH_DATA_DIR` | `data` | Database, secret key and cached data |
| `TZ` | system | Time zone for "today" in the calendar, for example `Europe/Amsterdam` |
| `GDASH_CAST_APP_ID` | none | App ID of an own registered receiver ([docs/receiver.md](docs/receiver.md)); without it GDash uses DashCast |
| `GDASH_TRUSTED_PROXIES` | none | Addresses or CIDR ranges of reverse proxies, such as `172.17.0.1`; their `X-Forwarded-For` and `X-Forwarded-Proto` headers set the client address and secure cookies |

Location, news feeds and calendars are set in the web interface. For an
installation from before that existed, GDash copies `GDASH_LATITUDE`,
`GDASH_LONGITUDE`, `GDASH_NEWS_FEED` and `GDASH_CALENDAR_URLS` into the
settings on the first start and ignores them afterwards.

## Command line

```bash
gdash                    # start the server
gdash discover           # list Cast devices on the network
gdash status <address>   # show what a Cast device is doing, without changing it
gdash reset-password <user>  # give an account a new password, if you lost yours
gdash healthcheck        # exit 0 when the server answers, for Docker
gdash version
```

## Development

```bash
go test ./...                 # Go tests
go vet ./...
cd web && npm run check       # Svelte and TypeScript checks
cd web && npm run dev         # frontend with hot reload, proxies /api to :8420
```

The browser tests (Playwright, Chromium) run against the built server, so
build the web interface and the binary first. They start the binary on
`127.0.0.1:8435` with a temporary data directory and stop it afterwards; they
never touch your own data or any device on your network.

```bash
cd web && npm run build && cd .. && go build ./cmd/gdash
cd web && npx playwright install chromium    # once
cd web && npm run e2e                        # about 30 seconds
```

`GDASH_E2E_BINARY` points to another binary and `GDASH_E2E_PORT` to
another port. Screenshots and traces of failed tests, and the server's log, end
up in `web/test-results/`.

The code follows [CODE_GUIDELINES.md](CODE_GUIDELINES.md). The API is described
in [docs/api.md](docs/api.md).

## Security and changes

How to report a vulnerability and what GDash protects: [SECURITY.md](SECURITY.md).
What changed per version: [CHANGELOG.md](CHANGELOG.md).

## License

GDash is free software under the [GNU Affero General Public License
v3.0](LICENSE). You may run, change and share it; if you offer a changed
version to others over a network, you must offer them its source too. The
license keeps the dashboard from becoming a closed hosted service, the risk
the research names for this market.

The web interface links to the source, as section 13 of the license asks.
