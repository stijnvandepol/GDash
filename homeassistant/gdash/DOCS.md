# GDash

GDash shows your day on every TV in the house: clock, weather, calendar,
news and values from Home Assistant, cast to the Chromecast or Google TV you
already own. This app runs the GDash server next to Home Assistant, on the
same machine. No account, subscription or cloud is needed.

The full guide to GDash itself is in the
[GDash repository](https://github.com/stijnvandepol/gdash/blob/main/docs/guide.md).
This page covers what is different when GDash runs as an app.

## Before you start

- You need Home Assistant OS or Home Assistant Supervised. Apps (formerly
  called add-ons) do not exist in the Container and Core installations; there
  you run GDash with Docker Compose instead, see
  [docs/install.md](https://github.com/stijnvandepol/gdash/blob/main/docs/install.md).
- The Home Assistant machine and the TVs must be in the same network segment
  (VLAN). A guest network that isolates devices, or a switch that drops
  multicast, hides the TVs from GDash.
- The app uses **host networking**. GDash finds Cast devices with mDNS, which
  does not cross the private network of apps. This also means port **8420**
  on the Home Assistant machine belongs to GDash.

## Install

1. In Home Assistant, go to **Settings, Apps** (or **Add-ons**), open the
   store and add this repository, as the README of the repository shows.
2. Open **GDash**, choose **Install** and wait: the first install builds a
   small image on your machine, which takes a minute or two on a Raspberry Pi.
3. Choose **Start**. Turn on **Start on boot** and **Watchdog** if you want
   GDash to come back by itself.

## First start

1. Choose **Open Web UI**, or go to `http://<home-assistant-address>:8420/`,
   and create your account (at least 12 characters). From your home network
   no token is needed in the first hour after the app starts.
2. After that hour, GDash asks for the **setup token** in the **Log** tab:

   ```text
   level=WARN msg="GDash is not set up yet. Open the web interface within an hour to create the first account; …" setup_token=...
   ```

   It only works while no account exists; restarting the app opens the hour
   again.
3. Under **Settings, Data sources**, choose the location for the weather and
   add news feeds and calendars.

GDash does not appear in Home Assistant's sidebar (no "ingress"). The web
interface and the pages the TVs load use addresses that start at the root of
the server (`/api/...`, `/d/<display>`), so they cannot work under the path
Home Assistant gives an app, and the TVs have to reach GDash on port 8420
directly anyway. Open it with the button on the app page, or bookmark it.

## Pairing TVs

Under **Displays, Add a display**, GDash lists the Cast devices it found.
Pick your TV, give the display a name, such as Kitchen, and choose what it
shows. Within a few seconds the TV shows that dashboard, and GDash keeps it on
screen.

If the list says **No Cast devices found**, GDash is not in the same network
segment as the TV, or multicast is blocked. Check that the app runs with host
networking (it does by default) and see "Troubleshooting" below.

A screen without Cast, such as a tablet or a Raspberry Pi in kiosk mode, opens
`http://<home-assistant-address>:8420/d/<display>` and shows a code. Enter that
code at the display under **Displays** and choose **Connect**.

## Show Home Assistant values on a TV

GDash reads from Home Assistant with a long-lived access token and never
switches anything.

1. In Home Assistant, open your profile, **Security**, and create a
   **long-lived access token**. A separate Home Assistant user for GDash,
   without admin rights, limits what the token can see.
2. In GDash, under **Settings, Data sources, Home Assistant**, enter the
   address you open Home Assistant with, such as `http://192.168.1.10:8123`,
   and the token. GDash tries them before it saves them and stores the token
   encrypted.
3. Add a **Home Assistant** widget to a dashboard and search for the
   entities it should show.

## Announcements from Home Assistant

An automation can put a short message over the dashboard of every TV, such as
"The washing machine is done".

1. In GDash, under **Settings, Automations, API tokens**, make a token named
   "Home Assistant". Copy it: it is shown once.
2. Keep it out of your configuration, in `secrets.yaml`. The word `Bearer`
   and the space belong to the value:

   ```yaml
   # secrets.yaml
   gdash_authorization: "Bearer gdk_your_token"
   ```

3. Add a `rest_command` to `configuration.yaml` and restart Home Assistant (or
   reload the REST commands). Use the address of the Home Assistant machine
   on your network:

   ```yaml
   # configuration.yaml
   rest_command:
     gdash_announce:
       url: "http://192.168.1.10:8420/api/announcements"
       method: post
       content_type: "application/json"
       headers:
         authorization: !secret gdash_authorization
       payload: >-
         {"title": {{ title | default('') | tojson }},
          "message": {{ message | tojson }},
          "seconds": {{ seconds | default(60) }}}
   ```

4. Use it in an automation:

   ```yaml
   # automations.yaml
   - alias: "Announce that the washing machine is done"
     triggers:
       - trigger: numeric_state
         entity_id: sensor.washing_machine_power
         below: 3
         for: "00:02:00"
     actions:
       - action: rest_command.gdash_announce
         data:
           title: "Laundry"
           message: "The washing machine is done"
           seconds: 120
   ```

`message` is required (up to 280 characters of plain text), `title` is
optional (up to 60), `seconds` is how long it stays (5 to 3600) and `displays`
may list display IDs such as `["kitchen"]`. If nothing shows up, the token
list in GDash says **Never used** when the request does not arrive. In
Developer tools, Actions, run the command and read its answer: `401` means the
token is wrong, `422` names the field to correct, and `429` means more than 30
announcements in a minute. More in the
[guide](https://github.com/stijnvandepol/gdash/blob/main/docs/guide.md#announcements-from-home-assistant-or-any-automation).

## Options

Almost everything is set in the GDash web interface. The app has two
options, both for special cases; leave them empty otherwise.

| Option | Meaning |
|---|---|
| `cast_app_id` | The 8-character ID (capitals and digits, such as `ABCD1234`) of a Cast receiver you registered yourself, see [docs/receiver.md](https://github.com/stijnvandepol/gdash/blob/main/docs/receiver.md). Empty: GDash uses the public DashCast receiver. |
| `trusted_proxies` | Addresses or ranges of a reverse proxy in front of GDash, separated by commas, such as `192.168.1.5`. Only needed if you open GDash through a proxy with HTTPS; empty means the connection is used as it arrives. |

The location for the weather is not an option: GDash asks for it in its own
Settings, with a place search.

The time zone follows Home Assistant's.

## Data, backups and updates

- GDash keeps everything in the app's data folder: the database, the key that
  encrypts calendar addresses and the Home Assistant token, your photos and
  daily copies of the database. Home Assistant backups include it. The backup
  stops the app for a moment, so the database is not copied while it is being
  written.
- A backup holds `secret.key`, so keep it like a password. Anyone with it can
  read your calendar addresses and your Home Assistant token.
- GDash can also download its own backup under **Settings, Account, Backup**.
- To update, install the new version of the app. GDash writes a copy of the
  database before it changes it.

## Security

GDash serves plain HTTP on your local network, because a TV that opens the
dashboard through Cast cannot be given a certificate it trusts for a LAN
address. Do not forward port 8420 to the internet. If you open GDash from
outside your home, put a reverse proxy with HTTPS in front of it and set
`trusted_proxies`. GDash runs as an unprivileged user inside the app, and
Displays only receive data after they are paired. More in
[SECURITY.md](https://github.com/stijnvandepol/gdash/blob/main/SECURITY.md).

## Troubleshooting

**The app does not install, "manifest unknown" or "denied".** The first
install downloads the GDash program from `ghcr.io/stijnvandepol/gdash` for
the version of the app. Check that the Home Assistant machine reaches the
internet, and report an issue if the version does not exist.

**The app stops at once after Start.** Read the Log tab. GDash says what is
wrong, for example an invalid `cast_app_id`.

**The web interface does not open.** Another program on the Home Assistant
machine may use port 8420; the log then says the address is in use.

**"No Cast devices found."** See "Pairing TVs" above.

**The TV shows nothing, or "Connecting to GDash...".** The TV cannot reach
port 8420 on the Home Assistant machine: check for a firewall or a network
that separates the TV from it.

**Lost the password.** Run this on the machine that runs Home Assistant, with
access to its host (on Home Assistant OS the advanced SSH access on port
22222; the normal SSH app has no `docker` command):

```bash
docker ps --filter name=gdash          # shows the container, addon_<id>_gdash
docker exec -u 65532 addon_<id>_gdash /usr/bin/gdash reset-password <user>
```

It prints a new password and logs out every browser.

More answers are in the troubleshooting section of
[docs/install.md](https://github.com/stijnvandepol/gdash/blob/main/docs/install.md#troubleshooting).

## License

GDash is free software under the GNU Affero General Public License v3.0.
