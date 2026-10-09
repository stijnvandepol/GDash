# API

All endpoints are under `/api` and exchange JSON. Routes marked "admin" need
the session cookie (`nookboard_session`) that `POST /api/setup` and
`POST /api/sessions` set. Display streams need a paired screen's cookie
(`nookboard_display`) or an admin session.
The handlers in `internal/api` document each endpoint where it is defined;
this page is the overview.

## Errors

Every error has the same shape:

```json
{"error": {"code": "invalid_input", "message": "Some fields are not valid.",
           "fields": [{"field": "name", "message": "use 1 to 60 characters"}]}}
```

`fields` is only present for `422`. Request bodies must be one JSON object of
at most 64 KiB; unknown fields are rejected with `400 invalid_json`.

Browser requests that change something are rejected with `403` when they
come from another site (cross-site request forgery protection).

## Endpoints

| Method and path | Needs | Success | Errors |
|---|---|---|---|
| `GET /api/setup` | no | `200 {"needed": bool}` | |
| `POST /api/setup` `{setupToken, username, password}` | no | `201` user, sets cookie | `403 invalid_setup_token`, `409 setup_done`, `422`, `429 rate_limited` |
| `POST /api/sessions` `{username, password}` | no | `201` user, sets cookie | `401 invalid_credentials`, `429 rate_limited` |
| `DELETE /api/sessions/current` | admin | `204` | `401` |
| `GET /api/me` | admin | `200` user | `401` |
| `PUT /api/me/password` `{currentPassword, newPassword}` | admin | `204`; every other session of the account ends | `401`, `422` (wrong current or too short new password), `429 rate_limited` |
| `GET /api/system` | admin | `200 {"version"}` | `401` |
| `GET /api/audit-events?limit=100` | admin | `200 {"events": [{at, actor, action, detail, ip}]}`, newest first, at most 200 | `400`, `401` |
| `GET /api/displays` | admin | `200 {"displays": [display + pairedScreens + cast?]}` | `401` |
| `POST /api/displays` `{id, name, castDeviceId, activeFrom?, activeUntil?, schedule?}` | admin | `201` display | `401`, `409 conflict`, `422` |
| `PATCH /api/displays/{id}` `{name?, castDeviceId?, dashboardId?, activeFrom?, activeUntil?, schedule?, rotation?, rotateSeconds?, textScale?}` | admin | `200` display | `401`, `404 not_found`, `422` (also for an unknown dashboard) |
| `DELETE /api/displays/{id}` | admin | `204` | `401`, `404 not_found` |
| `GET /api/widget-types` | admin | `200 {columns, rows, themes, types: {name: {minW, minH, settings: {key: {kind, min?, max?, required}}}}}` | `401` |
| `GET /api/dashboard-templates` | admin | `200 {"templates": [{id, name, description, theme, widgets}]}`; create a dashboard from one with `POST /api/dashboards` and its `theme` and `widgets` | `401` |
| `GET /api/dashboards` | admin | `200 {"dashboards": [dashboard]}` | `401` |
| `POST /api/dashboards` `{id, name, theme?, widgets}` | admin | `201` dashboard | `401`, `409 conflict`, `422` |
| `GET /api/dashboards/{id}` | admin | `200` dashboard | `401`, `404` |
| `PUT /api/dashboards/{id}` `{name, theme?, widgets}` | admin | `200` dashboard; screens update at once | `401`, `404`, `422` |
| `DELETE /api/dashboards/{id}` | admin | `204`; its displays fall back to the default | `401`, `404`, `409` for `default` |
| `GET /api/dashboards/{id}/preview?textScale=` | admin | `200` snapshot of the stored dashboard with the data a screen gets now; no display needed | `401`, `404`, `422` for a `textScale` other than 100, 125, 150 or 200 |
| `POST /api/dashboards/{id}/preview?textScale=` `{name, theme?, widgets}` | admin | `200` snapshot of that unsaved layout; nothing is stored and no screen changes | `401`, `404` (the dashboard must exist), `422` |
| `GET /api/settings` | admin | `200 {"settings": settings, "status": {key: {updatedAt?, failing}}}` | `401` |
| `PUT /api/settings/location` `{name, latitude, longitude}` | admin | `200` location | `401`, `422` |
| `PUT /api/settings/language` `{language}` | admin | `200`; screens switch at once | `401`, `422` for a language other than `en` or `nl` |
| `DELETE /api/settings/location` | admin | `204`; weather widgets show that no location is set | `401` |
| `GET /api/places?name=Oss` | admin | `200 {"places": [place]}` | `401`, `422`, `502 upstream_unavailable` |
| `POST /api/news-feeds` `{name, url}` | admin | `201` news feed | `401`, `409` at 5 feeds, `422` |
| `DELETE /api/news-feeds/{id}` | admin | `204` | `401`, `404` |
| `POST /api/calendars` `{name, url}` | admin | `201` calendar, without its address | `401`, `409` at 10 calendars, `422` |
| `PATCH /api/calendars/{id}` `{color}` | admin | `204` | `401`, `404`, `422` for another colour |
| `DELETE /api/calendars/{id}` | admin | `204` | `401`, `404` |
| `PUT /api/homeassistant` `{url, token}` | admin | `200 {"url": …}` after Home Assistant accepted the token | `401`, `422` naming `url` or `token` |
| `DELETE /api/homeassistant` | admin | `204`; Home Assistant widgets show that it is not connected | `401` |
| `GET /api/homeassistant/entities` | admin | `200 {"entities": [entity]}`, sorted by ID | `401`, `409 not_connected`, `502 upstream_unavailable` |
| `GET /api/photos` | admin | `200 {"photos": [{name, size, addedAt}]}`, oldest first | `401` |
| `POST /api/photos` multipart, field `photo` | admin | `201` photo | `401`, `409` at 50 photos, `422` for another kind of file or one over 8 MB |
| `DELETE /api/photos/{name}` | admin | `204` | `401`, `404` |
| `GET /api/photos/{name}` | admin or any paired screen | `200` the image | `404`, also for anyone else |
| `GET /api/cast-devices` | admin | `200 {"devices": [device]}` | `401`, `500 discovery_failed` |
| `GET /api/displays/{id}/events` | paired screen or admin | `text/event-stream`, `snapshot` events | `401 not_paired`, `404` for an invalid id |
| `POST /api/pairing-requests` `{displayId}` | no | `201 {code, pollToken, expiresAt}` | `404`, `429 rate_limited` |
| `POST /api/display-sessions` `{pollToken}` or `{launchToken}` | no | `201 {displayId}`, sets `nookboard_display`; `202` while pending | `400`, `401 invalid_token` |
| `GET /api/display-sessions/current` | paired screen | `200 {displayId}` | `401 not_paired` |
| `POST /api/displays/{id}/pairings` `{code}` | admin | `201 {displayId}` | `401`, `422 invalid_code` |
| `DELETE /api/displays/{id}/pairings` | admin | `200 {removed}` | `401` |

## Pairing

A display page that is not paired asks `POST /api/pairing-requests` for a code
and shows it. An admin enters the code in the web interface, which calls
`POST /api/displays/{id}/pairings`. Meanwhile the page polls
`POST /api/display-sessions {pollToken}` every few seconds; once approved it
gets the `nookboard_display` cookie. Codes are valid for 10 minutes.

A page opened by a cast keeper carries a single-use `launch` token in its URL,
valid for 2 minutes, and exchanges it with
`POST /api/display-sessions {launchToken}`. A display keeps one such cast token,
replaced on every launch.

Open streams re-check their access every 20 seconds, so unpairing stops a
screen within that time. Deleting a display unpairs all its screens.

## Settings

The weather location, news feeds and calendars are managed here instead of
in environment variables. On the very first start Nookboard copies
`NOOKBOARD_LATITUDE`, `NOOKBOARD_LONGITUDE`, `NOOKBOARD_NEWS_FEED` (default: NOS)
and `NOOKBOARD_CALENDAR_URLS` into the settings; later starts ignore them.

A calendar's iCal address gives read access to everyone who has it. It is
stored encrypted with `secret.key` in the data directory and never returned
by the API: the web interface only sees its name and host. A calendar whose
address cannot be decrypted (the database was moved without its key) is
reported with `"unreadable": true` and skipped until it is added again.

Feeds and calendars may point at the LAN or at localhost, so a Nextcloud or
Radicale at home works. Link-local addresses (169.254.0.0/16, fe80::/10)
and AWS's IPv6 metadata address (fd00:ec2::254), where cloud hosts serve
instance metadata, are refused when connecting, also in their NAT64 form.

Home Assistant is connected with its address and a long-lived access token,
made in Home Assistant under the user's profile, Security. Nookboard tries the
connection before storing it, stores address and token encrypted together,
and never returns or logs the token. It only reads: once a minute it asks
`/api/states/<entity>` for each entity that a Home Assistant widget shows,
at most 40. The token gives the rights of the Home Assistant user who made
it, so a separate, non-admin user for Nookboard is the safer choice.

Changes apply at once: a changed source is fetched again, a removed one
disappears from the displays. `status` has an entry per configured source by
data key (`weather.current`, `news.default`, `calendar.default`,
`homeassistant.default`).

Place search sends the typed name to Open-Meteo's geocoding service, which
uses GeoNames data; the coordinates can also be entered by hand.

## Objects

**user:** `{"id": 1, "username": "stijn"}`

**display:** `{"id": "kitchen", "name": "Kitchen TV", "castDeviceId": "e542…", "dashboardId": "", "activeFrom": "06:30", "activeUntil": "23:00", "schedule": [slot], "createdAt": "2026-10-08T17:00:00Z"}`.
`id` is 1 to 32 lowercase letters, digits or dashes and is fixed after creation;
`castDeviceId` is empty for browser and kiosk displays; an empty `dashboardId` means the default dashboard.
`activeFrom`/`activeUntil` are HH:MM in the server's time zone (`TZ`), may wrap past midnight, and are both empty for
always on. Outside them the page is dark, the snapshot has no data and the cast is stopped.
`schedule` holds at most 6 slots and is replaced as a whole by a `PATCH`; it is always a list, empty when unused.
`rotation` lists up to 5 dashboard ids shown in turn with the display's own, each for `rotateSeconds` (10 to 600),
while no slot applies; empty with `rotateSeconds` 0 means no rotation. Deleting a dashboard removes it from every
rotation. A rotating snapshot carries `pages` (the display's own dashboard first) and `pageSeconds`; the screen turns
the page on its clock, so every screen of a display shows the same page.

**cast:** `{"state": "showing", "since": "2026-10-09T08:31:00Z", "problem"?: "…"}`, only for a display on a Cast
device. `state` is `connecting`, `showing`, `standby` (the receiver runs but the TV sleeps), `other_app` (someone else
is casting; Nookboard waits), `resting` (outside active hours) or `unreachable`, with `problem` saying why.

`textScale` is the display's text size in percent: 100 (default), 125, 150 or 200. The snapshot carries it; the
screen draws its stage narrower by that factor and in the screen's own shape (between 4:3 and 21:9), so a small
screen shows the same layout larger and without bars. Widgets then show fewer list items.

**slot:** `{"from": "06:30", "until": "09:00", "days": ["mon", "tue"], "dashboardId": "morning"}`. During the slot the
display shows that dashboard instead of its own; the first slot that contains the current time wins. `days` holds
`mon` to `sun`; empty means every day. A slot past midnight belongs to the day it starts, so Friday 22:00 to 01:00
still applies at 00:30 on Saturday. Times follow the same rules as
active hours, but both are required. Deleting a dashboard removes the slots that use it. Field errors are named
`schedule[i]`, and `schedule` for a slot with an unknown dashboard.

**dashboard:** `{"id": "default", "name": "Default", "columns": 24, "rows": 14, "theme": "minimal-dark", "widgets": [widget]}`.
`theme` is one of `minimal-dark` (default), `minimal-light`, `glass`, `warm-light`, `high-contrast` and `night`; an unknown theme is a `422`. The grid size and
every widget's `source` are set by the server; values sent by a client are ignored.

**widget:** `{"id": "news", "type": "news", "x": 10, "y": 9, "w": 14, "h": 5, "source": "news.default", "settings": {"maxItems": 3}}`.
A layout is rejected with field errors such as `widgets[2]` when a widget is outside the grid, smaller than its type
allows, overlaps another, lacks a required setting, or has a setting its type does not take.

| Type | Minimum | Data | Settings |
|---|---|---|---|
| `clock` | 4x3 | none | |
| `weather` | 4x4 | `weather.current` | |
| `forecast` | 6x4 | `weather.current` | `days`: 1 to 7, default 5 |
| `sun` | 5x4 | `weather.current` | |
| `news` | 6x4 | `news.default` | `maxItems`: 1 to 20 |
| `calendar` | 6x4 | `calendar.default` | `maxItems`: 1 to 20 |
| `text` | 4x3 | none | `text`: 1 to 300 characters, line breaks allowed (required) |
| `photos` | 4x3 | `photos.default` | `seconds`: 5 to 600 per photo, default 30 |
| `countdown` | 4x4 | none | `label`: 1 to 60 characters, `date`: `YYYY-MM-DD` (both required) |
| `homeassistant` | 4x3 | `homeassistant.default` | `entities`: 1 to 8 entity IDs such as `sensor.living_room_temperature` (required) |

A setting's `kind` is `integer`, `text`, `date` or `entities`; `min` and `max` bound an integer, a text's length in
characters, or the number of entities.
Text settings are shown as plain text on the screen, never as HTML.

**settings:** `{"language": "nl", "location": {"name": "Oss", "latitude": 51.77, "longitude": 5.52} | null, "newsFeeds": [{"id": 1, "name": "NOS", "url": "https://…"}], "calendars": [{"id": 2, "name": "Family", "host": "calendar.google.com", "color": "blue", "unreadable": false}]}`.
A calendar's `color` is one of `blue`, `green`, `orange`, `purple`, `red`, `teal`, `pink` and `yellow`; until one is
chosen it follows the calendar's place in the list. Its events carry the colour to the TV.
Names are 1 to 60 characters; addresses use http or https, and calendars also webcal.
Several news feeds are merged into one list, newest first, each headline with `source` set to its feed's name and
an empty `title`, which the screen fills in in its own language.

**place:** `{"name": "Oss", "region": "North Brabant, Netherlands", "latitude": 51.765, "longitude": 5.518}`

**settings.homeAssistant:** `{"url": "http://192.168.1.10:8123", "unreadable": false}`, or `null` when not connected.

**entity:** `{"id": "sensor.living_room_temperature", "name": "Living room", "state": "21.4", "unit": "°C", "deviceClass": "temperature"}`.
The `homeassistant.default` data on a screen maps the entity IDs that its own widgets show to the same object
without `id`; an entity Home Assistant does not know is left out.

**device:** `{"id": "e542…", "name": "tv woonkamer", "model": "Chromecast HD", "addr": "192.168.1.160:8009", "hasScreen": true}`

**snapshot event:** `{"type": "snapshot", "version": 3, "serverTime": "…", "language": "nl", "dashboard": {…}, "data": {"weather.current": {"value": {…}, "fetchedAt": "…", "staleAfter": "…", "failing": false}}}`.
Each display gets the dashboard for the current time (its schedule or its own), and `data` only holds the sources its widgets use. `active` is false outside the
display's active hours; `data` is then empty.
A preview snapshot has the same shape: `active` is true, the household's `language` and the chosen `textScale`
(default 100) are set, there are no `pages`, and `data` holds the sources of that one dashboard. Its `version` is a
hash of the content, so a poller sees whether anything changed. Data a draft needs that no saved dashboard uses yet
(a new Home Assistant entity) is absent until the dashboard is saved. The web interface polls the preview every few
seconds rather than opening a stream: a preview shows one dashboard that may be a draft, which the per-display
event stream cannot do, and polling needs no connection kept open per editor.
The page `GET /preview/{id}` (optionally `?textScale=150`) shows that preview full screen, drawn by the same code
as a display page; it loads without a session, like the web interface, and needs the admin session for its data.
Its content security policy is the display's, except that the web interface may show it in a frame.
See `internal/display/display.go` and `web/src/protocol.ts`.
