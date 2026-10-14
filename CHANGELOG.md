# Changelog

All notable changes to GDash. Versions follow [Semantic Versioning](https://semver.org);
the format follows [Keep a Changelog](https://keepachangelog.com).

## Unreleased

### Added

- Twelve more styles inspired by films, series and games, each with its own
  colours, typeface, tile shape and a pattern drawn without images. Under
  Film and series: Hyperspace, Dark side, Desert planet, Wizard school,
  Middle realm, Rain city, Time machine and Agent 00; in a new Games group:
  Shiny hunt, Pocket console, Block world and Quest menu. They borrow no
  names, logos or symbols, and every text colour reaches 4.5:1.
- Two typefaces to choose per dashboard: Oswald, a condensed sans, and
  Jersey 10, a pixel face (both SIL Open Font License).

### Fixed

- An integration's setup closes when you click outside it or press Escape,
  instead of staying open until its own Close.
- Two typefaces to choose per dashboard, Jost (geometric) and Fraunces (an
  expressive serif), both under the SIL Open Font License and shipped with
  GDash.

## 0.9.2 – 2026-10-14

### Added

- News sources to choose from: about 50 feeds that their publishers offer,
  such as the NOS, NU.nl, RTL Nieuws, the national papers, Tweakers,
  Security.nl, twelve regional broadcasters, VRT NWS, the BBC, The Guardian,
  tagesschau, Le Monde and France 24, each checked to answer with recent
  headlines. Under Integrations, News feeds a searchable list by country and
  kind lets you tick them; your own RSS or Atom address still works. GDash
  keeps up to ten feeds (was five), and refuses the same address twice.
  `GET /api/news-catalogue` lists them.
- A News widget can show only the sources you tick in its settings, so each
  screen shows its own news; without a choice it shows all of them, as
  before. A widget with one source has that source's name as its title, and
  with several each headline names its source. A screen only receives the
  headlines of the sources its own widgets show.

### Changed

- The news feed of the first start is called NOS Nieuws instead of
  feeds.nos.nl, also in existing installations that kept that name.
- Waste collection has a setup of its own under Integrations: paste the
  calendar link of your address from your municipality or mijnafvalwijzer.nl,
  with a short guide to where to find it, and **Test** shows the pickups of
  the next 8 weeks and the kinds of waste it recognises before you add it.
  Add one per address, such as home and a holiday house. Waste calendars no
  longer appear under Calendars or in the Calendar widget, and a Waste
  collection widget only offers waste calendars; with one, it shows that one
  without asking. A calendar that a Waste collection widget already showed
  moves to the Waste collection card by itself when you upgrade.
- The Waste collection widget recognises the collection of Christmas trees
  (kerstbomen), with a red dot.
- Cards that use the weather location say so, such as "Uses your weather
  location: Oss · Change", and the Weather card's setup lists them.
- The Photos card is called Uploaded photos, and it, Immich and Synology
  Photos each say that they are a source of the Photos widget; the API
  tokens and Homey cards say what they are for.

### Fixed

- A long headline in the News widget wraps onto a second line instead of
  being cut off halfway; headlines that do not fit the tile are left out whole.
- On a TV whose built-in browser is older than Chrome 105, such as the Cast
  runtime of many Chromecasts, widgets size their text to their tile again:
  the Weather widget fits, the Waste widget's text is no longer tiny and the
  badges of Who's home keep their ring. GDash loads a fallback for
  container queries there, and on Chrome 85 to 87 also for the scoped
  styles that relied on `:where()`. Chrome 80 to 84 can run the page again
  too, and the Weather warnings banners and the Rain chart's base line no
  longer depend on `color-mix()`.
- The Waste widget's text grows with its tile; at 6x4 it was 25 pixels on a
  720p TV. A Weather widget in a short tile sizes its lines to the tile, so
  the measured temperature's line of the KNMI fits and reads at 26 pixels
  instead of 22.

### Added

- The dashboard on a Google TV is restarted the moment its page says the
  screensaver covered it, not only every 15 minutes, and only while the TV
  shows GDash and nobody uses it. At each Cast display, **Keep in front of
  the screensaver** chooses how often it is restarted: every 5, 10, 15 or 30
  minutes, or off (`screensaverMinutes`). The display list says when GDash
  last restarted the dashboard and when the screen was last hidden, and the
  log has a line for every restart with its reason.
- Every screen reports what its browser can do: under **Displays**, **What
  the screen reports** names the browser and version, the screen size and,
  in plain words, what the browser lacks and what GDash does about it, plus
  script errors and when its page was last hidden or shown.
  `POST /api/displays/{id}/screen-reports` takes the report
  from the screen itself; `GET /api/displays` lists them as `screenReports`.
  `?compat=1` on a display or preview page shows the fallback for older
  browsers in any browser.

## 0.9.1 – 2026-10-14

### Changed

- The devices and services that used to be presets of a custom API, such as
  the HomeWizard meters, Tibber, Pi-hole, Plex and Todoist, are integrations
  of their own: their card asks only for what is yours, such as an IP
  address or a token, with **Test** before **Save**, says whether it is
  connected, and lists each one when you set up more than one, such as two
  P1 meters. How GDash asks them is under Advanced. AdGuard Home, Pi-hole and
  Speedtest Tracker are under a new Network category.
- In the dashboard editor each device or service that is set up is a widget
  of its own, such as HomeWizard P1 meter or Todoist, that starts with
  sensible settings and suggested rules, with what to show and which values
  chosen by name. It is still stored as a Custom widget, so exported
  dashboards keep working.
- Custom APIs, its card and its widget are for your own JSON APIs only; a
  custom API made from a preset before can be moved under its own card.

### Added

- A custom source keeps the preset it was made from (`preset`), and
  `POST /api/custom-source-tests` fetches a source once without saving it.

## 0.9.0 – 2026-10-13

### Added

- Camera widget: a still image of a camera at home, such as Frigate's latest
  image or a Reolink, Hikvision or Home Assistant snapshot, refreshed every 5
  to 300 seconds. Cameras are added under Integrations, Camera snapshots, with a user
  name and password, a token or an API key, which are stored encrypted. GDash
  fetches the image itself, once per refresh however many screens show it and
  only while one does, and only screens of a display that shows the camera can
  load it. When the camera stops answering, the last image stays with the time
  it was taken in the warning colour.
- The Photos widget can show an Immich album: connect Immich under
  Integrations, choose an album, and set the widget to Immich album. A random
  part of up to 100 photos changes every 30 minutes; videos are left out, and
  the screens never see Immich's address or key.
- The Photos widget can show a Synology Photos album through its share link,
  with or without a password, so GDash needs no account on the NAS: set it up
  under Integrations, Synology Photos. For a NAS with a self-signed
  certificate, Accept this NAS's certificate pins the certificate it shows at
  that moment; the http link on port 5000 works too.
- A **Rain** widget: the rain in the next two hours at the weather location or
  a place you choose, as a bar per five minutes and one sentence such as "Dry
  for the next 2 hours", "Rain from 14:20 (light)" or "Rain until 15:05
  (moderate)", in the screen's language. Light is up to 2.5 mm/h, moderate up
  to 10, heavy above. The forecast is Buienradar's in the Netherlands and
  Belgium, credited on the widget, and Open-Meteo's elsewhere or when
  Buienradar fails; no key. GDash asks every 5 minutes, once per place; see
  [docs/guide.md](docs/guide.md#rain).
- A **Weather warnings** widget: the KNMI's code yellow, orange and red
  warnings for a province, or the province of the weather location, from the
  public MeteoAlarm feed, every 15 minutes. "No warnings" in a calm style, or a
  banner per warning with the code, the kind of weather and when; red also has
  a warning sign. With **Announce** on, a code orange or red warning that
  starts is shown over every screen for five minutes, once per warning; see
  [docs/guide.md](docs/guide.md#weather-warnings).
- A **Now playing** widget shows what plays on the other Cast devices in the
  house (speakers, speaker groups, Nest Hubs): the device, the app, the
  title, artist and album, the cover and how far the track is, with the
  most recently started one large and the others in a list. It follows any
  device or up to eight chosen ones, never a TV that GDash shows a
  dashboard on, and only reads: it never starts, pauses or stops anything.
  Covers are fetched by the server and served at
  `GET /api/now-playing/artwork/{id}` only to the screens that show them.
  The TV shows what plays in the house; see
  [docs/guide.md](docs/guide.md#now-playing).
- A `boolean` kind for widget settings, shown as a checkbox in the editor.
- `GDASH_TEST_UPSTREAM` lets the end-to-end tests stand in for Buienradar,
  Open-Meteo's rain forecast and MeteoAlarm on the loopback address.
- A **Solar forecast** widget: what the household's solar panels are
  expected to produce today, the best hour, a bar per hour for the rest of
  the day and tomorrow's total. Set the panels up under Integrations, Solar
  panels: tilt, direction and peak power, for one or two groups of panels,
  at the weather location or a place of their own. GDash computes the
  estimate from Open-Meteo's sunlight forecast on the panels' plane and a
  performance ratio (0.85 unless you change it), or asks Forecast.Solar
  instead; both need no key. It is refreshed every 30 minutes and is an
  estimate from the cloud forecast, not a measurement.
- A custom API can **log in first**, for an API that gives a session token
  for a password, such as Pi-hole 6. Choose the authentication "Log in
  first": GDash posts a JSON body with the password in it to a login address
  on the API's own host, reads the token from the answer (such as
  `session.sid`) and sends it in a header, an address parameter or as a
  bearer token. It keeps the token encrypted in memory only and logs in
  again when it runs out or the API refuses it. The guide has the Pi-hole
  settings.

### Changed

- The editor names each count after what the widget shows: "Headlines at
  most" instead of "Items", and "Days ahead" for the forecast.
- The data sources moved from Settings to a fourth tab, Integrations:
  weather location, news feeds, calendars, Home Assistant, Homey, custom
  APIs, photos and API tokens. Settings keeps the language on the TVs, the
  test announcement and the account. Texts on the TVs and error messages
  now point to Integrations.

### Fixed

- A **Holidays** widget lists the coming school holidays of your region
  (North, Middle or South) and the Dutch public holidays, the soonest
  first, in the language of the screens: "Autumn holiday · Starts in 4 days
  (17–25 Oct)", "King's Day · Tue 27 Apr", and "Now, until Sunday" for the
  holiday that is on. It can show both, or only school or only public
  holidays. The school holidays come from the government's open data once a
  day, only while a dashboard shows them; the public holidays, Easter and
  King's Day on a Sunday included, are worked out without network.
- A **Moon** widget draws the moon in its current phase in the colours of
  the theme, with the phase's name, how much is lit and the next full and
  new moon. It is computed on the screen, without network.
- A Web page widget shows a page in a tile, such as a Grafana panel, a Home
  Assistant dashboard in kiosk mode, a weather radar or a status page on
  your network. Admins add pages under Integrations, Web pages,
  with a reload interval, a zoom and whether scripts may run, and **Check**
  says whether the page allows being shown in a frame (with the fix for Home
  Assistant and Grafana). The editor picks a page by name. The page is
  sandboxed, and a TV's page only lets in frames from the pages its own
  dashboards show. See the guide, "Web page".
- Traffic widget: jams, roadworks and closures, mobile speed checks and fixed
  speed cameras on up to ten roads, or within a radius of the weather
  location. Jams, roadworks and speed checks come from the ANWB's traffic
  list, unofficially: it is off until an admin turns on "ANWB traffic info
  (unofficial)" under Integrations and acknowledges it is for their own household
  only, it is asked at most once every 3 minutes for all screens, backs off
  after failures and stops after repeated refusals, and screens say
  "Unofficial · ANWB". Fixed cameras and average speed checks come from
  OpenStreetMap through the Overpass API, at most once a week, and need no
  setting.
- Measured weather: with a free KNMI EDR API key under Integrations, KNMI
  measured weather, a
  Weather widget set to "Measured by KNMI" shows the temperature, humidity
  and wind that the nearest KNMI station measured, every 10 minutes, with the
  station and time.
- `GDASH_TEST_UPSTREAM` also stands in for the ANWB's traffic list, Overpass
  and the KNMI.
- A custom API can be sent to GDash instead of fetched, for a system that
  cannot answer requests, such as a Homey Pro (Early 2019) with HomeyScript,
  Node-RED or a Home Assistant automation. Choose "Your system sends it to
  GDash" and GDash shows the address, makes an API token and gives curl,
  HomeyScript and Home Assistant examples. The system sends its JSON with
  `PUT /api/custom-sources/{id}/data` and an API token; it is handled like a
  fetched answer and marked as old after two intervals and 30 seconds
  without data. API tokens can now also send this data.
- Homey Pro (2016–2019) support. This Homey has no API keys, so under
  Settings, Homey, choose that your Homey sends its data: GDash shows a
  HomeyScript, with an API token filled in, that a Flow runs every minute.
  It sends every device and user to `PUT /api/homey/snapshot`; the Homey and
  Who's home widgets and the editor's pickers work from the last snapshot,
  which survives a restart, and mark values as old after three minutes
  without one.
- An Integrations page lists every service GDash can show as a searchable
  catalogue, grouped by category, with filters for what is connected,
  Dutch, local or without a key. The search stays in the address. Each card
  says whether it is connected and when it last updated, or why it fails,
  and opens its setup under it. Built-in sources (fuel prices, departures,
  electricity prices, air quality, sunrise and sunset, waste collection)
  appear as cards that lead to the dashboards; cards for sources that are
  still being built show by themselves once the server has them.
- Custom API presets for the HomeWizard P1, kWh and water meters, P1
  Monitor, DSMR-reader, Fronius, SolarEdge, Tibber, Electricity Maps, the
  Quatt heat pump, AdGuard Home, Speedtest Tracker, Jellyfin, Plex, Mealie
  and Todoist. Fill in the IP address or key; GDash checks it, makes the
  custom API with its named values, asks it once and shows what it says.
  The presets are reviewable JSON files in `web/src/admin/presets`, and the
  server's tests check each against its rules.
- After an update or restart GDash no longer says "Someone is using the TV"
  about its own dashboard: it remembers the cast it started and keeps it,
  with the restarts against the screensaver.

## 0.8.0 – 2026-10-12

### Changed

- The Displays page says what each screen is doing in plain words. A display
  on a Cast TV leads with the TV's state, such as "On Tv Keuken since 13:40",
  "Tv Keuken is asleep (standby)" or "Cannot find Tv Keuken on the network",
  and only counts the extra screens it is open on ("Also open on 2 other
  screens"). A display without Cast says "No screen yet" with its address and
  the field for the code right in its row, and "Shown on 1 screen" once
  connected. "Pair a screen" became "Open on another screen…" or "Add another
  screen…", "Open screen" became "Preview", and the dashboard choice is
  labelled **Shows**. The cast status in `GET /api/displays` gains a `reason`
  for an unreachable TV: `not_found` or `no_answer`.
- The code screen on a TV says "Connect this screen" and names the display by
  the address the web interface shows, in all four TV languages.
- The dashboard editor adds widgets from a panel beside the canvas instead
  of a row of buttons above it: a search on name and description, then the
  widgets by category (Time & date, Weather & nature, Home, Agenda &
  household, Travel, Media & news), each with an icon and a line on what it
  shows. A selected widget's settings take the panel's place, with a way
  back; on a phone the panel sits below the canvas.
- The Custom API widget's settings are about design only: choose the API,
  how to show it and its named values from lists, and build rules as a
  sentence ("When Outside is below 0, colour Bad"). The JSON tree and paths
  moved to Settings; written conditions stay under Advanced. Widgets keep
  storing paths, so existing dashboards and dashboard files work as before.
- **GDash no longer takes the TV back from someone using it.** When the
  dashboard is closed on the remote, an app or another cast replaces it,
  or the TV switches input, GDash waits until the TV has been switched off,
  or has shown its own idle screen for 10 minutes, as a Nest Hub does after
  the radio (state `yielded`, "Someone is using the TV"). Two GDash
  servers on one TV no longer take it from each other. After a restart
  GDash also waits when it finds something on the TV, and it never starts
  while the TV shows another input. See
  [docs/guide.md](docs/guide.md#when-gdash-is-not-on-the-tv).
- The first account can be created without the setup token from the home
  network, in the first hour after GDash starts, as with Home Assistant's
  onboarding. After that hour, or from outside the home network, the token
  from the log is still needed.

### Added

- **Badges** for the Custom widget: a round badge with a short label,
  such as initials, for every value that is on (`true`, a number other
  than 0, or `on`, `home`, `yes`, `open` or `true`), for example to show
  who is home. Screens only receive the badges that are on. See
  [docs/guide.md](docs/guide.md#badges-who-is-home).
- **Homey**: connect a Homey Pro under Settings, Data sources with its
  address and an API key (read only, stored encrypted). The Homey widget
  shows up to eight device values with their units, and yes or no in
  words; the Who's home widget shows a badge with initials for each chosen
  person who is home, and screens never learn who is away. See
  [docs/guide.md](docs/guide.md#homey).
- **Show dashboard now** at a display whose TV someone else is using, and
  `POST /api/displays/{id}/cast-takeovers` for the same.
- **Named values** for custom APIs: under Settings, Custom APIs, Values,
  click values in the answer to give them a name (with a unit and
  decimals) and name lists with up to three columns. Stored with the API
  (`PUT /api/custom-sources/{id}/values`); see
  [docs/guide.md](docs/guide.md#name-its-values).
- `isOn(x)` in rule conditions, true when a badge would be on.
- Adding a display asks what it **shows**, offering the newest dashboard
  instead of always the default one. `POST /api/displays` takes `dashboardId`,
  and `GET /api/dashboards` gives each dashboard's `createdAt`.
- **Update notice:** once a day GDash asks GitHub for the latest release and,
  when it is newer, says "GDash 0.8.0 is available" at the top of the web
  interface, with links to what is new and to the update steps. The request
  sends only the version, in the User-Agent; drafts and pre-releases are
  ignored. Turn it off under Settings, Account, Updates, or with
  `GDASH_UPDATE_CHECK=off`. It is the only request GDash makes apart from its
  data sources ([SECURITY.md](SECURITY.md)).
- [docs/install.md](docs/install.md#updating) has the update steps for
  Synology Container Manager, Docker Compose (with optional automatic updates),
  the binaries and the Home Assistant app.

### Fixed

- Unpairing the extra screens of a display on a Cast TV no longer unpairs
  the TV itself, which then showed a pairing code until the next relaunch.

## 0.7.0 – 2026-10-12

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
- A **Fuel prices** widget: the cheapest petrol stations within 1 to 25 km of a
  place, for Euro 95 / E10, Euro 98, diesel, LPG or CNG, from the public price
  data of the ANWB, without an API key. Each row shows the price with three
  decimals, the station and its distance; the cheapest is marked with what it
  saves per litre against the dearest in the list. Choose the place with the
  place search in the dashboard editor, or leave it empty to use the weather
  location. GDash asks the ANWB every 15 minutes, once per place and radius a
  dashboard shows, and the screen marks the prices as stale when the ANWB cannot
  be reached. The prices may lag a station's own sign; see
  [docs/guide.md](docs/guide.md#fuel-prices).
- `place` and `choice` kinds for widget settings, with a place picker in the
  dashboard editor.
- **Custom APIs** and a **Custom API** widget: show data from any JSON API
  without code. Add the API under Settings, Data sources, Custom APIs, with
  GET or POST, a bearer token, an API key in a header or the address, or a
  user name and password, extra headers and a refresh from 30 seconds to a
  day; secrets are stored encrypted and never shown again. Test it, or
  paste a sample answer, and click values in the answer to pick what the
  widget shows: one value with a label and unit, a list with up to three
  values per row, or up to eight labelled values. Rules such as
  `value > 25` colour the value or a row, show a message, hide the data or
  announce the message on every display when they start to hold (at most
  every ten minutes). Rules are a small expression language, not scripts, so
  no code from the web interface ever runs on the server or the TVs. Only
  APIs that a dashboard shows are fetched, and each screen only gets the
  values its own widgets show. See [docs/guide.md](docs/guide.md#custom-apis).

### Fixed

- Calendar titles no longer show the backslash that feeds put before an escaped
  comma, such as `Plastic\, blik en drinkpakken`.

### Security

- Optional two-step login under Settings → Account: after the password,
  GDash asks for a 6-digit code from an authenticator app (RFC 6238; the QR
  code to scan is drawn by GDash itself). Setting it up asks for the
  password again and only turns it on after a first code; ten single-use
  recovery codes are shown once, with a download as text. Each code works
  once, wrong codes count towards the login limits, and the right password
  no longer clears them. The secret is encrypted with `secret.key`. Turning
  it off needs the password and a code. Wrong codes, recovery codes and
  every change are in the activity log. `gdash reset-password` now also
  turns two-step login off, for an admin who lost the phone and the
  recovery codes.

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
