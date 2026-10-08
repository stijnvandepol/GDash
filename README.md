# Daycast

Your day on every TV in the house. Self-hosted, casts to the Chromecast or
Google TV you already own. No extra hardware, no subscription, no cloud.

> **Status: v0.1 prototype.** This version exists to prove the risky parts of
> the design on real hardware. It has no setup wizard, no editor and **no
> authentication**: only run it on a network you trust.

## What works

- A Go server that fetches the weather from [Open-Meteo](https://open-meteo.com)
  (no API key), keeps the last good value when the internet drops, and marks
  data as stale.
- A display page at `/d/<name>` for any browser, kiosk or Raspberry Pi, updated
  live over Server-Sent Events. It keeps working when the server restarts.
- A Cast receiver and a built-in Cast sender: Daycast finds Chromecasts with
  mDNS, launches its receiver and keeps it on screen, without Home Assistant,
  without HTTPS and without a cloud service.

The research behind the design is in
[GoogleHome-Dashboard/docs/onderzoek](https://github.com/stijnvandepol/GoogleHome-Dashboard/tree/main/docs/onderzoek)
(Dutch).

## Run it

Requirements: Go 1.26 and Node.js 24.

```bash
cd web && npm ci && npm run build && cd ..
go build -o daycast ./cmd/daycast
DAYCAST_LATITUDE=51.65 DAYCAST_LONGITUDE=5.95 ./daycast
```

Open `http://<server>:8420/d/kitchen` in a browser.

Or with Docker: edit the location in `compose.yaml`, then `docker compose up -d`.

To show the dashboard on a TV, follow [docs/receiver.md](docs/receiver.md).

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `DAYCAST_LATITUDE`, `DAYCAST_LONGITUDE` | required | Location for the weather, rounded to about 1 km before it leaves the server |
| `DAYCAST_ADDR` | `:8420` | Listen address |
| `DAYCAST_DATA_DIR` | `data` | Where the last known data is kept |
| `DAYCAST_CAST_DEVICE` | none | Cast device to keep the dashboard on (`192.168.1.40` or `host:port`) |
| `DAYCAST_CAST_APP_ID` | none | App ID of the registered receiver |

## Development

```bash
go test ./...                 # Go tests
go vet ./...
cd web && npm run check       # Svelte and TypeScript checks
cd web && npm run dev         # frontend with hot reload, proxies /api to :8420
```

The code follows [CODE_GUIDELINES.md](CODE_GUIDELINES.md).

## License

Not chosen yet (AGPL-3.0 or Apache-2.0, see the research). Until a license is
added, no rights are granted to reuse this code.
