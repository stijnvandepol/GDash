# Voortgang

Stand van zaken op 8 oktober 2026, zodat ik thuis verder kan.

## Wat er staat (v0.1, deel 1)

- **Server (Go):** Open-Meteo-weer zonder sleutel, datahub met cache, last-known-good op schijf en stale-markering, SSE-stream per display, Cast-discovery (mDNS op alle interfaces), eigen CASTV2-client en een keeper die de receiver op de TV houdt zonder een andere cast te onderbreken.
- **Frontend (Svelte 5):** display-runtime met klok en weer op een vast 16:9-canvas, offline-indicator, herverbinden zonder reload. Ongeveer 19 KB JS (gzip).
- **Receiver:** dezelfde runtime in een CAF-receiver. Krijgt data via het Cast-berichtkanaal, dus geen HTTPS op het LAN nodig.
- **Getest:** alle Go-tests (`go test ./...`), `go vet`, `gofmt`, `svelte-check` zonder fouten. In de browser: het dashboard rendert, het offline-bolletje verschijnt als de server stopt en verdwijnt weer na een herstart.

## Wat nog niet getest is

- **Cast op echte hardware.** Op het netwerk waar ik werkte (145.93.x.x) zitten geen Chromecasts, dus discovery vond niets. Dat moet thuis.
- **De Docker-image.** Docker Desktop draaide niet, dus `docker build` is nog niet uitgevoerd.
- **De race detector.** `go test -race` heeft cgo nodig en dat staat op deze Windows-machine niet aan. Draait straks in CI op Linux.

## Thuis verder: in deze volgorde

1. `git clone https://github.com/stijnvandepol/daycast.git`, dan `cd web && npm ci && npm run build && cd ..` en `go build -o daycast.exe ./cmd/daycast`.
2. `./daycast.exe discover`: vindt hij de Chromecast/Google TV? Zo niet, kijk of de Windows-firewall het binnenkomende UDP-antwoord blokkeert.
3. Receiver registreren en testen volgens [receiver.md](receiver.md). Kost eenmalig 5 dollar in de Cast Developer Console.
4. De tabel in receiver.md invullen. Belangrijkste vraag: komt ambient mode van Google TV over het dashboard heen?
5. `docker build .` een keer draaien om de Dockerfile te controleren.

## Open beslissingen

- **Relaunch-beleid** (`shouldRelaunch` in `internal/cast/keeper.go`): nu neemt Daycast het scherm alleen over als er niets anders draait. Twee dingen moet ik nog beslissen. Wat als de TV in standby staat (casten zet hem via CEC aan, 's nachts ongewenst)? En wat als iemand op Google TV een eigen app zoals Netflix gebruikt? Die verschijnt mogelijk niet als Cast-app, dus dan zou Daycast eroverheen starten. Dat moet thuis getest worden.
- **TLS-verificatie naar Cast-apparaten:** nu uit, zoals pychromecast. Een optie is het certificaat bij de eerste verbinding onthouden (pinning).
- **Licentie:** AGPL-3.0 of Apache-2.0. De repo blijft privé tot dit gekozen is.
- **Service worker** voor browserdisplays, zodat een reload zonder server toch het laatste dashboard toont.
- **Naam:** Daycast is de werknaam. Merkencheck bij BOIP/EUIPO moet nog.

## Daarna (rest van v0.1 en v0.5)

Zie hoofdstuk 18 en 19 van het onderzoek in de repo GoogleHome-Dashboard (`docs/onderzoek/`): eerst de Cast-uitkomsten, dan SQLite, auth met wizard, koppelen van displays en de grid-editor.
