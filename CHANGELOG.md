# Changelog

All notable changes to GDash. Versions follow [Semantic Versioning](https://semver.org);
the format follows [Keep a Changelog](https://keepachangelog.com).

## Unreleased

### Added

- German and French on the TV, next to English and Dutch: every text,
  the weather words and the dates, chosen under Settings, Screens,
  Language on the TVs.
- A dashboard can switch to another theme after sunset: pick one under
  Style → After sunset (for instance Frost night after Frost). The server
  follows sunset and sunrise at the weather location and the screen changes
  within a minute, keeping the typeface; without a location nothing
  changes. The preview shows what a screen shows now, and dashboard files
  keep the choice.
- A Home Assistant app (experimental) in `homeassistant/`: GDash on the host
  network, with its data in the app's data folder, run as an unprivileged
  user. See [docs/install.md](docs/install.md#home-assistant-app).
- `CONTRIBUTING.md`, issue and pull request templates, a code of conduct and
  a checklist for making the repository public ([docs/publishing.md](docs/publishing.md)).
- A **Waste collection** widget: the next pickup of each kind of household
  waste in the coming 14 days, with a colour dot per kind, from one of your
  calendars. Add the iCal link of your municipality or of mijnafvalwijzer or
  afvalkalender as a calendar under Settings, then choose it in the widget.
  GFT, PMD, papier, restafval, glas and textiel (and their English names) are
  recognised; other titles are shown as they are.
- An **Energy price** widget: the dynamic electricity price in the Netherlands
  from the free EnergyZero API, with whether it is low, average or high today, a
  bar for every hour and the cheapest three hours. It is fetched hourly, and
  only while a dashboard has the widget.
- A `calendar` kind for widget settings, with a calendar picker in the
  dashboard editor.
- A Departures widget: the next buses, trams, metros and ferries at one stop
  in the Netherlands, from [OVapi](https://v0.ovapi.nl), without an API key.
  Each line shows its number, destination and the minutes until it leaves, in
  the warning colour with the delay when it is late, struck through when it is
  cancelled. Search the stop by name or town in the dashboard editor. GDash
  asks OVapi once every 30 seconds, only for stops a dashboard shows, and the
  screen marks the times as stale when OVapi cannot be reached. Trains are not
  in OVapi's data; see [docs/guide.md](docs/guide.md#departures).

### Fixed

- Calendar titles no longer show the backslash that feeds put before an escaped
  comma, such as `Plastic\, blik en drinkpakken`.

## 0.6.0 – 2026-10-11

### Changed

- The project is renamed from Daycast to GDash: the program is
  `gdash`, the image `ghcr.io/stijnvandepol/gdash` and the
  environment variables start with `GDASH_`. On its first start
  GDash renames an existing `daycast.db`, so the configuration stays,
  and dashboard files exported by Daycast still import. Log in again, and
  pair screens that show the page in a browser again; cast screens carry on.
- A look of its own, on the TV and in the web interface, like a well-made
  appliance: graphite on light grey, one signal orange that means "on",
  and a logo of a screen with its power light lit. Geist for text and
  figures (no slashed zero on the clock) and Geist Mono for times, codes
  and addresses; both ship with GDash. Minimal dark and Minimal light on
  the TV follow the same colours; the six themes keep their names, Glass
  is slate green, and every text colour in every theme reaches 4.5:1.
- The web interface is laid out like a control panel instead of a stack
  of cards: each page has a large title and its own action, sections are
  ruled with their title in a margin column, a display is drawn as a TV
  showing its dashboard, with a light that is on while the dashboard is
  on screen, and the activity log is a table. Settings is grouped into
  data sources, screens, automations and account; the dashboard editor
  is wider, draws its grid and outlines the selected widget; templates
  and your own dashboards are shown in their theme's colours. Removing something asks in GDash's own dialog
  instead of the browser's, and the main actions confirm with a short
  message.
- New passwords follow NIST SP 800-63B: at least 12 characters, not the
  user name or a well-known password. Existing passwords keep working.
- The Docker image runs as user 65532 with a read-only file system in the
  example compose file, and the systemd unit is sandboxed further.

### Added

- Fifteen style presets for the TV besides the six themes, each with its
  own typeface, tile shape and, for some, a texture: Frost, Frost night,
  Walnut, Swiss, Tonal, Candy, Gilded and Riviera, and, inspired by films
  and series, Galaxy, Terminal, Parchment, Neon, Upside down, Noir and
  Mars. A dashboard can also take another typeface than its theme's, out
  of eleven that ship with GDash. The editor shows them in a gallery of
  small screens in three groups, and three new templates use them: Living
  room, Study and Movie night.
- A backup to download under Settings → Account: one zip file with the
  database, the encryption key and the photos, behind your password. Unzipped
  into an empty data directory it is a working GDash again; see
  [docs/install.md](docs/install.md#a-backup-to-keep-elsewhere).
- Announcements: an automation, such as a Home Assistant `rest_command`,
  posts a short message with an API token and the TVs show it over the
  dashboard for a while. API tokens are made and revoked under Settings,
  where a test announcement can be sent too.
- Live preview in the dashboard editor: the dashboard as a TV draws it,
  with real data and unsaved changes, as a TV, a tablet or a Nest Hub and
  at every text size.
- Dashboard templates: Kitchen, Hallway, Bedroom, Small screen and Family,
  each with a fitting theme, to start from in one click.
- Air quality widget: the European air quality index now and the pollen in
  the air, from Open-Meteo without a key.
- Sun widget: sunrise, sunset, the length of the day and how much longer or
  shorter it is than yesterday.
- A new look for the web interface: light and dark following the system,
  icons, a tab bar at the bottom on phones, messages after an action, and
  branded login and setup screens that explain where the setup token is.
- Browser tests of the main journeys and an accessibility check of every
  page in light and dark mode, in CI.
- `GDASH_TRUSTED_PROXIES`: behind a reverse proxy, its forwarding
  headers set the client address and make cookies Secure over HTTPS.
- Home Assistant: connect it with a long-lived access token under Settings
  and add a Home Assistant widget that shows up to eight sensors or
  switches, in English or Dutch. GDash only reads, once a minute, and
  stores the token encrypted.
- Text size per display (100 to 200 %), for smart displays, tablets and
  small screens; the dashboard now fills screens that are not 16:9.
- `gdash reset-password <user>` for a lost password, and `gdash
  healthcheck`, which the Docker image uses.
- Releases carry an SBOM, and build provenance once the repository is
  public. Dependabot proposes updates weekly; CI also runs staticcheck,
  a secret scan of the whole history and fuzz-test seeds for every parser.

### Security

- Login attempts count before the password is checked, so parallel wrong
  logins cannot exceed the limit, and at most two password hashes run at a
  time.
- A launch token only works from the Cast device it was sent to, so a
  device that answers discovery in the TV's name cannot pair itself.
- Each screen only receives the Home Assistant entities its own widgets
  show, and the Home Assistant token never follows a redirect.
- Screens that have not been used for 90 days are unpaired.
- Slow request bodies and stalled event streams time out.
- IPv6 transition addresses (6to4, Teredo, IPv4-compatible) that reach a
  cloud metadata address are refused, like NAT64 already was.
- Widget positions so large that they wrap around are rejected.
- A calendar line such as `X;A=` no longer crashes the server, a
  recurrence such as `FREQ=SECONDLY` from 1990 no longer costs minutes per
  refresh, and a panic in any data source now fails only that refresh.
- A device on the network can no longer lock the admin out with wrong
  passwords; logins are limited per address, per account from an address
  and per account overall, with IPv6 counted per /64.
- Responses carry no-store for API answers, Permissions-Policy,
  Cross-Origin-Opener- and -Resource-Policy, HSTS behind an HTTPS proxy and
  a CSP on the receiver page; JSON bodies must be application/json.
- The database, its copies and the data directory are readable by GDash
  only; cache fingerprints are keyed with the secret key.
- Redirects from https to http, news feed addresses with a password and
  air quality values too large to be real are refused; the own Cast
  receiver no longer follows navigate messages.
- Failed setups and rejected API tokens are recorded in the activity log.
- Uploaded photos lose their location, camera, date and other metadata
  before they are stored; a JPEG keeps only its orientation.

## 0.5.0 – 2026-10-09

The first release, a beta: it runs every day on a Chromecast HD with
Google TV at home, but it has not been used anywhere else yet. Everything
below is new.

### Screens and casting

- Casts the dashboard to Chromecasts, Google TV and Nest Hubs on your
  network without a Cast developer account, through the public DashCast
  receiver, and keeps it on screen: it restarts the cast every 15 minutes
  (9 on a Nest Hub) against the Google TV screensaver. It never starts a
  cast while the TV is in standby, so it does not switch a TV on at night,
  and never over another app someone casts.
- Shows per display whether the TV shows the dashboard, sleeps, runs
  another app or cannot be reached.
- Any browser or kiosk can open a display at `/d/<display>` and pair with a
  code.
- Active hours per display: outside them the cast stops and the page goes
  dark. The dashboard shifts a few pixels every three minutes against
  burn-in.
- Texts and dates on the TV in English or Dutch.

### Dashboards

- A drag-and-drop editor on a 16:9 grid, several dashboards, and six
  themes: Minimal dark, Minimal light, Glass, Warm light, High contrast and
  Night.
- Widgets: clock, weather, a forecast for up to seven days, calendar, news,
  a note, a countdown and a photo slideshow, with weather icons. Widgets fit
  their type to the size of their tile.
- Schedules: another dashboard during parts of the day, on every day or on
  chosen days of the week.
- Export a dashboard to a file and import it again, here or elsewhere.
- Pages in turn: up to five dashboards shown one after another with the
  display's own, each for 15 seconds to 5 minutes.

### Data

- Weather and place search from Open-Meteo, without an API key.
- Up to five RSS or Atom feeds, merged newest first.
- Up to ten calendars from secret iCal addresses, with recurrences and
  exceptions, stored encrypted, each with its own colour on the TV.
- Everything is fetched once by the server for all screens; the last good
  value is kept when the internet drops and stale data is marked.

### Administration and security

- First-run setup with a token from the log, accounts with argon2id,
  rate-limited logins and password changes.
- Settings for the location, feeds, calendars and the language on the TVs.
- An audit log of logins, pairings and changes, kept for 90 days.
- Daily database backups, the last seven kept, and a backup before every
  database migration.

### Installation

- Docker images for amd64 and arm64, and archives for Linux (amd64, arm64,
  armv7) and Windows, with a hardened systemd unit.
- Licensed under the AGPL-3.0, with the notices of every bundled dependency
  in THIRD_PARTY_LICENSES.
