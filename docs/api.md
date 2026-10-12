# API

All endpoints are under `/api` and exchange JSON. Routes marked "admin" need
the session cookie (`gdash_session`) that `POST /api/setup` and
`POST /api/sessions` set. Display streams need a paired screen's cookie
(`gdash_display`) or an admin session. `POST /api/announcements` also takes
an API token, see [Announcements](#announcements).
The handlers in `internal/api` document each endpoint where it is defined;
this page is the overview.

## Errors

Every error has the same shape:

```json
{"error": {"code": "invalid_input", "message": "Some fields are not valid.",
           "fields": [{"field": "name", "message": "use 1 to 60 characters"}]}}
```

`fields` is only present for `422`. Request bodies must be one JSON object of
at most 64 KiB, sent as `Content-Type: application/json` (otherwise
`415 unsupported_media_type`); unknown fields are rejected with
`400 invalid_json`. API answers carry `Cache-Control: no-store`.

Browser requests that change something are rejected with `403` when they
come from another site (cross-site request forgery protection).

## Endpoints

| Method and path | Needs | Success | Errors |
|---|---|---|---|
| `GET /api/setup` | no | `200 {"needed": bool}` | |
| `POST /api/setup` `{setupToken, username, password}` | no | `201` user, sets cookie | `403 invalid_setup_token`, `409 setup_done`, `422`, `429 rate_limited` |
| `POST /api/sessions` `{username, password}` | no | `201` user, sets cookie; with two-step login on `202 {"pending"}` and no cookie | `401 invalid_credentials`, `429 rate_limited` |
| `POST /api/sessions/two-step` `{pending, code}` | no | `201` user, sets cookie; `code` is a 6-digit code from the app or a recovery code | `401 invalid_code` (try again), `401 login_expired` (after 5 minutes or 5 codes: password again), `429 rate_limited` (with `Retry-After` when wrong codes make the next one wait), `429 two_step_locked` (app codes refused, see below) |
| `DELETE /api/sessions/current` | admin | `204` | `401` |
| `GET /api/me` | admin | `200` user | `401` |
| `PUT /api/me/password` `{currentPassword, newPassword}` | admin | `204`; every other session of the account ends | `401`, `422` (wrong current or too short new password), `429 rate_limited` |
| `GET /api/me/two-step` | admin | `200 {"enabled", "recoveryCodesLeft"}` | `401` |
| `POST /api/me/two-step/enrolment` `{password}` | admin | `201 {secret, uri, qr}`: the new secret in base32, as an `otpauth://` address and as an SVG QR code; never returned again. Two-step login stays off until the next call | `401`, `409 two_step_on`, `422` (wrong password), `429 rate_limited` |
| `PUT /api/me/two-step` `{code}` | admin | `200 {"recoveryCodes": [10 codes]}`, shown this once; two-step login is on and every other session of the account ends | `401`, `409 no_two_step_setup`, `two_step_setup_replaced` (another setup replaced the secret meanwhile: start again) or `two_step_on`, `422` (wrong code), `429 rate_limited` (wrong codes wait as for the login; at the stop the setup has to start again) |
| `DELETE /api/me/two-step` `{password, code}` | admin | `204`; `code` may be a recovery code | `401`, `409 two_step_off`, `422` (wrong password or code), `429 rate_limited` or `two_step_locked` (as for the login) |
| `POST /api/backup` `{password}` | admin | `200 application/zip` attachment with `gdash.db`, `secret.key` and `photos/`; recorded in the audit log | `401`, `422` (wrong password), `429 rate_limited` |
| `GET /api/system` | admin | `200 {"version"}` | `401` |
| `GET /api/audit-events?limit=100` | admin | `200 {"events": [{at, actor, action, detail, ip}]}`, newest first, at most 200 | `400`, `401` |
| `GET /api/api-tokens` | admin | `200 {"tokens": [{id, name, createdAt, lastUsedAt}]}`, oldest first, never the secrets | `401` |
| `POST /api/api-tokens` `{name}` | admin | `201` token with its secret in `token`, shown this once | `401`, `409` at 10 tokens, `422` |
| `DELETE /api/api-tokens/{id}` | admin | `204`; automations that use it get `401` from then on | `401`, `404` |
| `POST /api/announcements` `{message, title?, seconds?, displays?}` | API token or admin | `201 {title?, message, expiresAt, displays}` | `400`, `401 invalid_token` or `not_logged_in`, `422`, `429 rate_limited` |
| `GET /api/displays` | admin | `200 {"displays": [display + pairedScreens + cast?]}` | `401` |
| `POST /api/displays` `{id, name, castDeviceId, activeFrom?, activeUntil?, schedule?}` | admin | `201` display | `401`, `409 conflict`, `422` |
| `PATCH /api/displays/{id}` `{name?, castDeviceId?, dashboardId?, activeFrom?, activeUntil?, schedule?, rotation?, rotateSeconds?, textScale?}` | admin | `200` display | `401`, `404 not_found`, `422` (also for an unknown dashboard) |
| `DELETE /api/displays/{id}` | admin | `204` | `401`, `404 not_found` |
| `POST /api/displays/{id}/cast-takeovers` | admin | `204`: the display's cast keeper shows the dashboard on the TV within seconds, also over what someone is watching, and stops waiting (`yielded`); recorded in the audit log as `cast_taken_over`. Not while the TV is in standby or outside active hours | `401`, `404 not_found`, `409 not_cast` (no Cast device) or `not_connected` (GDash is still connecting to the TV) |
| `GET /api/widget-types` | admin | `200 {columns, rows, themes, fonts, types: {name: {minW, minH, settings: {key: {kind, min?, max?, required}}}}}` | `401` |
| `GET /api/dashboard-templates` | admin | `200 {"templates": [{id, name, description, theme, widgets}]}`; create a dashboard from one with `POST /api/dashboards` and its `theme` and `widgets` | `401` |
| `GET /api/dashboards` | admin | `200 {"dashboards": [dashboard]}` | `401` |
| `POST /api/dashboards` `{id, name, theme?, font?, nightTheme?, widgets}` | admin | `201` dashboard | `401`, `409 conflict`, `422` |
| `GET /api/dashboards/{id}` | admin | `200` dashboard | `401`, `404` |
| `PUT /api/dashboards/{id}` `{name, theme?, font?, nightTheme?, widgets}` | admin | `200` dashboard; screens update at once | `401`, `404`, `422` |
| `DELETE /api/dashboards/{id}` | admin | `204`; its displays fall back to the default | `401`, `404`, `409` for `default` |
| `GET /api/dashboards/{id}/preview?textScale=` | admin | `200` snapshot of the stored dashboard with the data a screen gets now, in its night theme after sunset; no display needed | `401`, `404`, `422` for a `textScale` other than 100, 125, 150 or 200 |
| `POST /api/dashboards/{id}/preview?textScale=` `{name, theme?, font?, nightTheme?, widgets}` | admin | `200` snapshot of that unsaved layout; nothing is stored and no screen changes | `401`, `404` (the dashboard must exist), `422` |
| `GET /api/settings` | admin | `200 {"settings": settings, "status": {key: {updatedAt?, failing}}}` | `401` |
| `PUT /api/settings/location` `{name, latitude, longitude}` | admin | `200` location | `401`, `422` |
| `PUT /api/settings/language` `{language}` | admin | `200`; screens switch at once | `401`, `422` for a language other than `en`, `nl`, `de` or `fr` |
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
| `PUT /api/homey` `{url, key}` | admin | `200 {"url": …}` after Homey accepted the API key | `401`, `422` naming `url` or `key` |
| `DELETE /api/homey` | admin | `204`; Homey widgets show that it is not connected | `401` |
| `GET /api/homey/capabilities` | admin | `200 {"capabilities": [capability]}`, every readable value of every device, sorted by zone, device and title | `401`, `409 not_connected`, `502 upstream_unavailable` |
| `GET /api/homey/users` | admin | `200 {"users": [{id, name}]}`, sorted by name, without whether they are home | `401`, `409 not_connected`, `502 upstream_unavailable` (also when the key lacks View users) |
| `GET /api/departures/stops?q=utrecht+cs` | admin | `200 {"stops": [stop]}`, at most 20, best matches first | `401`, `422` naming `q` (2 to 100 characters), `502 upstream_unavailable` |
| `GET /api/custom-sources` and the other custom API routes | admin | see [Custom APIs](#custom-apis) | |
| `GET /api/photos` | admin | `200 {"photos": [{name, size, addedAt}]}`, oldest first | `401` |
| `POST /api/photos` multipart, field `photo` | admin | `201` photo | `401`, `409` at 50 photos, `422` for another kind of file or one over 8 MB |
| `DELETE /api/photos/{name}` | admin | `204` | `401`, `404` |
| `GET /api/photos/{name}` | admin or any paired screen | `200` the image | `404`, also for anyone else |
| `GET /api/cast-devices` | admin | `200 {"devices": [device]}` | `401`, `500 discovery_failed` |
| `GET /api/displays/{id}/events` | paired screen or admin | `text/event-stream`, `snapshot` events | `401 not_paired`, `404` for an invalid id |
| `POST /api/pairing-requests` `{displayId}` | no | `201 {code, pollToken, expiresAt}` | `404`, `429 rate_limited` |
| `POST /api/display-sessions` `{pollToken}` or `{launchToken}` | no | `201 {displayId}`, sets `gdash_display`; `202` while pending | `400`, `401 invalid_token` |
| `GET /api/display-sessions/current` | paired screen | `200 {displayId}` | `401 not_paired` |
| `POST /api/displays/{id}/pairings` `{code}` | admin | `201 {displayId}` | `401`, `422 invalid_code` |
| `DELETE /api/displays/{id}/pairings` | admin | `200 {removed}` | `401` |

## Two-step login

Codes follow RFC 6238 with the settings every authenticator app supports:
HMAC-SHA-1, 6 digits, a new code every 30 seconds. The code of the current
30 seconds and of the ones just before and after is accepted, each only
once. A recovery code is 16 letters and digits in groups of four; white
space (also tabs and non-breaking spaces), dashes and case do not matter,
and each works once.

Wrong codes count towards the same limits as wrong passwords (5 per 15
minutes per address, IPv6 per /64, and 50 per 15 minutes per account), and
a right password does not clear them, also not when it is confirmed again
for a backup or a password change. On top of that, codes from the app count
per account, over all addresses:

- A pending login ends after 5 codes; the password has to be entered again.
- The first 5 wrong codes in a row cost nothing. After that the next code
  waits 2, 4, 8, 16 and 32 minutes, then an hour each time (`429
  rate_limited` with `Retry-After`), so at most 10 wrong codes fit in any
  hour.
- After 50 wrong codes in a row, codes from the app are refused, also right
  ones (`429 two_step_locked`), until a recovery code is used or
  `gdash reset-password` runs. That is reached after about 40 hours of
  guessing as fast as allowed, and gives a guesser at most a 0.015% chance
  (NIST SP 800-63B allows up to 100 failures in a row). Reaching it is in
  the audit log as `two_step_locked`.
- Only a right code, a recovery code, a new setup, turning two-step login
  off or `gdash reset-password` ends a run of wrong codes. The count is kept
  in the database, so a restart does not end it.
- Codes are only checked after the right password, so nobody without it can
  make the owner wait. Recovery codes (80 bits) do not wait and do not count
  here, so they always work; only the limits per address and account apply.

The first code of a new setup counts the same way; at the stop, the setup
starts again with the password. `gdash reset-password` turns two-step login
off.

## Pairing

A display page that is not paired asks `POST /api/pairing-requests` for a code
and shows it. An admin enters the code in the web interface, which calls
`POST /api/displays/{id}/pairings`. Meanwhile the page polls
`POST /api/display-sessions {pollToken}` every few seconds; once approved it
gets the `gdash_display` cookie. Codes are valid for 10 minutes.

A page opened by a cast keeper carries a single-use `launch` token in its URL,
valid for 2 minutes, and exchanges it with
`POST /api/display-sessions {launchToken}`. A display keeps one such cast token,
replaced on every launch.

`DELETE /api/displays/{id}/pairings` unpairs the screens paired with a code;
the TV a cast keeper opened stays paired, and `removed` does not count it. Open
streams re-check their access every 20 seconds, so unpairing stops a screen
within that time. Deleting a display unpairs all its screens, the TV as well.

## Announcements

An automation, such as a Home Assistant `rest_command`, posts a short message
that every display (or the ones you name) shows over its dashboard for a while:

```sh
curl -X POST http://gdash.local:8420/api/announcements \
  -H "Authorization: Bearer $GDASH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title": "Laundry", "message": "The washing machine is done", "seconds": 60}'
```

| Field | Rules |
|---|---|
| `message` | required, 1 to 280 characters, plain text without line breaks or other control characters |
| `title` | optional, up to 60 characters, same rules |
| `seconds` | optional, 5 to 3600, default 60: how long it stays on screen |
| `displays` | optional list of display IDs, default all displays. An unknown ID is a `422`, and so is an empty list |

Leading and trailing spaces are trimmed. The answer is `201` with the message
as the screens get it: `{"title": "Laundry", "message": "…", "expiresAt": "…", "displays": ["kitchen", "hall"]}`.

**Authentication.** Make an API token under Settings, API tokens (or with
`POST /api/api-tokens`) and send it as `Authorization: Bearer gdk_…`. Only the
token's SHA-256 hash is stored, so the token is shown once. There are at most 10
tokens, and `lastUsedAt` shows when each was last used. A token is valid for this
route only: every other route answers `401` to it. Revoking a token takes effect
at once. A request with an `Authorization` header is judged on that header alone,
so a session cookie sent along never rescues a wrong token. Without the header an
admin session works as well, which is how the web interface sends its test
announcement. Twenty wrong tokens from one address block that address for 15
minutes (`429`).

**Cross-site requests.** The cross-site request forgery protection covers this route
for sessions and tokens alike: a browser request that is marked cross-site, or
that carries another `Origin`, is refused with `403`, also with a valid token.
Scripts and automations send neither header and are not affected. A web page on
another site can therefore not use the token, even if it had it.

**Delivery.** Announcements are kept in memory, so they are gone after a restart.
Each display shows the newest announcement aimed at it: a newer one replaces the
older one on the displays it names and leaves the other displays alone, so there
is no queue of messages that went stale while they waited. The snapshot carries
`announcement: {title?, message, expiresAt}` next to its `serverTime`; the screen
drops the message at `expiresAt` by itself, with the server's clock when the
device clock is far off, and the server publishes again at the expiry for
screens that connect later. A display outside its active hours gets no
announcement. Browser screens get it over the event stream and Cast screens over
the Cast channel, in the same snapshot. The message is shown as text, never as HTML.

**Limits.** A token (or admin) may post 30 announcements per minute; more is
`429 rate_limited` with `Retry-After: 60`, counted before the body is checked.
The log records who posted and to how many displays, never the token or the message.

## Settings

The weather location, news feeds and calendars are managed here instead of
in environment variables. On the very first start GDash copies
`GDASH_LATITUDE`, `GDASH_LONGITUDE`, `GDASH_NEWS_FEED` (default: NOS)
and `GDASH_CALENDAR_URLS` into the settings; later starts ignore them.

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
made in Home Assistant under the user's profile, Security. GDash tries the
connection before storing it, stores address and token encrypted together,
and never returns or logs the token. It only reads: once a minute it asks
`/api/states/<entity>` for each entity that a Home Assistant widget shows,
at most 40. The token gives the rights of the Home Assistant user who made
it, so a separate, non-admin user for GDash is the safer choice.

Homey is connected with the address of a Homey Pro (2023 or later) or Homey Self-Hosted Server and an API key made
in the Homey web app under Settings, API Keys, with View devices and View zones, and View users for Who's home
widgets. GDash tries the connection before storing it (`/api/manager/zones/zone` and `/api/manager/devices/device`
must answer as Homey's local Web API does), stores address and key encrypted together, never returns or logs the
key, sends it as `Authorization: Bearer`, and follows no redirects. It only reads: once a minute it asks
`/api/manager/devices/device` when a Homey widget shows a value and `/api/manager/users/user` when a Who's home widget
looks for someone, one request each, and keeps only what the widgets show, at most 40 values and 20 users. Homey
Cloud and Homey Bridge, which only offer OAuth through Athom's cloud, are not supported.

Departures come from OVapi (`v0.ovapi.nl`, the KV78Turbo feed of buses, trams, metros and ferries; no trains), which
needs no key. The server asks `/stopareacode/<code>,<code>` once every 30 seconds, one request for all stops that
dashboards show, at most ten, with a `User-Agent` that names GDash and its version. OVapi's HTTPS address answers with a
certificate for another domain, so the requests are plain HTTP; they carry no secret and the answer is public timetable
data. The stop search loads `/stopareacode`, all 4600 stop areas, on first use and then once a day, and searches it in
memory; when a refresh fails it keeps using the previous list.

Fuel prices come from the ANWB points-of-interest service (`api.anwb.nl/routing/points-of-interest/v3/all`), which needs
no key. Every 15 minutes the server asks once per distinct place and radius that dashboards show, at most ten, with
`type-filter=FUEL_STATION` and a `bounding-box-filter` around the place, sends a `User-Agent` that names GDash and its
version, reads at most 8 MB, and cuts the circle out of the box itself (haversine distance). A price below 0.10 euro is
ignored, as the service sends 0 for a fuel a station does not sell. When a request fails the last complete answer
stays and the entry is marked `failing`. The data is the ANWB's and can lag a station's own sign.

Changes apply at once: a changed source is fetched again, a removed one
disappears from the displays. `status` has an entry per configured source by
data key (`weather.current`, `airquality.current`, `news.default`,
`calendar.default`, `homeassistant.default`, `departures.default`). The waste, energy price and fuel sources
(`waste.default`, `energyprice.nl`, `fuel.default`), the Homey sources (`homey.default` for values, `homey.users` for
who is home) and custom sources (`custom.source.<id>`) exist only while a dashboard has a widget for them.

Place search sends the typed name to Open-Meteo's geocoding service, which
uses GeoNames data; the coordinates can also be entered by hand.

## Custom APIs

A custom source is any JSON HTTP API set up under Settings. Its secret values never leave the server: they are sealed
with `secret.key` like calendar addresses, never returned, logged or put in an error or the audit log.

**custom source:** `{"id": 1, "name": "Weather in Oss", "url": "https://api.open-meteo.com/v1/forecast?…", "method": "GET", "body": "", "auth": {"kind": "header", "name": "X-API-Key", "secretSet": true}, "headers": [{"name": "X-Tenant", "value": "", "secret": true}], "intervalSeconds": 900, "sample": {…} | null, "values": [named value], "lists": [named list], "unreadable": false, "createdAt": "…", "updatedAt": "…"}`.
A request body has the same fields, with `auth.secret` instead of `auth.secretSet` and `sample` as any JSON value,
and without `values` and `lists`, which change on their own (below) and are kept by a `PUT` of the source:

| Field | Rule |
|---|---|
| `name` | 1 to 60 characters, no leading or trailing spaces |
| `url` | `http` or `https`, at most 2048 characters, without a user name, password or `#` |
| `method` | `GET`, or `POST` with an optional `body` of valid JSON up to 8 KB (sent as `application/json`) |
| `auth.kind` | `none`, `bearer` (`Authorization: Bearer <secret>`), `header` (`<name>: <secret>`), `query` (`?<name>=<secret>`, added when fetching, never stored in `url`; the parameter may not be in `url` already) or `basic` (`name` is the user name) |
| `auth.secret` | up to 2048 characters on one line; a bearer token has no spaces. Empty keeps the stored secret, if the kind is the same |
| `headers` | up to 10 `{name, value, secret}`; not `Host`, `Authorization`, `Content-Type`, `Content-Length` or another header that the request sets itself, no name twice. An empty `value` of a `secret` header keeps the stored one with that name |
| `intervalSeconds` | 30 to 86400 |
| `sample` | an answer to design against, up to 48 KB of JSON, or `null` |

`unreadable` is true when the secrets cannot be decrypted (the database was moved without its key); such a source is
not fetched until its secrets are entered again. At most 20 sources exist.

**Named values.** The values and lists of a source that an admin named under Settings, so the dashboard editor offers
them by name instead of by path. A Custom widget still stores the paths it shows (and a value's unit and decimals), so
renaming or removing a named value never changes a dashboard. **named value:** `{"name": "Outside", "path":
"current.temperature_2m", "unit": "°C", "decimals": 1}`, with `unit` (up to 12 characters) and `decimals` (0 to 6)
optional. **named list:** `{"name": "Releases", "path": "$", "columns": [{"name": "Version", "path": "tag_name"}]}`,
with 1 to 3 columns whose paths start at an item (`$` is the item itself). Names are 1 to 40 characters on one line,
unique per source ignoring case (values and lists together), and a column's name is unique within its list. Paths
follow the syntax below. At most 40 values and 10 lists per source.

| Method and path | Needs | Success | Errors |
|---|---|---|---|
| `GET /api/custom-sources` | admin | `200 {"sources": [custom source]}` | `401` |
| `POST /api/custom-sources` | admin | `201` custom source | `401`, `409` at 20 sources, `422` naming each field, such as `auth.secret` or `headers[1].name` |
| `PUT /api/custom-sources/{id}` | admin | `200` custom source | `401`, `404`, `422` |
| `PUT /api/custom-sources/{id}/values` `{values, lists}` | admin | `200` custom source | `401`, `404`, `422` naming each field, such as `values[1].path` or `lists[0].columns[2].name` |
| `DELETE /api/custom-sources/{id}` | admin | `204`; its widgets say it is not available | `401`, `404` |
| `POST /api/custom-sources/{id}/tests` | admin | `200 {"data", "truncated", "bytes"}`: one fresh answer, cut down for the editor (25 items per list, 200 characters per text, 12 levels, 2000 values) | `401`, `404`, `502 fetch_failed` with what went wrong, such as the status the API answered |
| `POST /api/custom-rule-checks` `{when, data, value?, row?}` | admin | `200 {"result"}`, or `{"error"}` when the condition does not apply to `data` | `401`, `422` naming `when` |

**Fetching.** Only sources that a stored dashboard's Custom widget names are fetched, each on its own interval, through
the same client as feeds and calendars: LAN and localhost addresses work, link-local and cloud metadata addresses are
refused when connecting. A request has 15 seconds; `Accept: application/json` and a `User-Agent` that names GDash are
sent. A redirect is followed only to the same host (at most 3, never from https to http), because a key in a custom
header would otherwise go along. The answer must be a 2xx with at most 1 MB of JSON (`application/json`, `*+json`,
`text/json`, `text/plain` or no type). The last good answer is kept on failure and on disk, like other sources; the
status is under `custom.source.<id>` in `GET /api/settings`. The raw answer stays on the server.

**Data on a screen (`custom.default`).** No source fetches this key: for each screen the server builds, per Custom
widget of that screen's own dashboards, keyed `<dashboardId>/<widgetId>`, only what the widget shows:
`{"kitchen/prices": {"state": "ok", "stale": false, "failing": false, "mode": "list", "title": "Prices", "rows": [{"cells": ["Bread", 2.5], "tone": "warn"}], "tone": "", "message": "Something is dear", "hidden": false}}`.
`state` is `ok`, `loading`, `failing` (no answer yet) or `missing` (the source does not exist or no saved dashboard
uses it yet). Value mode has `value`, `label`, `unit` and `decimals`; key-values mode `pairs: [{label, value}]`; badges mode `badges`, the labels of the badges that are on.
Values are numbers, `true`, `false`, `null` or one line of at most 100 characters; a list or object becomes text. A
list sends at most 15 rows. The screen shows everything as text, never as HTML.

**Paths.** `name` or `.name` (letters, digits and `_`), `["any key"]` or `['any key']` with `\"`, `\'`, `\\` and `\n`
as escapes, `[2]` or `[-1]` (from the end) for an element, `[*]` for every element (the path then gives a list), and
`$` for the whole answer. At most 200 characters and 32 steps. A missing key or element gives nothing (`null` in a
rule). `internal/custom/path.go` and `web/src/admin/jsonPath.ts` implement the same syntax.

**Expressions.** A rule's `when`, at most 500 characters and 20 levels of nesting:

```
or      = and { ("or" | "||") and }
and     = not { ("and" | "&&") not }
not     = ("not" | "!") not | compare
compare = sum [ ("==" | "!=" | "<" | "<=" | ">" | ">=") sum ]
sum     = product { ("+" | "-") product }
product = unary { ("*" | "/") unary }
unary   = "-" unary | primary
primary = number | string | true | false | null | path | name "(" [ or { "," or } ] ")" | "(" or ")"
```

Strings are in double or single quotes. A path starting with `value` is the widget's value (value mode) and one
starting with `row` is the current item (list mode); `$.value` reaches a key called `value` in the answer. `==` and
`!=` compare numbers, texts, booleans and `null` (different kinds are unequal; a list or object equals nothing). The
ordering operators take two numbers or two texts, `+` also joins two texts (up to 4096 bytes), `and`, `or` and `not`
take booleans and stop early. Functions: `len(text | list | object)`, `contains(text, text)` or `contains(list,
value)`, `lower(text)`, `upper(text)`, `round(number)` and `round(number, 0..6)`, `number(number | text)`,
`minutesSince(time)` (ISO 8601 with or without a time and zone, UTC when absent, or a Unix time in seconds or
milliseconds; negative for the future), `isOn(value)` (whether a badge would be on: `true`, a number other than 0, or
the text `on`, `home`, `yes`, `open` or `true` in any case). Anything else, such as comparing `null` with a number, dividing by zero or a
result that is not a finite number, is an evaluation error: the rule does not match. A rule matches only when its
condition gives `true`.

**Rules.** `{"when": "value > 25", "tone": "ok" | "warn" | "bad" | "", "message": "…", "hide": true, "announce": true}`,
up to 6 per widget. A rule that reads `row` is evaluated for every item of the list (it holds when any item matches)
and colours or hides that item; others are evaluated once and colour the value, every row and every pair. The most
serious tone wins; the first matching rule with a message sets `message`; `hide` hides the data, not the title and
message. A rule needs at least a tone, a message or `hide`; `announce` needs a `message`, which is posted as an
announcement to every display, with the widget's title, when the rule starts to hold, at most once every 10 minutes per
rule. The first evaluation after a start, or after the rule is edited, only records its state. Rules are checked after
every data change and every minute; each announcement is in the audit log as `custom_rule_announced`, by the actor
`rule`, with the dashboard and widget and the message.

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
device. `state` is `connecting`, `showing`, `standby` (the TV sleeps), `other_app` (someone else is casting or the TV
shows another input; GDash starts when it ends), `yielded` (someone took the TV from GDash, or used it when GDash
started; GDash waits until the TV has been in standby, or a `cast-takeovers` request), `resting` (outside active hours)
or `unreachable`, with `problem` saying why.

`textScale` is the display's text size in percent: 100 (default), 125, 150 or 200. The snapshot carries it; the
screen draws its stage narrower by that factor and in the screen's own shape (between 4:3 and 21:9), so a small
screen shows the same layout larger and without bars. Widgets then show fewer list items.

**slot:** `{"from": "06:30", "until": "09:00", "days": ["mon", "tue"], "dashboardId": "morning"}`. During the slot the
display shows that dashboard instead of its own; the first slot that contains the current time wins. `days` holds
`mon` to `sun`; empty means every day. A slot past midnight belongs to the day it starts, so Friday 22:00 to 01:00
still applies at 00:30 on Saturday. Times follow the same rules as
active hours, but both are required. Deleting a dashboard removes the slots that use it. Field errors are named
`schedule[i]`, and `schedule` for a slot with an unknown dashboard.

**dashboard:** `{"id": "default", "name": "Default", "columns": 24, "rows": 14, "theme": "minimal-dark", "font": "", "nightTheme": "", "widgets": [widget]}`.
`theme` is one of `minimal-dark` (default), `minimal-light`, `glass`, `warm-light`, `high-contrast`, `night`, `frost`, `walnut`,
`swiss`, `tonal`, `gilded`, `riviera`, `galaxy`, `terminal`, `parchment`, `neon`, `frost-night`, `candy`, `upside-down`, `noir` and
`mars`; `font` is empty for the theme's own typeface or one of `geist`, `inter`, `outfit`, `space-grotesk`, `lora`, `cormorant`, `bodoni`,
`news-cycle`, `jetbrains-mono`, `im-fell` and `courier-prime`. An unknown theme
or font is a `422`. A `PUT` replaces the whole dashboard: leaving `theme` or `font` out sets the default theme or the
theme's own typeface. The grid size and
every widget's `source` are set by the server; values sent by a client are ignored.

`nightTheme` is empty (the default: no switch) or one of the themes. Between sunset and sunrise at the weather location the server
sends screens, and answers the preview, with that theme as the dashboard's `theme`; the `font` stays as chosen, and `nightTheme`
itself is still in the dashboard object. The sun times and the location's UTC offset come from the weather report, so a dashboard
never switches without a weather location, while the first report is still loading, or when the report has no sun times for today.
Screens get a new snapshot every minute, so the switch is on screen within a minute of sunset or sunrise. Near the poles, where
the sun does not rise or set on some days, a day with only a sunset is day until then, a day with only a sunrise is night until
then, and a day with neither follows the weather report's own day or night. An unknown `nightTheme` is a `422` on that field.

**widget:** `{"id": "news", "type": "news", "x": 10, "y": 9, "w": 14, "h": 5, "source": "news.default", "settings": {"maxItems": 3}}`.
A layout is rejected with field errors such as `widgets[2]` when a widget is outside the grid, smaller than its type
allows, overlaps another, lacks a required setting, or has a setting its type does not take.

| Type | Minimum | Data | Settings |
|---|---|---|---|
| `clock` | 4x3 | none | |
| `weather` | 4x4 | `weather.current` | |
| `forecast` | 6x4 | `weather.current` | `days`: 1 to 7, default 5 |
| `sun` | 5x4 | `weather.current` | |
| `airquality` | 5x4 | `airquality.current` | |
| `waste` | 6x4 | `waste.default` | `calendar`: the `id` of one of the calendars in the settings (required) |
| `energyprice` | 6x4 | `energyprice.nl` | |
| `news` | 6x4 | `news.default` | `maxItems`: 1 to 20 |
| `calendar` | 6x4 | `calendar.default` | `maxItems`: 1 to 20 |
| `text` | 4x3 | none | `text`: 1 to 300 characters, line breaks allowed (required) |
| `photos` | 4x3 | `photos.default` | `seconds`: 5 to 600 per photo, default 30 |
| `countdown` | 4x4 | none | `label`: 1 to 60 characters, `date`: `YYYY-MM-DD` (both required) |
| `homeassistant` | 4x3 | `homeassistant.default` | `entities`: 1 to 8 entity IDs such as `sensor.living_room_temperature` (required) |
| `homey` | 4x3 | `homey.default` | `capabilities`: 1 to 8 keys `<device id>/<capability id>` such as `5f2c…/measure_temperature`, from the search (required) |
| `homeypresence` | 4x3 | `homey.presence` | `users`: 1 to 8 `{id, label}`, a Homey user's `id` and a label of 1 to 12 characters on one line, such as initials (required); screens receive this widget without its settings |
| `departures` | 6x4 | `departures.default` | `stop`: a stop area code such as `utrcsw`, from the stop search (required), `stopName`: 1 to 100 characters, shown as the title, `maxItems`: 1 to 20 |
| `fuel` | 6x4 | `fuel.default` | `place`: a place with `name`, `latitude` and `longitude`, default the weather location, `radius`: 1 to 25 km, default 5, `fuel`: `EURO95` (default), `EURO98`, `DIESEL`, `AUTOGAS` or `CNG`, `maxItems`: 1 to 10, default 5 |
| `custom` | 4x3 | `custom.default` | `source`: the `id` of a custom source (required), `mode`: `value`, `list`, `keyValues` or `badges` (required), `title`: up to 60 characters; value mode: `path` (required), `label` (up to 40), `unit` (up to 12), `decimals` 0 to 6; list mode: `path` of the list and `fields`, 1 to 3 paths read from each item (both required); key-values and badges mode: `pairs`, 1 to 8 `{label, path}` (required); a badge is on when its value is `true`, a number other than 0, or the text `on`, `home`, `yes`, `open` or `true` in any case, and a screen only receives the labels of the badges that are on, as `badges`; `rules`: up to 6, see [Custom APIs](#custom-apis) |

A setting's `kind` is `integer`, `text`, `date`, `calendar`, `entities`, `homeyCapabilities`, `homeyUsers`, `stop`,
`place` or `choice`, or for the Custom widget `customSource`, `path`, `paths`, `pairs` and `rules`; `min` and `max` bound an integer, a text's length
in characters, or the number of entities or items. A `choice` is one of the strings in the setting's `options`. A
`place` is exactly `{"name": "Oss", "latitude": 51.765, "longitude": 5.518}`: a name of 1 to 100 characters, a latitude
from -90 to 90 and a longitude from -180 to 180 as numbers, and no other field; it is what `GET /api/places` returns
for a result, without `region`. A Custom widget's settings are checked together, and an error names the setting, such
as `widgets[2]` with `rules[0].when: … at character 7`. Its `source`, like a `calendar`, is not checked against the
sources when a dashboard is saved or imported: a widget whose source is deleted, or that came from another GDash, says
on the screen that its API is not available. A `calendar` setting is a whole number from 1 up, the `id` of a
calendar; the server does not check that the calendar exists when a dashboard is saved, so a widget whose calendar is
deleted later says on the screen that it cannot find it. A `stop` is one code of 1 to 16 letters and digits.
Text settings are shown as plain text on the screen, never as HTML.

**settings:** `{"language": "nl", "location": {"name": "Oss", "latitude": 51.77, "longitude": 5.52} | null, "newsFeeds": [{"id": 1, "name": "NOS", "url": "https://…"}], "calendars": [{"id": 2, "name": "Family", "host": "calendar.google.com", "color": "blue", "unreadable": false}]}`.
A calendar's `color` is one of `blue`, `green`, `orange`, `purple`, `red`, `teal`, `pink` and `yellow`; until one is
chosen it follows the calendar's place in the list. Its events carry the colour to the TV.
Names are 1 to 60 characters; addresses use http or https, and calendars also webcal.
Several news feeds are merged into one list, newest first, each headline with `source` set to its feed's name and
an empty `title`, which the screen fills in in its own language.

**place:** `{"name": "Oss", "region": "North Brabant, Netherlands", "latitude": 51.765, "longitude": 5.518}`

**settings.homeAssistant:** `{"url": "http://192.168.1.10:8123", "unreadable": false}`, or `null` when not connected.

**settings.homey:** `{"url": "http://192.168.1.20", "unreadable": false}`, or `null` when not connected.

**weather data (`weather.current`):** the current values, `days` for the forecast and `sun`, the sunrise and sunset
of yesterday and the coming days as local times, `{"date": "2026-10-09", "sunrise": "2026-10-09T07:50", "sunset": "2026-10-09T18:56"}`.
Either time is empty on a day without that sunrise or sunset. `utcOffsetSeconds` is how far the location's local time is ahead
of UTC, which the server needs to compare its clock with those local times (see `nightTheme`); a report cached by an older
version lacks it until the next fetch.

**air quality data (`airquality.current`):** `{"europeanAqi": 20, "observedAt": "2026-10-09T13:00", "pollen": [{"type": "birch", "grainsPerM3": 12.5}]}`.
The index runs from 0 to 100 and above (good, fair, moderate, poor, very poor, extremely poor at 20, 40, 60, 80, 100).
`pollen` only lists the kinds (alder, birch, grass, mugwort, olive, ragweed) that Open-Meteo has a value for; it is
empty outside Europe and, depending on the model, outside the pollen season.

**waste data (`waste.default`):** the pickups of the calendars that the waste widgets of the display show, by calendar
`id` as text: `{"3": [{"kind": "gft", "title": "GFT", "date": "2026-10-13"}]}`. `kind` is `gft`, `pmd`, `paper`, `rest`,
`glass`, `textile` or `other`, recognised from the event's title (Dutch and English names); `title` is the title as the
calendar has it, which the screen shows for `other`; `date` is the local day the event starts. The server fetches the
next 15 days of each calendar every 30 minutes, at most 60 pickups per calendar, and the screen shows the next pickup of
each kind within 14 days. A calendar that is deleted or cannot be read is left out.

**energy price data (`energyprice.nl`):** `{"days": [{"date": "2026-10-12", "hours": [{"start": "2026-10-12T00:00:00+02:00", "price": 0.22}], "cheapest": {"start": "2026-10-12T12:00:00+02:00", "end": "2026-10-12T15:00:00+02:00", "average": 0.1767}}]}`.
`days` holds today and, from the afternoon on, tomorrow, by Dutch calendar day (`Europe/Amsterdam`); a day has 23, 24 or 25
`hours`. `price` is euro per kWh including VAT as EnergyZero reports it, and can be negative. `cheapest` is the cheapest
stretch of three consecutive hours of that day (the earliest when several cost the same), `end` being when its last hour
ends, or `null` when the day has fewer than three hours. Refreshed hourly.

**entity:** `{"id": "sensor.living_room_temperature", "name": "Living room", "state": "21.4", "unit": "°C", "deviceClass": "temperature"}`.
The `homeassistant.default` data on a screen maps the entity IDs that its own widgets show to the same object
without `id`; an entity Home Assistant does not know is left out.

**capability:** `{"key": "5f2c…/measure_temperature", "zone": "Kitchen", "device": "Thermostat", "title": "Temperature", "capability": "measure_temperature", "value": 21.5, "units": "°C", "decimals": 1}`,
a value of a Homey device. `value` is a number, a boolean, a text or `null` when the device is unavailable; `units`,
`decimals` (0 to 3) and `zone` are left out when Homey has none. The `homey.default` data on a screen maps the keys
that its own widgets show to the same object without `key` and `zone`; a value Homey does not know is left out.

**Homey presence data (`homey.presence`):** maps each Who's home widget on the screen, as `<dashboard id>/<widget id>`,
to the labels of its users who are home, in the widget's order: `{"hall/home": ["AV", "CS"]}`. A user Homey does not
know as present (`false` or `null`) is left out. The server builds it for each screen from `homey.users`, which
never reaches a screen, and sends the widget without its `users` setting, so a screen cannot tell who is away.

**stop:** `{"code": "utrcsw", "name": "Utrecht, CS Jaarbeurszijde"}`, a stop area of OVapi. The name includes the town.
Codes that are not letters and digits only are not listed.

**fuel data (`fuel.default`):** maps the keys of the lists that a screen's own widgets show to the cheapest stations
within the radius, cheapest first, then nearest, at most 10:
`{"51.765,5.518,5,EURO95": [{"name": "Argos, Heesch", "price": 2.369, "distanceKm": 2.9}]}`. A key is the place rounded to
three decimals, the radius and the fuel, joined by commas; a widget without a place has the key `here,<radius>,<fuel>`
and follows the weather location, so no key holds its coordinates. `name` is the station's name with its town unless the
name holds it already, `price` is euro per litre (per kilogram for `CNG`) and `distanceKm` is the straight-line distance
with one decimal. A place without a station that sells the fuel has an empty list. Without a place and without a weather
location there is no `fuel.default` entry at all.

**departures data (`departures.default`):** maps the stop codes that a screen's own widgets show to their next
departures, soonest first and at most 20:
`{"utrcsw": [{"line": "28", "destination": "Science Park P+R", "expected": "2026-10-12T11:07:20+02:00", "delayMinutes": 3, "cancelled": true}]}`.
`expected` is when the vehicle leaves according to the real-time data, as an instant; the times of OVapi are Dutch local
times and GDash reads them as such. `delayMinutes` is how many whole minutes `expected` is later than the timetable and
`cancelled` is true for a cancelled trip; both are left out when zero or false. A stop without departures has an empty
list. Vehicles that have left, and journeys that end at the stop, are not in the list; the screen counts the minutes to
`expected` itself and drops a vehicle when it has gone.

**device:** `{"id": "e542…", "name": "Living room TV", "model": "Chromecast HD", "addr": "192.168.1.50:8009", "hasScreen": true}`

**snapshot event:** `{"type": "snapshot", "version": 3, "serverTime": "…", "language": "nl", "dashboard": {…}, "data": {"weather.current": {"value": {…}, "fetchedAt": "…", "staleAfter": "…", "failing": false}}}`.
Each display gets the dashboard for the current time (its schedule or its own), and `data` only holds the sources its widgets use. `active` is false outside the
display's active hours; `data` is then empty. `announcement` is only present while a message is shown, see
[Announcements](#announcements).
A preview snapshot has the same shape: `active` is true, the household's `language` and the chosen `textScale`
(default 100) are set, there are no `pages`, and `data` holds the sources of that one dashboard. Its `version` is a
hash of the content, so a poller sees whether anything changed. Data a draft needs that no saved dashboard uses yet
(a new Home Assistant entity, Homey value or user, transit stop or custom source) is absent until the dashboard is saved; a Custom widget
then has `state` `missing`. The web interface polls the preview every few
seconds rather than opening a stream: a preview shows one dashboard that may be a draft, which the per-display
event stream cannot do, and polling needs no connection kept open per editor.
The page `GET /preview/{id}` (optionally `?textScale=150`) shows that preview full screen, drawn by the same code
as a display page; it loads without a session, like the web interface, and needs the admin session for its data.
Its content security policy is the display's, except that the web interface may show it in a frame.
See `internal/display/display.go` and `web/src/protocol.ts`.
