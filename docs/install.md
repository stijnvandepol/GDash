# Installing GDash

GDash runs on any machine that is on all the time and on the same network
as your TVs: a NAS, a Raspberry Pi, a mini PC or a home server. It needs no
cloud account and no extra hardware at the TV.

## What the network needs

| From | To | Why |
|---|---|---|
| GDash | TVs and Cast devices, TCP 8009 | Start and keep the dashboard on screen |
| GDash | the network, UDP 5353 multicast | Find Cast devices (mDNS) |
| TVs and browsers | GDash, TCP 8420 | Load the dashboard and receive updates |
| GDash | the internet, HTTPS | Weather, air quality, news, calendars and place search |

GDash and the TVs must be in the same network segment (VLAN). A guest
network that isolates devices, or a switch that drops multicast, hides the
TVs from GDash.

## Docker Compose (Linux, NAS, Raspberry Pi)

The image runs on amd64 and arm64 (Raspberry Pi 3, 4 and 5 with a 64-bit
OS). Host networking is needed: mDNS does not cross Docker's bridge network.

```yaml
services:
  gdash:
    image: ghcr.io/stijnvandepol/gdash:latest
    network_mode: host
    environment:
      TZ: "Europe/Amsterdam"
    volumes:
      - gdash-data:/data
    restart: unless-stopped

volumes:
  gdash-data:
```

```bash
docker compose up -d
docker compose logs gdash
```

The log of the first start shows a **setup token**. Open
`http://<server>:8420/`, enter it and create your account. Then choose the
location under **Settings**, add a display and pick your TV.

To build the image yourself instead, use the `compose.yaml` in this
repository, which builds from source.

**Synology:** in Container Manager, create a project, paste the file above and
start it. Host networking is allowed for projects. GDash sends its mDNS
queries from a port of its own instead of 5353, so it does not compete with
the Bonjour service of DSM.

**Docker Desktop on Windows or macOS** does not reach the LAN with multicast,
so GDash cannot find TVs there. Use the binary instead.

## Binary with systemd (Linux, Raspberry Pi)

Every release has archives for Linux on amd64, arm64 and armv7 (32-bit
Raspberry Pi OS), with a checksum file.

```bash
sha256sum --check --ignore-missing SHA256SUMS
tar -xzf gdash_<version>_linux_arm64.tar.gz
cd gdash_<version>_linux_arm64
sudo install -m 755 gdash /usr/local/bin/gdash
sudo install -m 644 gdash.service /etc/systemd/system/gdash.service
sudo systemctl daemon-reload
sudo systemctl enable --now gdash
journalctl -u gdash
```

The unit runs GDash as a throwaway user that can only write its data in
`/var/lib/gdash`. If a firewall runs on the machine, allow TCP 8420 from
the network.

## Windows

Unpack the Windows archive and start `gdash.exe`. Allow it on private
networks when Windows asks, so the TVs can reach port 8420. The data is
written to a `data` folder in the folder GDash starts in.

To start GDash with Windows, create a task in Task Scheduler: trigger
"At startup", action "Start a program" with `gdash.exe`, "Start in" set to
its folder, and "Run whether user is logged on or not".

## Updating

With Docker: `docker compose pull && docker compose up -d`. With the binary:
replace it and restart the service. A new version that changes the database
first writes a copy of it, `gdash.db.bak-v<N>`, next to the original.

## Backups

GDash copies its database once a day to `backups/` in the data directory
and keeps the last seven copies, named by date. They undo a mistake or a
failed upgrade, but they sit on the same disk: copy the whole data directory
to another machine now and then as well.

The data directory holds the database (`gdash.db`), the key that encrypts
calendar addresses (`secret.key`), the daily copies (`backups`), your photos
(`photos`) and the last fetched data (`cache`). Without `secret.key` a restored database still
works, but its calendars have to be added again.

To restore a daily copy: stop GDash, remove `gdash.db`, `gdash.db-wal`
and `gdash.db-shm`, copy the backup to `gdash.db` and start GDash.

## Security

Under Settings, "Recent activity" lists logins, failed logins, pairings and
changes for 90 days, with the address they came from.

GDash serves plain HTTP on the LAN, because a TV that opens the dashboard
through Cast cannot be given a certificate it trusts for a LAN address. The
web interface carries your login: if you open it from outside your home
network, put a reverse proxy with HTTPS in front of it and do not expose
port 8420 to the internet. Set `GDASH_TRUSTED_PROXIES` to the proxy's
address, so login limits and the activity log see the real client and
cookies are only sent over HTTPS. Displays only receive data after they are paired.

## Troubleshooting

**"No Cast devices found."** GDash is not in the same network segment as
the TV, multicast is blocked, or the container has no host networking. Run
`gdash discover` on the server to see what it finds; in Docker:
`docker compose exec gdash /gdash discover`.

**The TV shows nothing, or "Connecting to GDash…".** The TV cannot reach
the server on port 8420: check the firewall on the server. GDash tells the
TV the server address on the TV's own subnet; a VPN such as Tailscale on the
server is fine.

**The dashboard disappears after about 19 minutes on Google TV.** The
screensaver covers the cast. GDash restarts the cast every 15 minutes
against this; if it still happens, check the server log for cast errors.

**The calendar says it cannot be read on this server.** The database was
moved without its `secret.key`. Remove the calendar and add it again under
Settings.

**The TV is dark outside the active hours.** That is the display's
schedule; change it under Displays, Schedule.
