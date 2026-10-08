# Daycast

Your day on every TV in the house. Self-hosted, casts to the Chromecast or
Google TV you already own. No extra hardware, no subscription, no cloud.

> **Status: prototype.** It works on real hardware at home, but there is no
> dashboard editor yet and the TV pages are not paired. Run it only on a
> network you trust.

## What works

- **Weather** from [Open-Meteo](https://open-meteo.com) (no API key). The last
  good value is kept when the internet drops, and stale data is marked.
- **News headlines** from any RSS or Atom feed (default: NOS). Fetched once by
  the server for all TVs, with conditional requests; only plain-text titles
  reach the screen.
- **A web interface** at `/` to set up the first account, log in and manage
  displays.
- **Displays**: link a display to a Cast device found on your network, and
  Daycast keeps the dashboard on that TV. It restarts the cast every 15
  minutes, because Google TV covers a running cast with its screensaver after
  about 19 minutes. No Home Assistant, no HTTPS and no Cast developer account
  needed.
- **Browser displays**: any browser, kiosk or Raspberry Pi can open
  `/d/<display>` instead.

Progress and test results: [docs/progress.md](docs/progress.md). The research
behind the design (Dutch) is in
[GoogleHome-Dashboard/docs/onderzoek](https://github.com/stijnvandepol/GoogleHome-Dashboard/tree/main/docs/onderzoek).

## Run it

Requirements: Go 1.26 and Node.js 24.

```bash
cd web && npm ci && npm run build && cd ..
go build -o daycast ./cmd/daycast
DAYCAST_LATITUDE=51.65 DAYCAST_LONGITUDE=5.95 ./daycast
```

On first start Daycast logs a **setup token**. Open `http://<server>:8420/`,
enter the token and create your account. Then add a display and pick your TV.

With Docker: set the location in `compose.yaml`, run `docker compose up -d`
and read the token with `docker compose logs daycast`.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `DAYCAST_LATITUDE`, `DAYCAST_LONGITUDE` | required | Location for the weather, rounded to about 1 km before it leaves the server |
| `DAYCAST_ADDR` | `:8420` | Listen address |
| `DAYCAST_DATA_DIR` | `data` | Database and cached data |
| `DAYCAST_NEWS_FEED` | NOS general news | RSS or Atom URL for the news widget |
| `DAYCAST_CAST_APP_ID` | none | App ID of an own registered receiver ([docs/receiver.md](docs/receiver.md)); without it Daycast uses DashCast |

## Command line

```bash
daycast                    # start the server
daycast discover           # list Cast devices on the network
daycast status <address>   # show what a Cast device is doing, without changing it
```

## Development

```bash
go test ./...                 # Go tests
go vet ./...
cd web && npm run check       # Svelte and TypeScript checks
cd web && npm run dev         # frontend with hot reload, proxies /api to :8420
```

The code follows [CODE_GUIDELINES.md](CODE_GUIDELINES.md). The API is described
in [docs/api.md](docs/api.md).

## License

Not chosen yet (AGPL-3.0 or Apache-2.0, see the research). Until a license is
added, no rights are granted to reuse this code.
