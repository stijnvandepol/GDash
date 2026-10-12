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
| GDash | the internet, HTTPS | Weather, air quality, electricity prices (EnergyZero), fuel prices (`api.anwb.nl`), news, calendars and place search |

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
    # GDash needs no root and no capabilities, and writes only to /data.
    user: "65532:65532"
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    tmpfs:
      - /tmp

volumes:
  gdash-data:
```

```bash
docker compose up -d
docker compose logs gdash
```

`docker compose ps` shows whether GDash is healthy: the image checks its own
`/healthz` every 30 seconds. The image has no shell, so the check is the
command `/gdash healthcheck`, which you can also run by hand with
`docker compose exec gdash /gdash healthcheck`.

Open `http://<server>:8420/` and create your account. From your home network
no token is needed in the first hour after GDash starts. After that hour, or
from an address outside the home network, GDash asks for the **setup token**
that its log shows; restarting GDash opens the hour again. Then choose the
location under **Settings**, add a display and pick your TV.

To build the image yourself instead, use the `compose.yaml` in this
repository, which builds from source.

**Synology:** in Container Manager, create a project, paste the file above and
start it. Host networking is allowed for projects. GDash sends its mDNS
queries from a port of its own instead of 5353, so it does not compete with
the Bonjour service of DSM.

**Docker Desktop on Windows or macOS** does not reach the LAN with multicast,
so GDash cannot find TVs there. Use the binary instead.

### Data directory permissions

The image runs GDash as user and group `65532`, never as root, and
`/data` in the image belongs to that user and can only be read by it. A
named volume that Docker fills from the image gets this by itself, so the
file above needs nothing more. GDash has always run as this user in the
image, so volumes of earlier versions are fine too.

If `/data` belongs to someone else, GDash stops at the start with `data
directory "/data" is not writable`. That happens with a folder you mount
yourself (`./gdash-data:/data`), with a volume that was filled by a different
image, or with data copied from a setup that ran as root. Give the data to the
GDash user once, with the container stopped:

```bash
# A folder you mount yourself:
sudo chown -R 65532:65532 ./gdash-data && sudo chmod 700 ./gdash-data

# A named volume (docker volume ls shows its name, often <folder>_gdash-data):
docker run --rm -v gdash-data:/data alpine sh -c 'chown -R 65532:65532 /data && chmod 700 /data'
```

Then start GDash again.

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
`/var/lib/gdash`, which only that user can read (systemd sets the owner by
itself, also on data moved there from an earlier setup). It also has no
capabilities, cannot see other processes or the home directories, may only
open IP, local and netlink sockets (netlink lists the network interfaces for
finding TVs), and is limited to ordinary system calls. `systemd-analyze
security gdash` shows the result. If you change `GDASH_ADDR` to a port below
1024, set both `AmbientCapabilities=` and `CapabilityBoundingSet=` to
`CAP_NET_BIND_SERVICE` in the unit. If a firewall runs on the machine, allow
TCP 8420 from the network.

## Windows

Unpack the Windows archive and start `gdash.exe`. Allow it on private
networks when Windows asks, so the TVs can reach port 8420. The data is
written to a `data` folder in the folder GDash starts in.

To start GDash with Windows, create a task in Task Scheduler: trigger
"At startup", action "Start a program" with `gdash.exe`, "Start in" set to
its folder, and "Run whether user is logged on or not".

## Home Assistant app

On Home Assistant OS or Home Assistant Supervised, GDash can run as an app
(formerly called add-on) next to Home Assistant. It uses host networking, like
the Docker setup above, so it finds the TVs by itself, and its web interface
is at `http://<home-assistant-address>:8420/`. It does not appear in Home
Assistant's sidebar: the web interface and the TV pages use addresses from
the root of the server, which the path of Home Assistant's "ingress" would
break. Home Assistant Container and Core have no apps; use Docker Compose
there.

Home Assistant needs an app repository with `repository.yaml` at the root of a
git repository. The files are in [`homeassistant/`](../homeassistant), and the
repository you add in Home Assistant is a separate public one that holds a
copy of that folder, because this repository is private for now:

```text
https://github.com/stijnvandepol/gdash-homeassistant
```

In Home Assistant, open **Settings, Apps** (older: **Add-ons**), the store,
the menu at the top right, **Repositories**, add that address, then install and
start **GDash**. The first start, pairing TVs, the Home Assistant connection
and announcements are described in
[DOCS.md of the app](../homeassistant/gdash/DOCS.md). The app is experimental:
it has not been run on a real Home Assistant yet.

## Checking a download

`SHA256SUMS` in every release lists the checksum of each archive. The release
also has a CycloneDX SBOM (`gdash_<version>_sbom.cdx.json`) with the Go modules
and npm packages the build pins; the SBOM of the Docker image, with the
packages of its base image as well, is kept with the release workflow run
(Actions, **Release**, Artifacts).

Once the repository is public, releases also carry signed build provenance
that says which commit and workflow made each file. Check it with the GitHub
CLI:

```bash
gh attestation verify gdash_<version>_linux_arm64.tar.gz --repo stijnvandepol/gdash
gh attestation verify oci://ghcr.io/stijnvandepol/gdash:<version> --repo stijnvandepol/gdash
```

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

### A backup to keep elsewhere

Settings → Account → Backup downloads one zip file with a fresh copy of the
database, `secret.key` and the photos, after asking for your password again.
It is everything needed to run GDash on another machine. Because it holds
`secret.key`, anyone with the file can read your calendar addresses and Home
Assistant token: keep it like a password. Every download is in the activity log.

To restore it: stop GDash, unzip the file into an empty data directory (for
Docker, the directory mounted at `/data`, owned by user `65532`), and start
GDash with that directory. Log in with the account and password the backup
was made with. Paired screens keep working as long as they reach GDash at the
same address.

## Security

Under Settings, Account, "Recent activity" lists logins, failed logins, pairings and
changes for 90 days, with the address they came from.

Two-step login (Settings → Account) asks for a code from an authenticator app
after the password. Its secret is stored encrypted with `secret.key`, like the
calendar addresses. A restored backup keeps it on, and your app's codes keep
working.

GDash serves plain HTTP on the LAN, because a TV that opens the dashboard
through Cast cannot be given a certificate it trusts for a LAN address. The
web interface carries your login: if you open it from outside your home
network, put a reverse proxy with HTTPS in front of it and do not expose
port 8420 to the internet. Set `GDASH_TRUSTED_PROXIES` to the proxy's
address, so login limits and the activity log see the real client and
cookies are only sent over HTTPS. Displays only receive data after they are paired.

## Troubleshooting

**"data directory is not writable" at the start.** The data directory belongs
to another user than the one GDash runs as. See
[Data directory permissions](#data-directory-permissions).

**Lost the password.** On the server, run `gdash reset-password <user>`
with the same `GDASH_DATA_DIR` as the server; in Docker:
`docker compose exec gdash /gdash reset-password <user>`. It prints a new
password, logs out every browser and notes the reset in the activity log.
Only someone who can run commands on the server can do this.

**Lost the phone with two-step login.** Log in with one of the recovery
codes you saved when you turned two-step login on, then turn it off and on
again under Settings → Account with your new phone. Without a recovery code,
`gdash reset-password <user>` (above) is the way back in: besides a new
password, it turns two-step login off. Turn it on again afterwards.

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
Settings, Data sources.

**The TV is dark outside the active hours.** That is the display's
schedule; change it under Displays, Schedule.
