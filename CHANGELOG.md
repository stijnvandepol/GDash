# Changelog

All notable changes to Daycast. Versions follow [Semantic Versioning](https://semver.org);
the format follows [Keep a Changelog](https://keepachangelog.com).

## Unreleased

The first release. Everything below is new.

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
  a note, a countdown and a photo slideshow. Widgets fit their type to the
  size of their tile.
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
