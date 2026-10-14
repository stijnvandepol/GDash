# Progress

State of the project on 8 October 2026, so work can continue on any machine.

## What works

- **Added on 12 October 2026 by parallel agents, merged and tested by the lead:** a Fuel prices widget (the cheapest stations near a place, from the ANWB's public price data, no key), Custom APIs with a Custom widget (any JSON API, open or with a key, token or password, sealed like calendar addresses; values picked by clicking the answer; rules in a small expression language, no scripts) and stricter two-step login (at most 5 wrong codes per pending login, a backoff and a hard stop after 50 wrong codes per account, also across addresses; turning it on only with the secret whose code was checked). Go tests, 338 Vitest tests and 25 Playwright tests passed. The Custom widget has not run on a real TV yet, and two-step login has not been set up with a real phone.
- **Backups:** a consistent copy of the database every day in `backups/`, the last seven kept, made while GDash runs. A backup to download (database, `secret.key`, photos) under Settings → Account, behind the password; tested on 10 October by unzipping one into an empty data directory, starting GDash on it and logging in.
- **Built on 9 October 2026 by six agents in parallel, reviewed and merged by the lead:** announcements with API tokens (tested against a throwaway server with curl and in the browser in three themes and at 200 % text), a live preview in the dashboard editor (draft layouts, three screen shapes, four text sizes), five dashboard templates, air quality and sun widgets (checked at their minimum sizes in five themes and both languages), a redesigned web interface for light and dark (contrast AA, checked at 375, 768 and 1280 px), and 19 Playwright browser tests including axe accessibility checks in light and dark mode, also in CI.
- **Security review (9 October 2026):** a read-only review found eleven points; fixed are parallel login guessing, the launch token that a spoofed Cast device could use, Home Assistant entities reaching screens that do not show them, the token following redirects, missing time-outs, screen tokens that never expired, IPv6 transition addresses to metadata services, widget coordinates that overflow, and the client address and cookie security behind a reverse proxy. Open: a per-username lockout that a LAN device can trigger for 15 minutes, a pending-code pool that rotating IPv6 addresses can fill, unsalted source fingerprints in the cache files, and checking the sender in the own Cast receiver.
- **Home Assistant:** connected under Settings with an address and a long-lived access token, which is tried first and stored encrypted. A Home Assistant widget shows up to eight entities, chosen in the editor by searching Home Assistant's entity list; one entity is shown large, more as a list. The server reads only the entities that dashboards show, once a minute. Tested on 9 October 2026 against a stand-in for Home Assistant's REST API: a wrong token and an unreachable address are reported at their field, numbers show in the screen's language with their unit, a door as closed, a person as away, an unavailable entity as a dash and an unknown one as not found. Not yet tried with a real Home Assistant.
- **Small screens:** a text size per display (100, 125, 150 or 200 %) and a stage that takes the screen's shape. Measured on a 1024x600 Nest Hub screen: at 150 % calendar times grew from 15 to 22 px and headlines from 19 to 30 px, the dashboard filled the screen and no tile overflowed.
- **Weather icons:** the weather and forecast widgets show a line icon for the sky (sun or moon, clouds, rain, drizzle, snow, fog, thunder), drawn for GDash in the theme's colour. Measured at the minimum tile sizes: everything still fits.
- **Photos:** up to 50 photos uploaded under Settings and shown by a Photos widget as a slideshow. They are stored in the data directory and only served to admins and paired screens; uploads are checked by content and size. Tested with synthetic images in the browser.
- **Pages in turn:** a display can show up to five other dashboards one after another with its own, each for a chosen time. The screen turns the pages on its clock, so screens of one display stay in step; a time slot pauses the rotation. Tested in the browser with a 15-second interval.
- **Language on the TV:** English, Dutch, German or French for days, dates, weather words and every other text on the screens, chosen under Settings. The pairing screen follows the device's own language, because it shows before the first snapshot. Tested in the browser with both dashboards.
- **Cast status:** the display list says per TV whether the dashboard is on screen and since when, whether the TV sleeps, another app is on it, or the device cannot be reached and why. It refreshes every 15 seconds. Tested with a display linked to a device that does not exist.
- **Audit log:** logins and failed logins, password changes, pairings and every change to displays, dashboards and settings, with time, user and IP address, under Settings. Kept for 90 days and at most 5000 events. A failed login stores no username and a calendar only its name.
- **Account:** change the password under Settings. The current password is checked with the same rate limit as logins, and every other logged-in browser is logged out.
- **Server (Go):** weather from Open-Meteo without an API key; a data hub with cache, last-known-good on disk and stale markers; an SSE stream per display; SQLite with migrations and backups; accounts with argon2id, sessions and login rate limiting; a JSON API for displays.
- **Web interface:** first-run setup with a token from the server log, login, and a page to add, rename and remove displays and link them to a Cast device found on the network. Devices without a screen (speakers, soundbars) are shown but cannot be picked.
- **Pairing:** a display page shows a code until it is paired in the web interface; a TV opened by GDash pairs itself with a single-use launch token. Only paired screens and logged-in admins receive dashboard data, and unpairing takes effect within 20 seconds.
- **News:** headlines from any RSS or Atom feed (default NOS), fetched once by the server.
- **Calendar colours:** every calendar has its own colour, shown as a dot before each event on the TV; chosen under Settings or given by the calendar's place in the list.
- **Calendar:** the coming week from secret iCal addresses, with recurrences, exceptions and moved occurrences, grouped by day on the TV. Tested with a synthetic calendar; a real Google or iCloud calendar is still to try.
- **Dashboard editor:** dashboards are stored in SQLite and edited by dragging and resizing widgets on the 24x14 grid; each display picks one. Saving reaches open screens within seconds. A display only receives the data its widgets use.
- **Themes:** Minimal dark, Minimal light, Glass, Warm light, High contrast (white on black with thick borders) and Night (dim amber, no blue light, for bedrooms), chosen per dashboard. Glass uses translucent tiles over a still gradient, no blur, so it stays light for Cast receivers.
- **Settings:** the weather location (searched by name through Open-Meteo, or entered as coordinates), up to five news feeds that are merged into one list, and up to ten calendars, all in the web interface. Changes apply within seconds without a restart. Calendar addresses are stored encrypted with a key file in the data directory and never sent back to the browser. Environment variables only fill the settings on the first start. Tested in the browser: seeding from the environment, adding a second feed (merged headlines with feed names), searching and choosing Oss, and a weather widget that says no location is set.
- **Widgets:** clock, weather, a forecast for up to seven days, calendar, news, a note and a countdown. Widget settings are described by the server, so the editor builds its form from them. Every widget was measured at its minimum size on a 1920x1080 stage: type scales with the tile, lists only show items that fit entirely, and a long note shrinks down to 28 px.
- **Schedules:** per display up to six time slots that each show another dashboard, such as a morning dashboard from 06:30 to 09:00, on every day or only on chosen days of the week (a weekend dashboard). The first matching slot wins; outside every slot the display shows its own dashboard. Tested in the browser: the page switched dashboards without a reload.
- **Active hours:** per display, for example 06:30 to 23:00. Outside them the cast is stopped and the page is dark without data. The dashboard shifts a few pixels every three minutes against burn-in.
- **TV page:** clock, weather, calendar and news on a fixed 16:9 stage, an offline indicator, and reconnection without a reload. About 20 KB of JavaScript (gzip).
- **Casting:** for every display linked to a Cast device, a keeper finds the device by ID, starts the receiver and keeps it on screen. Without an own receiver it uses the public DashCast receiver, which needs no Cast developer account.

## Tested on real hardware (home network, 8 October 2026)

- Discovery found 8 Cast devices; the `ca` capability flag separated TVs and displays from speakers.
- A status read over the own CASTV2 client worked on Chromecasts, Google TV devices and a Nest Hub.
- DashCast showed the GDash page on a Chromecast HD (Google TV) and kept a live connection.
- **The Google TV screensaver covered the cast after about 19 minutes**, while the device still reported DashCast as running. Stopping and relaunching the receiver brought the dashboard back. The keeper now does this every 15 minutes (every 9 on a Nest Hub), never while the TV is in standby.
- After the switch to pairing, the TV paired itself through the launch token in the DashCast URL and showed clock, weather and news. The database migrated from version 2 to 3 and wrote its backup first.
- Soak test of 12 hours, 8 October 19:18 to 9 October 07:14 (144 checks every 5 minutes): DashCast was on screen at every check, with 49 periodic restarts on schedule. Between 01:00 and 01:30 the connection to the TV failed six times (timeouts and EOF); each time the keeper reconnected within a second. The server reached the TV through a Tailscale address that night, so a network hiccup on that route is the likely cause. From 01:13 to 07:09 three or four connections from the TV to port 8420 were open instead of one; the count includes ordinary HTTP connections, so it is worth watching on the next run, not yet a finding.
- Memory: the TV server used 26 MB (working set) after five and a half hours; a freshly started server with all features 47 MB. The research's limit is 50 MB.
- Three more problems found and fixed: the URL for the TV used the Tailscale address instead of the home network address; a DashCast session that was already running ignored the new URL; a late status update after a stop caused a second launch.

## Releases

A version tag such as `v1.0.0` builds archives for Linux (amd64, arm64,
armv7) and Windows with checksums as a draft GitHub release, and a Docker
image for amd64 and arm64 on `ghcr.io/stijnvandepol/gdash`. The release
stays a draft until someone publishes it. `gdash version`, the server log
and the web interface show the version.

**0.5.0** was tagged on 9 October 2026: the workflow made the draft release
with the four archives and checksums, and the image. The Windows archive was
downloaded again, its checksum verified, and it started from an empty data
directory. The draft is waiting to be published. It still carries the old
name Daycast, so it is better replaced by a new draft with the next tag.

**0.6.0** was tagged on 11 October 2026, the first release as GDash: the
workflow made the draft release with the four archives, checksums and SBOM,
and the image (as ghcr.io/stijnvandepol/daycast until the repository is
renamed). Its notes are the changelog's 0.6.0 section. The Windows archive
was downloaded again, its checksum verified, and it reported v0.6.0, asked
for setup on an empty data directory and served the web interface.

**0.7.0** was tagged on 12 October 2026, the first release after the repository was renamed to GDash, so its image is `ghcr.io/stijnvandepol/gdash`. It adds German and French, the waste, energy price, departures, fuel price and Custom widgets, the night style, two-step login and the Home Assistant app. The 0.6.0 image only exists as `ghcr.io/stijnvandepol/daycast`.

**0.8.0** was tagged on 12 October 2026: Homey, a TV that is no longer taken back from someone using it, the widget palette and design-only editor with named values for custom APIs, badges, clearer displays and pairing, an update notice, and setup without a token in the first hour. 35 Playwright tests passed.

**0.9.0** was tagged on 13 October 2026: the Integrations tab with presets, push sources (Homey Pro 2016–2019 with HomeyScript), rain, weather warnings, holidays, moon, cameras, Immich and Synology Photos albums, Now playing on Cast devices, solar forecast, web pages, traffic (unofficial ANWB, opt-in) with OpenStreetMap speed cameras, KNMI measured weather, custom APIs that log in first (Pi-hole), and GDash keeping its own dashboard on the TV after a restart. 65 Playwright tests passed. Not yet tried on real hardware: Synology share links, KNMI with a key, Now playing, Homey push.

**0.9.2** was tagged on 14 October 2026: dashboards drawn on TV browsers older than Chrome 105 (the kitchen TV's Chromecast HD showed text at the wrong size and no badge circles), screens report their browser under Displays, the screensaver restart no longer starts over on every reconnect and also reacts when the page is hidden, built-in news sources, waste calendars with their own setup, and long headlines on two lines. 75 Playwright tests passed.

**0.9.3** was tagged on 14 October 2026: 34 new styles (film, series and games, art and abstract, OLED and night) with an OLED rating per style, OLED care and night dimming per display, a title on every tile that does not explain itself, Google, Outlook and iCloud calendars that can be tested and are read even when large, and an integration setup that closes on a click outside it. 87 Playwright tests passed.

## Checked by CI on every push

gofmt, go vet, staticcheck, the Go tests with the race detector (fuzz seeds included), govulncheck, svelte-check with zero warnings, the web unit tests (Vitest), npm audit, an up-to-date THIRD_PARTY_LICENSES, a gitleaks scan of the whole history, 24 Playwright browser tests (setup, settings, backup, dashboards, styles, pairing, schedule) with axe accessibility checks in light and dark mode and a check that every button keeps 4.5:1 while hovered, and a Docker build for amd64 and arm64. The first run passed on 8 October 2026; all of it passed on 9 October 2026 after the security and design rounds.

## Security and design rounds (9 October 2026)

- **OWASP ASVS 4.0.3 level 2 review** (read-only agent): one medium finding, a recurrence rule that cost minutes of CPU per refresh, and a fuzz-found crash on a malformed calendar line, both fixed; all low findings fixed (password policy after NIST SP 800-63B, caching and security headers, content type checks, audit gaps, data directory and database file modes, https-to-http redirects, recovery with `gdash reset-password`). Left as information: no two-factor login (one household admin on a LAN, written down in SECURITY.md), photos keep their EXIF data, the TV's cached snapshot is not cleared on its own.
- **Supply chain:** image bases pinned by digest, non-root image with a read-only compose example, hardened systemd unit, Dependabot, SBOM and provenance (provenance starts once the repository is public). The Docker image built in CI; it has not been run on a NAS yet.
- **Design:** checked in a real Chromium at 1920x1080, 1440x900 and 390 px wide, light and dark. On 10 October the first look (warm paper, teal, Fraunces and Atkinson) was replaced after Stijn chose from three directions: "Apparaat", graphite with signal orange, Geist and Geist Mono, a screen-with-power-light logo, and pages laid out as ruled panels instead of cards. Every text colour computed to reach 4.5:1 in the web interface and in all six TV themes; axe finds nothing serious in light or dark mode.
- **Style presets (10 October):** fifteen themes with their own typeface besides the six (Frost, Frost night, Walnut, Swiss, Tonal, Candy, Gilded, Riviera; inspired by films and series: Galaxy, Terminal, Parchment, Neon, Upside down, Noir, Mars) and a typeface per dashboard out of eleven. Also checked with two test calendars on a local server: event dots, all-day events and day headings read in every theme. Render cost on a 6x slowed CPU, 30 s of a busy dashboard with a photo every 5 s: main-thread work 0.10 to 0.17 s per theme; rasterising 11 ms (Minimal dark) to 62 ms (Glass, which ran 24 hours on the kitchen TV), Neon 61 ms, the other presets 16 to 45 ms. Not yet measured on a real Cast device. At Nest Hub size (1024x600) all twenty-one themes fit too; Gilded and Riviera set body text at weight 500 there so their hairlines stay readable. Checked at 1920x1080 with every widget at its smallest size, also in the widest typeface (JetBrains Mono): no text leaves its tile. themes.test.ts measures every theme's text at 4.5:1 or more.

## Standby

The keeper used to launch the receiver whenever the device showed nothing,
also while the TV was in standby: after a server restart at night that
could switch the TV on through HDMI-CEC. It now leaves a sleeping TV alone,
launches once the TV is switched on (within one 30-second status poll), and
asks the device for its state before every periodic restart. Tested with a
simulated device; not yet on the real TV.

## Not fighting for the TV (12 October)

Two GDash servers cast to the kitchen TV and took it from each other every
few minutes, and the keeper relaunched the dashboard seconds after someone
pressed Back. Decided (the open "relaunch policy"): GDash keeps the session
it launched and yields when someone takes the TV, until the TV has been in
standby or an admin chooses **Show dashboard now**; a keeper that starts
with the server and finds something on the TV yields too, because it
cannot prove the page is its own; it never launches while the TV reports
another input. See `claim` in `internal/cast/claim.go` and
[guide.md](guide.md#when-gdash-is-not-on-the-tv). Tested with the simulated
device; still to watch on the real Google TV: the status after Back or
Home, after opening Netflix, after switching HDMI input and back, during
and after standby, and on a Nest Hub whether `isActiveInput` is absent
rather than false.

## Not tested yet

- **An own receiver** (option A from the research): needs a Cast developer account (USD 5) and a registered device, see [receiver.md](receiver.md).
- **Running the Docker image** on a NAS or Raspberry Pi. CI builds it for amd64 and arm64, but nobody has started it yet; Docker Desktop on the development PC did not start its engine.
- **A real Google or iCloud calendar** and a Dutch display on a real TV.

## Open decisions

- **TLS:** the code guidelines require TLS for all traffic, but a TV opening the display page through DashCast can only use plain HTTP on a LAN address. See "Known oddities" in CODE_GUIDELINES.md.
- **TLS verification towards Cast devices:** off, as in pychromecast. Pinning the certificate on first connection is an option.
- **Name:** GDash since 9 October 2026, the owner's choice; before that Daycast (taken by 22 repositories and an iOS app) and, for one day, Nookboard. GDash is not unique either: ripienaar/gdash (a Graphite dashboard, 750 stars) and kadalu/gdash (GlusterFS) exist and the GitHub user `gdash` is taken. A trademark check (BOIP, EUIPO) is still to do.

## The kitchen TV

The kitchen TV (a Chromecast HD with Google TV, called "tv woonkamer" in Google Home) runs the current version: first on 9 October at 09:13, when the database migrated from version 3 to 10 with a backup before every step and the location and news feed were copied from the environment, and again at 09:38 and 09:58 with the later features (migrations 11 and 12). A 24-hour soak test ran from 9 October 09:14 to 10 October 09:11: in all 288 checks, one every five minutes, the dashboard (DashCast) was on screen, and the event stream was connected in 287 of them (the one miss fell in a restart of the cast). Restarting the server restarts the cast, which can switch a TV on through HDMI-CEC, so that waits until someone is home and awake.

The Nest Hub ("Keuken", 192.168.1.33) ran an 8-hour test on 9 October from a separate instance, with the 9-minute restart: in 240 checks, one every two minutes, the dashboard was on screen 198 times; 40 times radio was playing, and GDash waited instead of taking the screen; twice the device did not answer for a moment and the dashboard came back by itself.

On 12 October the Nest Hub ran the style presets for an hour from the same separate instance, with six dashboards in turn every two minutes (Frost, Walnut, Galaxy, Terminal, Neon, Gilded), so each switch loaded another typeface and texture: in 60 checks, one a minute, the dashboard was on screen every time and the screen kept its connection to the server; the server logged no warning or error. Afterwards the display's active hours were set outside the day, so GDash stopped the cast itself and the Nest Hub went back to its own screen.

On 11 October at 23:07 the kitchen TV's server was updated from 0.5 to the current GDash (the evening, with permission; the TV was already on, so the restart could not switch it on). The database was renamed from daycast.db to gdash.db and migrated from version 12 to 15 with a backup before each step; a copy of the whole data directory and of the old program is kept next to it. The receiver restarted with the new page within three seconds, and the TV held five open connections to the server right after. The program still runs as daycast.exe, built from GDash: Windows Firewall only allows that path to accept connections from the TV, and changing firewall rules is left to Stijn (run gdash.exe once and allow it, then switch).

## Next

1. Choose the location and calendars under Settings on the kitchen TV's server.
2. Test the Nest Hub with the 9-minute restart (with permission, in the daytime).
3. Publish the 0.6.0 draft release when the repository is ready to be seen; the 0.5.0 draft, still named Daycast, can be deleted.
4. Still open from the research's v1.0: an external review of the authentication (the Home Assistant add-on exists since 0.6). Scan the two-step QR code with a real phone, and show a Custom widget on a real TV. Dashboards can be exported and imported; the rest of the configuration moves with the data directory.
