# GDash

**Your household's day on every TV in the house.** GDash is a self-hosted
dashboard for the Chromecast, Google TV, Nest Hub or any browser you already
own: the time, the weather, the family calendar, the next bus, the waste
collection, Home Assistant sensors and more, on a calm screen that runs all
day. No extra hardware at the TV, no subscription, no cloud account.

> **Status: beta (0.6).** It runs every day on a TV and a Nest Hub at home,
> but has not been used anywhere else yet. Run it on a network you trust.

![GDash on a TV: Home Assistant values, clock, sunrise and sunset, air quality and weather in the Glass theme](docs/images/tv-glass.jpg)

## Contents

- [Why GDash](#why-gdash)
- [Features](#features)
- [Quick start](#quick-start)
- [Screens it works with](#screens-it-works-with)
- [Configuration](#configuration)
- [Command line](#command-line)
- [Updating and backups](#updating-and-backups)
- [Documentation](#documentation)
- [Development](#development)
- [Security, license and credits](#security-license-and-credits)

## Why GDash

Most home dashboards are built for a phone or a tablet in your hand. A TV in
the kitchen or the hallway is different: it is read from across the room, it
must stay on screen for hours without anyone touching it, and it has to
survive the internet dropping out. GDash is made for exactly that.

- **It uses the TVs you have.** GDash finds Chromecast and Google TV devices
  on your network and keeps the dashboard on them, even past Google TV's
  screensaver. Any other screen opens a web page instead.
- **It runs on one small box.** A single program with an embedded database,
  for a NAS, a Raspberry Pi, a mini PC or a Home Assistant server.
- **It keeps your data at home.** Calendar addresses and tokens are stored
  encrypted, every screen only receives what its own dashboard shows, and
  nothing is sent to a GDash cloud, because there is none.

## Features

### On the screen

| Widget | Shows | Source |
|---|---|---|
| Clock | Time and date | — |
| Weather and forecast | Now and up to seven days | [Open-Meteo](https://open-meteo.com), no key |
| Sun | Sunrise, sunset and the length of the day | Open-Meteo |
| Air quality | Air quality index and pollen (Europe) | Open-Meteo |
| Calendar | The coming week from Google, Outlook, iCloud or Nextcloud | Secret iCal address, no OAuth |
| Waste collection | The next pickup per kind (GFT, PMD, paper, ...) | Your municipality's iCal link |
| Departures | Next buses, trams, metros and ferries at a stop, with delays | [OVapi](https://v0.ovapi.nl) (Netherlands), no key |
| Fuel prices | The cheapest petrol stations near a place | ANWB price data (Netherlands), no key |
| Energy price | Today's dynamic electricity price per hour, cheapest window | [EnergyZero](https://www.energyzero.nl) (Netherlands), no key |
| Home Assistant | Sensors and switches, such as the temperature or a door | Your Home Assistant, read only |
| News | Headlines from up to five RSS or Atom feeds | Any feed (default: NOS) |
| Photos | A slideshow of photos you upload | Stored on your server |
| Note and countdown | A short text, or the days until a date | — |

The last good value of every source is kept when the internet drops, and a
screen marks data that has gone stale.

### Looks

- **Twenty-one styles** in three groups: calm essentials (Minimal dark and
  light, Glass, Warm light, High contrast, Night), styles with a character of
  their own (Frost, Walnut, Swiss, Tonal, Candy, Gilded, Riviera) and styles
  inspired by films and series (Galaxy, Terminal, Parchment, Neon, Upside
  down, Noir, Mars).
- **Eleven typefaces** that ship with GDash, chosen per dashboard.
- **A night style** that takes over after sunset at your location.
- Texts on the TV in **English, Dutch, German or French**.
- Text sizes from 100 to 200 % per display, and every text colour at 4.5:1
  contrast or more in every style.

![Eight of the twenty-one styles on the same dashboard: Frost, Walnut, Swiss, Gilded, Galaxy, Terminal, Noir and Candy](docs/images/styles.jpg)

### In the web interface

- A **dashboard editor** on the TV's own 16:9 grid: drag, resize, preview with
  your real data before saving. Start from a template or from scratch, and
  export or import dashboards as files.
- **Displays** with their own dashboard, **schedules** (another dashboard on
  weekday mornings or at night), dashboards **in turn**, and **active hours**
  outside which the TV may sleep.
- **Announcements**: a Home Assistant automation posts "The washing machine is
  done" with an API token, and the TVs show it over the dashboard.
- **Two-step login** with an authenticator app, a **backup** download, and an
  **activity log** of logins, pairings and changes.

![The web interface: your dashboards and ready-made templates, each shown in its own theme](docs/images/web-dashboards.jpg)

## Quick start

GDash needs a machine that is always on and on the same network as your TVs.
The full guide, including a Linux service, Windows and the Home Assistant app,
is in [docs/install.md](docs/install.md).

**With Docker Compose** (amd64 and arm64, so also a Raspberry Pi 3, 4 or 5):

```yaml
services:
  gdash:
    image: ghcr.io/stijnvandepol/gdash:latest
    network_mode: host          # needed to find Cast devices with mDNS
    environment:
      TZ: "Europe/Amsterdam"
    volumes:
      - gdash-data:/data
    restart: unless-stopped
    user: "65532:65532"
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
volumes:
  gdash-data:
```

```bash
docker compose up -d
docker compose logs gdash      # shows the one-time setup token
```

**Without Docker:** download the archive for your system from the
[releases](https://github.com/stijnvandepol/gdash/releases), unpack it and run
`gdash` (or `gdash.exe` on Windows).

**Then, in the browser:**

1. Open `http://<your-server>:8420/` and create your account with the setup
   token from the log.
2. Under **Settings**, choose the location for the weather and add the
   calendars, news feeds or Home Assistant you want.
3. Under **Dashboards**, start from a template such as Kitchen or Hallway, or
   build your own.
4. Under **Displays**, add a display and pick your TV. GDash puts the
   dashboard on it within seconds. For any other screen, open the display's
   page in a browser and pair it with the code it shows.

## Screens it works with

| Screen | How | Tested |
|---|---|---|
| Chromecast with Google TV, Google TV sets | GDash casts and keeps the dashboard in front of the screensaver | 24 hours on a Chromecast HD |
| Nest Hub | The same, as a Cast device | 8 hours, and an hour of style changes |
| Any browser, kiosk, Fire TV, tablet or Raspberry Pi | Opens `/d/<display>` and pairs with a code | Chromium (in the browser tests) |

GDash restarts the cast every 15 minutes, because Google TV covers a running
cast with its screensaver after about 19 minutes; it waits when another app
such as YouTube is on screen. The dashboard shifts a few pixels every three
minutes to protect OLED screens.

## Configuration

Almost everything is set in the web interface. The server itself takes a few
environment variables:

| Variable | Default | Meaning |
|---|---|---|
| `GDASH_ADDR` | `:8420` | Listen address |
| `GDASH_DATA_DIR` | `data` | Database, encryption key, photos and cached data |
| `TZ` | system | Time zone for "today", for example `Europe/Amsterdam` |
| `GDASH_CAST_APP_ID` | none | App ID of your own registered Cast receiver ([docs/receiver.md](docs/receiver.md)); without it GDash uses the public DashCast receiver |
| `GDASH_TRUSTED_PROXIES` | none | Addresses or CIDR ranges of reverse proxies whose `X-Forwarded-For` and `X-Forwarded-Proto` GDash trusts |

The network needs: TCP 8009 from GDash to the TVs, mDNS (UDP 5353) to find
them, TCP 8420 from the TVs to GDash, and HTTPS from GDash to the internet for
the data sources. See [docs/install.md](docs/install.md#what-the-network-needs).

## Command line

```bash
gdash                          # start the server
gdash discover                 # list the Cast devices on the network
gdash status <address>         # show what a Cast device is doing, without changing it
gdash reset-password <user>    # set a new password (also turns two-step login off)
gdash healthcheck              # exit 0 when the server answers, for Docker
gdash version
```

## Updating and backups

Replace the program or pull the new image and start it again. Before a
database upgrade GDash writes a copy of the database next to it, and it makes a
daily copy in `backups/` that keeps the last seven.

For a backup to keep elsewhere, use **Settings → Account → Backup**: one zip
with the database, the encryption key and the photos. It holds the key to
your calendar addresses and tokens, so keep it like a password. Restoring is
unzipping it into an empty data directory. See
[docs/install.md](docs/install.md#backups).

## Documentation

| Document | For |
|---|---|
| [docs/install.md](docs/install.md) | Installing with Docker, as a Linux service, on Windows or as a Home Assistant app; network, permissions, backups |
| [docs/guide.md](docs/guide.md) | Using GDash: displays, dashboards, styles, widgets, schedules, announcements |
| [docs/api.md](docs/api.md) | The HTTP API, for automations and other clients |
| [docs/receiver.md](docs/receiver.md) | Registering your own Cast receiver |
| [homeassistant/gdash/DOCS.md](homeassistant/gdash/DOCS.md) | The Home Assistant app (experimental) |
| [SECURITY.md](SECURITY.md) | What GDash protects and how to report a vulnerability |
| [CHANGELOG.md](CHANGELOG.md) | What changed per version |
| [docs/progress.md](docs/progress.md) | What has been built and tested, on which devices |

## Development

You need Go 1.26 and Node.js 24.

```bash
cd web && npm ci && npm run build && cd ..   # build the web interface
go build -o gdash ./cmd/gdash                # build the server, with the web interface inside
./gdash
```

```bash
go test ./...               # Go tests
cd web && npm run check     # Svelte and TypeScript checks
cd web && npx vitest run    # web unit tests
cd web && npm run dev       # web interface with hot reload, proxies /api to :8420
```

The browser tests (Playwright) start the built server on `127.0.0.1:8435`
with a temporary data directory, so they never touch your own data or devices:

```bash
cd web && npx playwright install chromium   # once
cd web && npm run e2e
```

Contributions are welcome; read [CONTRIBUTING.md](CONTRIBUTING.md) and
[CODE_GUIDELINES.md](CODE_GUIDELINES.md) first. The design research behind
GDash (Dutch) is in
[GoogleHome-Dashboard/docs/onderzoek](https://github.com/stijnvandepol/GoogleHome-Dashboard/tree/main/docs/onderzoek).

## Security, license and credits

**Security.** Passwords follow NIST SP 800-63B and are hashed with argon2id,
two-step login is optional, secrets are encrypted at rest, every screen is
paired and only receives its own data, and the web interface has a strict
Content Security Policy. Report a vulnerability as described in
[SECURITY.md](SECURITY.md).

**License.** GDash is free software under the [GNU Affero General Public
License v3.0](LICENSE): you may run, change and share it, and if you offer a
changed version to others over a network, you offer them its source too. The
web interface links to the source, as section 13 of the license asks.

**Credits.** Weather and air quality by [Open-Meteo](https://open-meteo.com),
departures by [OVapi](https://v0.ovapi.nl), electricity prices by
[EnergyZero](https://www.energyzero.nl), fuel prices from the public data of
the [ANWB](https://www.anwb.nl). The typefaces are under the SIL Open Font
License; all bundled licences are in [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES).
