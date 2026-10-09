# Progress

State of the project on 8 October 2026, so work can continue on any machine.

## What works

- **Backups:** a consistent copy of the database every day in `backups/`, the last seven kept, made while Daycast runs.
- **Weather icons:** the weather and forecast widgets show a line icon for the sky (sun or moon, clouds, rain, drizzle, snow, fog, thunder), drawn for Daycast in the theme's colour. Measured at the minimum tile sizes: everything still fits.
- **Photos:** up to 50 photos uploaded under Settings and shown by a Photos widget as a slideshow. They are stored in the data directory and only served to admins and paired screens; uploads are checked by content and size. Tested with synthetic images in the browser.
- **Pages in turn:** a display can show up to five other dashboards one after another with its own, each for a chosen time. The screen turns the pages on its clock, so screens of one display stay in step; a time slot pauses the rotation. Tested in the browser with a 15-second interval.
- **Language on the TV:** English or Dutch for days, dates, weather words and every other text on the screens, chosen under Settings. The pairing screen follows the device's own language, because it shows before the first snapshot. Tested in the browser with both dashboards.
- **Cast status:** the display list says per TV whether the dashboard is on screen and since when, whether the TV sleeps, another app is on it, or the device cannot be reached and why. It refreshes every 15 seconds. Tested with a display linked to a device that does not exist.
- **Audit log:** logins and failed logins, password changes, pairings and every change to displays, dashboards and settings, with time, user and IP address, under Settings. Kept for 90 days and at most 5000 events. A failed login stores no username and a calendar only its name.
- **Account:** change the password under Settings. The current password is checked with the same rate limit as logins, and every other logged-in browser is logged out.
- **Server (Go):** weather from Open-Meteo without an API key; a data hub with cache, last-known-good on disk and stale markers; an SSE stream per display; SQLite with migrations and backups; accounts with argon2id, sessions and login rate limiting; a JSON API for displays.
- **Web interface:** first-run setup with a token from the server log, login, and a page to add, rename and remove displays and link them to a Cast device found on the network. Devices without a screen (speakers, soundbars) are shown but cannot be picked.
- **Pairing:** a display page shows a code until it is paired in the web interface; a TV opened by Daycast pairs itself with a single-use launch token. Only paired screens and logged-in admins receive dashboard data, and unpairing takes effect within 20 seconds.
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
- DashCast showed the Daycast page on a Chromecast HD (Google TV) and kept a live connection.
- **The Google TV screensaver covered the cast after about 19 minutes**, while the device still reported DashCast as running. Stopping and relaunching the receiver brought the dashboard back. The keeper now does this every 15 minutes (every 9 on a Nest Hub), never while the TV is in standby.
- After the switch to pairing, the TV paired itself through the launch token in the DashCast URL and showed clock, weather and news. The database migrated from version 2 to 3 and wrote its backup first.
- Soak test of 12 hours, 8 October 19:18 to 9 October 07:14 (144 checks every 5 minutes): DashCast was on screen at every check, with 49 periodic restarts on schedule. Between 01:00 and 01:30 the connection to the TV failed six times (timeouts and EOF); each time the keeper reconnected within a second. The server reached the TV through a Tailscale address that night, so a network hiccup on that route is the likely cause. From 01:13 to 07:09 three or four connections from the TV to port 8420 were open instead of one; the count includes ordinary HTTP connections, so it is worth watching on the next run, not yet a finding.
- Memory: the TV server used 26 MB (working set) after five and a half hours; a freshly started server with all features 47 MB. The research's limit is 50 MB.
- Three more problems found and fixed: the URL for the TV used the Tailscale address instead of the home network address; a DashCast session that was already running ignored the new URL; a late status update after a stop caused a second launch.

## Releases

A version tag such as `v1.0.0` builds archives for Linux (amd64, arm64,
armv7) and Windows with checksums as a draft GitHub release, and a Docker
image for amd64 and arm64 on `ghcr.io/stijnvandepol/daycast`. The release
stays a draft until someone publishes it. `daycast version`, the server log
and the web interface show the version. No release has been made yet; the
build script was tested locally with v0.9.0-test.

## Checked by CI on every push

gofmt, go vet, the Go tests with the race detector, govulncheck, svelte-check with zero warnings, the web unit tests (Vitest), npm audit, an up-to-date THIRD_PARTY_LICENSES, and a Docker build for amd64 and arm64. The first run passed on 8 October 2026.

## Standby

The keeper used to launch the receiver whenever the device showed nothing,
also while the TV was in standby: after a server restart at night that
could switch the TV on through HDMI-CEC. It now leaves a sleeping TV alone,
launches once the TV is switched on (within one 30-second status poll), and
asks the device for its state before every periodic restart. Tested with a
simulated device; not yet on the real TV.

## Not tested yet

- **A full 24-hour run** with the periodic restart (12 hours done, see above), on the current version.
- **The Nest Hub** with the 9-minute restart.
- **An own receiver** (option A from the research): needs a Cast developer account (USD 5) and a registered device, see [receiver.md](receiver.md).
- **Running the Docker image** on a NAS or Raspberry Pi. CI builds it for amd64 and arm64, but nobody has started it yet; Docker Desktop on the development PC did not start its engine.
- **A release:** the release workflow has not run yet; the build script was tested locally.
- **A real Google or iCloud calendar** and a Dutch display on a real TV.

## Open decisions

- **Relaunch policy** (`ShouldRelaunch` in `internal/cast/keeper.go`): Daycast only takes over a screen when nothing else runs. Google TV reports "no apps" while a native app such as Netflix is on screen, so Daycast could start over it.
- **TLS:** the code guidelines require TLS for all traffic, but a TV opening the display page through DashCast can only use plain HTTP on a LAN address. See "Known oddities" in CODE_GUIDELINES.md.
- **TLS verification towards Cast devices:** off, as in pychromecast. Pinning the certificate on first connection is an option.
- **Name:** Daycast is the working name; a trademark check (BOIP, EUIPO) is still to do.

## The living room TV

The living room TV runs the current version: first on 9 October at 09:13, when the database migrated from version 3 to 10 with a backup before every step and the location and news feed were copied from the environment, and again at 09:38 and 09:58 with the later features (migrations 11 and 12). A 24-hour soak test on it started at 09:14. Restarting the server restarts the cast, which can switch a TV on through HDMI-CEC, so that waits until someone is home and awake.

## Next

1. Deploy the current version to the living room TV during the day, and choose the location and calendars under Settings.
2. Test the Nest Hub with the 9-minute restart (with permission, in the daytime).
3. Make the first release (tag `v0.5.0`) and publish the draft after checking it.
4. Still open from the research's v1.0: a Home Assistant add-on and an external review of the authentication. Dashboards can be exported and imported; the rest of the configuration moves with the data directory.
