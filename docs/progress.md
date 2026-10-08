# Progress

State of the project on 8 October 2026, so work can continue on any machine.

## What works

- **Server (Go):** weather from Open-Meteo without an API key; a data hub with cache, last-known-good on disk and stale markers; an SSE stream per display; SQLite with migrations and backups; accounts with argon2id, sessions and login rate limiting; a JSON API for displays.
- **Web interface:** first-run setup with a token from the server log, login, and a page to add, rename and remove displays and link them to a Cast device found on the network. Devices without a screen (speakers, soundbars) are shown but cannot be picked.
- **TV page:** clock and weather on a fixed 16:9 stage, an offline indicator, and reconnection without a reload. About 20 KB of JavaScript (gzip).
- **Casting:** for every display linked to a Cast device, a keeper finds the device by ID, starts the receiver and keeps it on screen. Without an own receiver it uses the public DashCast receiver, which needs no Cast developer account.

## Tested on real hardware (home network, 8 October 2026)

- Discovery found 8 Cast devices; the `ca` capability flag separated TVs and displays from speakers.
- A status read over the own CASTV2 client worked on Chromecasts, Google TV devices and a Nest Hub.
- DashCast showed the Daycast page on a Chromecast HD (Google TV) and kept a live connection.
- **The Google TV screensaver covered the cast after about 19 minutes**, while the device still reported DashCast as running. Stopping and relaunching the receiver brought the dashboard back. The keeper now does this every 15 minutes (every 9 on a Nest Hub), never while the TV is in standby.
- Three more problems found and fixed: the URL for the TV used the Tailscale address instead of the home network address; a DashCast session that was already running ignored the new URL; a late status update after a stop caused a second launch.

## Checked by CI on every push

gofmt, go vet, the Go tests with the race detector, govulncheck, svelte-check with zero warnings, npm audit, and a Docker build for amd64 and arm64. The first run passed on 8 October 2026.

## Not tested yet

- **A full 24-hour run** with the periodic restart.
- **The Nest Hub** with the 9-minute restart.
- **An own receiver** (option A from the research): needs a Cast developer account (USD 5) and a registered device, see [receiver.md](receiver.md).
- **Running the Docker image** on a NAS or Raspberry Pi. CI builds it for amd64 and arm64, but nobody has started it yet.

## Open decisions

- **Relaunch policy** (`ShouldRelaunch` in `internal/cast/keeper.go`): Daycast only takes over a screen when nothing else runs. Google TV reports "no apps" while a native app such as Netflix is on screen, so Daycast could start over it.
- **TLS:** the code guidelines require TLS for all traffic, but a TV opening the display page through DashCast can only use plain HTTP on a LAN address. See "Known oddities" in CODE_GUIDELINES.md.
- **TLS verification towards Cast devices:** off, as in pychromecast. Pinning the certificate on first connection is an option.
- **License:** AGPL-3.0 or Apache-2.0. The repository stays private until this is chosen.
- **Name:** Daycast is the working name; a trademark check (BOIP, EUIPO) is still to do.

## Next

Measure a full day on the living room TV and a Nest Hub, then continue with v0.5 from the research (`GoogleHome-Dashboard/docs/onderzoek/`, chapters 18 and 19): display pairing for the TV pages, the grid editor and more widgets.
