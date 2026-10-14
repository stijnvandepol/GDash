# API

All endpoints are under `/api` and exchange JSON. Routes marked "admin" need
the session cookie (`gdash_session`) that `POST /api/setup` and
`POST /api/sessions` set. Display streams need a paired screen's cookie
(`gdash_display`) or an admin session. `POST /api/announcements`,
`PUT /api/custom-sources/{id}/data` and `PUT /api/homey/snapshot` also take an
API token, see [Announcements](#announcements).
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
| `GET /api/system` | admin | `200 {"version", "updateCheckLocked", "update"?: {"version", "url"}}`; `update` is a newer release the daily check found, `url` its release notes; `updateCheckLocked` is true when `GDASH_UPDATE_CHECK=off` turns the check off | `401` |
| `GET /api/audit-events?limit=100` | admin | `200 {"events": [{at, actor, action, detail, ip}]}`, newest first, at most 200 | `400`, `401` |
| `GET /api/api-tokens` | admin | `200 {"tokens": [{id, name, createdAt, lastUsedAt}]}`, oldest first, never the secrets | `401` |
| `POST /api/api-tokens` `{name}` | admin | `201` token with its secret in `token`, shown this once | `401`, `409` at 10 tokens, `422` |
| `DELETE /api/api-tokens/{id}` | admin | `204`; automations that use it get `401` from then on | `401`, `404` |
| `POST /api/announcements` `{message, title?, seconds?, displays?}` | API token or admin | `201 {title?, message, expiresAt, displays}` | `400`, `401 invalid_token` or `not_logged_in`, `422`, `429 rate_limited` |
| `GET /api/displays` | admin | `200 {"displays": [display + pairedScreens + cast? + screenReports]}`; `pairedScreens` counts the screens paired with a code, never the TV a cast keeper opened; `screenReports` is what the display's screens last said about their browsers, newest first, see [Screen reports](#screen-reports) | `401` |
| `POST /api/displays` `{id, name, castDeviceId, dashboardId?, activeFrom?, activeUntil?, schedule?}` | admin | `201` display | `401`, `409 conflict`, `422` (also for an unknown dashboard) |
| `PATCH /api/displays/{id}` `{name?, castDeviceId?, dashboardId?, activeFrom?, activeUntil?, schedule?, rotation?, rotateSeconds?, textScale?}` | admin | `200` display | `401`, `404 not_found`, `422` (also for an unknown dashboard) |
| `DELETE /api/displays/{id}` | admin | `204` | `401`, `404 not_found` |
| `POST /api/displays/{id}/cast-takeovers` | admin | `204`: the display's cast keeper shows the dashboard on the TV within seconds, also over what someone is watching, and stops waiting (`yielded`); recorded in the audit log as `cast_taken_over`. Not while the TV is in standby or outside active hours | `401`, `404 not_found`, `409 not_cast` (no Cast device) or `not_connected` (GDash is still connecting to the TV) |
| `GET /api/widget-types` | admin | `200 {columns, rows, themes, fonts, types: {name: {minW, minH, settings: {key: {kind, min?, max?, required}}}}}` | `401` |
| `GET /api/dashboard-templates` | admin | `200 {"templates": [{id, name, description, theme, widgets}]}`; create a dashboard from one with `POST /api/dashboards` and its `theme` and `widgets` | `401` |
| `GET /api/dashboards` | admin | `200 {"dashboards": [dashboard + createdAt]}` | `401` |
| `POST /api/dashboards` `{id, name, theme?, font?, nightTheme?, widgets}` | admin | `201` dashboard | `401`, `409 conflict`, `422` |
| `GET /api/dashboards/{id}` | admin | `200` dashboard | `401`, `404` |
| `PUT /api/dashboards/{id}` `{name, theme?, font?, nightTheme?, widgets}` | admin | `200` dashboard; screens update at once | `401`, `404`, `422` |
| `DELETE /api/dashboards/{id}` | admin | `204`; its displays fall back to the default | `401`, `404`, `409` for `default` |
| `GET /api/dashboards/{id}/preview?textScale=` | admin | `200` snapshot of the stored dashboard with the data a screen gets now, in its night theme after sunset; no display needed | `401`, `404`, `422` for a `textScale` other than 100, 125, 150 or 200 |
| `POST /api/dashboards/{id}/preview?textScale=` `{name, theme?, font?, nightTheme?, widgets}` | admin | `200` snapshot of that unsaved layout; nothing is stored and no screen changes | `401`, `404` (the dashboard must exist), `422` |
| `GET /api/settings` | admin | `200 {"settings": settings, "status": {key: {updatedAt?, failing}}}` | `401` |
| `PUT /api/settings/location` `{name, latitude, longitude}` | admin | `200` location | `401`, `422` |
| `PUT /api/settings/language` `{language}` | admin | `200`; screens switch at once | `401`, `422` for a language other than `en`, `nl`, `de` or `fr` |
| `PUT /api/settings/update-check` `{enabled}` | admin | `200 {enabled}`; turns the daily check for a new release on or off | `400` for anything but a boolean, `401`, `422` without `enabled` |
| `DELETE /api/settings/location` | admin | `204`; weather widgets show that no location is set | `401` |
| `PUT /api/settings/solar` `{location, arrays, performanceRatio, source}` | admin | `200` the panels, see **settings.solar** | `400`, `401`, `422` naming every invalid field, such as `arrays[0].tilt` |
| `DELETE /api/settings/solar` | admin | `204`; solar forecast widgets say that no panels are set up | `401` |
| `GET /api/places?name=Oss` | admin | `200 {"places": [place]}` | `401`, `422`, `502 upstream_unavailable` |
| `GET /api/news-catalogue` | admin | `200 {"categories": [category], "sources": [catalogue entry]}`, see **news catalogue** | `401` |
| `POST /api/news-feeds` `{name, url}` | admin | `201` news feed | `401`, `409` at 10 feeds, `422`, also for an address that is added already |
| `DELETE /api/news-feeds/{id}` | admin | `204` | `401`, `404` |
| `POST /api/calendars` `{name, url, purpose?}` | admin | `201` calendar, without its address; `purpose` `"waste"` adds a waste collection calendar, which `settings.wasteCalendars` lists, and without it a calendar of appointments | `401`, `409` at 10 calendars or 5 waste calendars, `422` naming `name`, `url` or `purpose` |
| `PATCH /api/calendars/{id}` `{color}` | admin | `204` | `401`, `404`, `422` for another colour |
| `DELETE /api/calendars/{id}` | admin | `204`; for either kind of calendar | `401`, `404` |
| `POST /api/waste-calendar-tests` `{url}` | admin | `200 {"pickups": [{kind, title, date}]}`: the pickups of the next 56 days, read now and not stored, as in the **waste data** | `401`, `422` naming `url`, `502 fetch_failed` with what went wrong, such as the status the calendar server answered, never the address |
| `PUT /api/homeassistant` `{url, token}` | admin | `200 {"url": …}` after Home Assistant accepted the token | `401`, `422` naming `url` or `token` |
| `DELETE /api/homeassistant` | admin | `204`; Home Assistant widgets show that it is not connected | `401` |
| `GET /api/homeassistant/entities` | admin | `200 {"entities": [entity]}`, sorted by ID | `401`, `409 not_connected`, `502 upstream_unavailable` |
| `PUT /api/homey` `{url, key}` or `{"mode": "push"}` | admin | `200 {"mode": "local", "url": …}` after Homey accepted the API key, or `200 {"mode": "push", "url": ""}` | `401`, `422` naming `mode`, `url` or `key` |
| `PUT /api/homey/snapshot` with a snapshot | admin or API token | `204` | see Settings below |
| `DELETE /api/homey` | admin | `204`; Homey widgets show that it is not connected | `401` |
| `GET /api/homey/capabilities` | admin | `200 {"capabilities": [capability]}`, every readable value of every device, sorted by zone, device and title | `401`, `409 not_connected` or `nothing_received`, `502 upstream_unavailable` |
| `GET /api/homey/users` | admin | `200 {"users": [{id, name}]}`, sorted by name, without whether they are home | `401`, `409 not_connected` or `nothing_received`, `502 upstream_unavailable` (also when the key lacks View users) |
| `GET /api/traffic` | admin | `200 {enabled, acknowledgedAt?, anwb: {state, httpStatus?, nextAttemptAt?}, data?, cameras?}`; `state` is `idle`, `ok`, `failing` or `stopped`, `data` and `cameras` are `{updatedAt?, failing}` while a Traffic widget needs them | `401` |
| `PUT /api/traffic` `{enabled, acknowledged}` | admin | `200 {enabled, acknowledgedAt?}`; turning it on needs `acknowledged: true` and starts asking the ANWB afresh, also after a stop | `401`, `422` naming `acknowledged` |
| `GET /api/knmi` | admin | `200 {connected, unreadable, data?}`, never the key | `401` |
| `PUT /api/knmi` `{key}` | admin | `200 {"connected": true}` after the KNMI accepted the key | `401`, `422` naming `key` (refused, or the KNMI cannot be reached) |
| `DELETE /api/knmi` | admin | `204`; Weather widgets show the model's values again | `401` |
| `GET /api/departures/stops?q=utrecht+cs` | admin | `200 {"stops": [stop]}`, at most 20, best matches first | `401`, `422` naming `q` (2 to 100 characters), `502 upstream_unavailable` |
| `GET /api/custom-sources` and the other custom API routes | admin | see [Custom APIs](#custom-apis) | |
| `GET /api/web-pages` and the other web page routes | admin | see [Web pages](#web-pages) | |
| `GET /api/photos` | admin | `200 {"photos": [{name, size, addedAt}]}`, oldest first | `401` |
| `POST /api/photos` multipart, field `photo` | admin | `201` photo | `401`, `409` at 50 photos, `422` for another kind of file or one over 8 MB |
| `DELETE /api/photos/{name}` | admin | `204` | `401`, `404` |
| `GET /api/photos/{name}` | admin or any paired screen | `200` the image | `404`, also for anyone else |
| `PUT /api/immich` `{url, key}` | admin | `200` [settings.immich](#objects) after Immich listed its albums with the key; the album stays chosen when the address stays the same | `401`, `422` naming `url` or `key` |
| `DELETE /api/immich` | admin | `204`; Photos widgets set to Immich show that it is not set up | `401` |
| `GET /api/immich/albums` | admin | `200 {"albums": [{id, name, assetCount}]}`, owned and shared, at most 500 | `401`, `409 not_connected`, `502 upstream_unavailable` |
| `PUT /api/immich/album` `{albumId}` | admin | `200` settings.immich | `401`, `409 not_connected`, `422` naming `albumId` for an album Immich does not list, `502 upstream_unavailable` |
| `GET /api/immich/assets/{id}` | admin, or a screen paired with a display that shows the Immich album now | `200` the photo, Immich's `preview` size with its location and camera details removed, `Cache-Control: private, max-age=86400` | `404` for anyone else and for any ID that is not one of the album's current photos (`photos.immich`), `502 upstream_unavailable` |
| `PUT /api/synology` `{link, password, acceptCertificate}` | admin | `200` settings.synologyPhotos with `photos`, how many photos GDash can show, after the NAS opened the album; an empty `password` keeps the stored one for the same link; with `acceptCertificate` GDash reads the certificate the NAS presents and from then on accepts only that one | `401`, `422` naming `link`, `password` (the NAS refused the link or password) or `acceptCertificate` (an untrusted certificate, or an http link) |
| `DELETE /api/synology` | admin | `204`; Photos widgets set to Synology Photos show that it is not set up | `401` |
| `GET /api/synology/photos/{id}` | admin, or a screen paired with a display that shows the Synology Photos album now | `200` the photo, as for Immich | `404` for anyone else and for any ID that is not one of the album's current photos (`photos.synology`), `502 upstream_unavailable` |
| `GET /api/cameras` | admin | `200 {"cameras": [camera], "unreadable": false}`; `unreadable` is true when the cameras cannot be decrypted, and adding one then starts a new list | `401` |
| `POST /api/cameras` `{name, url, auth}` | admin | `201` camera | `401`, `409` at 10 cameras, `422` naming each field |
| `PUT /api/cameras/{id}` `{name, url, auth}` | admin | `200` camera; an empty `auth.secret` keeps the stored one for the same kind | `401`, `404`, `422` |
| `DELETE /api/cameras/{id}` | admin | `204`; Camera widgets that show it say it is not available | `401`, `404` |
| `POST /api/cameras/{id}/tests` | admin | `200 {contentType, bytes}` after one image, past what is kept | `401`, `404`, `502 fetch_failed` with what went wrong, naming the host but never the address's path, query or a secret |
| `GET /api/cameras/{id}/image` | admin, or a screen paired with a display that shows the camera now | `200` the latest image, `Cache-Control: no-store`, `Last-Modified` when GDash received it, `X-Image-Stale: true` when the camera failed since | `404` for anyone else and for an unknown camera, `502 camera_unavailable` when the camera has not sent an image yet |
| `GET /api/now-playing/artwork/{id}` | admin, or a paired screen whose display was sent this cover in its `nowplaying.default` data | `200` the cover (JPEG, PNG, WebP or GIF, at most 2 MB), `Cache-Control: private, max-age=600` | `404`, also for anyone else, a cover of something that no longer plays and one that could not be fetched |
| `GET /api/cast-devices` | admin | `200 {"devices": [device]}` | `401`, `500 discovery_failed` |
| `GET /api/displays/{id}/events` | paired screen or admin | `text/event-stream`, `snapshot` events | `401 not_paired`, `404` for an invalid id |
| `POST /api/displays/{id}/screen-reports` with a [screen report](#screen-reports) | a screen paired with that display; not an admin | `204`; kept in memory as the screen's last report | `400`, `401 not_paired`, `403 other_display`, `404` for an invalid id, `413 too_large` above 8 KB, `422` naming every invalid field, `429 rate_limited` above 10 reports per screen in 10 minutes (with `Retry-After`) |
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

## Screen reports

A TV's built-in browser is hard to inspect, so every display page tells the
server about its own browser: once it has drawn the dashboard, and again
after it reconnects, at most once a minute. The server keeps the last report
of each screen in memory, never in the database, for at most 5 screens per
display; a restart, unpairing the screens or deleting the display forgets
them. `GET /api/displays` lists them per display as `screenReports`, and the
Displays page shows them under **What the screen reports**.

```json
{"userAgent": "Mozilla/5.0 (Linux; Android 12.0; …) Chrome/87.0.4280.141 Safari/537.36 CrKey/1.56.500000 DeviceType/AndroidTV",
 "viewport": {"width": 1280, "height": 720}, "devicePixelRatio": 1.5, "stageScale": 0.6667,
 "features": {"containerUnits": false, "containerQueries": false, "where": true, "has": false, "colorMix": false,
              "aspectRatio": true, "flexGap": true, "inset": true, "lineClamp": true, "textWrap": false,
              "nesting": false, "cascadeLayers": false},
 "compat": true, "fontsLoaded": true, "firstRenderMs": 2400,
 "errors": [{"message": "TypeError: x is not a function", "source": "/assets/App-abc.js:1:200"}]}
```

`userAgent` is 1 to 512 characters; `viewport` is the page's size in CSS
pixels (1 to 16384); `devicePixelRatio` is above 0 and at most 16;
`stageScale` (above 0, at most 32) is how far the 1920-pixel stage is scaled
to fit the screen. `features` says which CSS features the browser has, all
twelve as booleans; `compat` is true when the page uses its fallback for
browsers without container queries. `fontsLoaded` is whether the typefaces
had loaded, `firstRenderMs` the milliseconds from loading the page until the
dashboard was first on screen (0 to 3600000, or `null` when nothing was
drawn, for example outside active hours). `errors` holds at most 5 uncaught
script errors: a `message` of 1 to 300 characters and a `source` of up to 200,
the script's path with line and column, never the page's address. Texts are
one line without control characters. In the list each report also has
`screen` (`cast` for the TV a cast keeper opened, `screen` for one paired
with a code) and `receivedAt`.

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

**Authentication.** Make an API token under Integrations, API tokens (or with
`POST /api/api-tokens`) and send it as `Authorization: Bearer gdk_…`. Only the
token's SHA-256 hash is stored, so the token is shown once. There are at most 10
tokens, and `lastUsedAt` shows when each was last used. A token is valid for this
route and for sending the data of a pushed custom source
(`PUT /api/custom-sources/{id}/data`, see [Custom APIs](#custom-apis)) or a
Homey snapshot (`PUT /api/homey/snapshot`, see [Settings](#settings)) only:
every other route answers `401` to it. Every token can do all three; there are
no narrower ones. Revoking a token takes effect at once. A request with an `Authorization` header is judged on that header alone,
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
and `GDASH_CALENDAR_URLS` into the settings; later starts ignore them. A feed from the news catalogue gets the
catalogue's name, such as NOS Nieuws; another is named after its host.

With `GDASH_TEST_UPSTREAM` (the end-to-end tests only, a loopback address) the feeds of the news catalogue are read
from `<GDASH_TEST_UPSTREAM>/news/<catalogue id>` instead of their publishers; an address of one's own is fetched as
it is.

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

A Homey without API keys, such as a Homey Pro (Early 2019), sends its data instead: `PUT /api/homey` with
`{"mode": "push"}` (no `url` or `key`, else `422`) drops any connection and makes GDash wait for snapshots, which
the HomeyScript shown under Integrations, Homey sends every minute from a Flow:

```sh
curl -X PUT http://gdash.local:8420/api/homey/snapshot \
  -H "Authorization: Bearer $GDASH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"at": "2026-10-13T08:00:00Z",
       "devices": [{"id": "5f2c…", "name": "Thermostat", "zone": "Kitchen", "available": true,
                    "caps": {"measure_temperature": {"title": "Temperature", "value": 21.5, "units": "°C"}}}],
       "users": [{"id": "9a1b…", "name": "Anna", "present": true, "asleep": false}]}'
```

It takes an API token or an admin session, like announcements (with the same cross-site protection), and answers
`204`; `409 not_pushed` when Homey is not set up to send, `413` over 1 MB, `415` without
`Content-Type: application/json`, `422 invalid_json` or `422 not_a_snapshot` without a list of `devices`, `429`
within 5 seconds of the last accepted snapshot (a refused one does not count). The shape is read leniently: a
device or user with an ID that is not letters, digits and dashes (at most 64), or repeated, is left out, as is a
capability whose ID is not one Homey uses; texts are cut to 200 characters, a value that is not a number, boolean
or text becomes `null`, a device that is not `available` shows no values, and only the first 500 devices, 64
capabilities per device (by ID) and 50 users are kept; `at` is ignored. The last snapshot is kept whole, also on
disk, so a restart shows it until the next one; `GET /api/homey/capabilities` and `/users` answer from it
(`409 nothing_received` before the first), and screens get `homey.default` and `homey.presence` from it in the
same form and narrowed the same way as with a connection. Its entries count as current for three minutes.
Snapshots are not audited.

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

Rain comes from Buienradar's `gpsgadget.buienradar.nl/data/raintext?lat=<lat>&lon=<lon>` with two decimals, which needs
no key and answers one `VVV|HH:MM` line per five minutes for two hours, in Dutch time; the intensity is
`10^((VVV-109)/32)` mm/h, and 0 is dry. Outside the Netherlands and Belgium (a box from 49.4 to 53.8 north and 2.4 to 7.3
east), or when Buienradar fails or sends fewer than six readable lines, the server asks Open-Meteo for
`minutely_15=precipitation&forecast_minutely_15=10&timezone=auto` and turns each quarter's total into mm/h for the
quarter it covers. Every 5 minutes it asks once per distinct place rounded to two decimals, at most ten, with a
`User-Agent` that names GDash, and reads at most 64 KB. Buienradar's data is free for non-commercial use with a credit to
buienradar.nl, which the widget shows.

Weather warnings come from MeteoAlarm's feed of the Netherlands (`feeds.meteoalarm.org/api/v1/warnings/feeds-netherlands`),
which needs no key and holds the KNMI's CAP alerts. Every 15 minutes the server reads it once (at most 16 MB) and keeps,
per land region (the twelve provinces and the Wadden Islands, matched by MeteoAlarm's `EMMA_ID` geocode or the area's
name), the alerts with `status` `Actual` and `msgType` `Alert` or `Update` at awareness level 2 (yellow), 3 (orange) or 4
(red) that have not expired. An update counts under the identifier of the first alert in its `references`, and the
newest version wins. A widget without a region takes the region of the weather location by the nearest of a list of
towns; outside the Netherlands it has no region and there is no entry for it. For widgets with `announce`, the server
checks after every data change and every minute which code orange or red warnings came into effect, and posts each one
once per identifier and level to every display for 300 seconds, with the code and region as the title and the KNMI's
headline as the message (Dutch on a Dutch screen, else English). Warnings already in effect when the server starts, or
when a region is first announced, are not posted. Each one is in the audit log as `weather_warning_announced`, by the
actor `weather warning`.

`GDASH_TEST_UPSTREAM`, for the end-to-end tests only, points Buienradar, Open-Meteo's rain and solar forecasts and
MeteoAlarm at `<address>/buienradar/data/raintext`, `<address>/open-meteo/v1/forecast`,
`<address>/open-meteo-solar/v1/forecast` and `<address>/meteoalarm/api/v1/warnings/feeds-netherlands`, and the ANWB,
Overpass and the KNMI at `<address>/anwb/traffic`, `<address>/overpass/api/interpreter` and `<address>/knmi/edr`; the
server refuses an address that is not on the loopback address.

Traffic info comes from the ANWB's undocumented traffic list
(`api.anwb.nl/traffic/traffic-info/v1/incidents/incidents-desktop`), unofficially: the ANWB forbids reusing its
content without permission, so it is off until an admin turns it on with `PUT /api/traffic` and
`acknowledged: true`, which is stored with its time and audited (`traffic_info_enabled`). While it is on and a
Traffic widget is on a dashboard, the server asks once for the whole country, at most every 3 minutes for all
widgets together, with a `User-Agent` that names GDash and its version, and reads at most 8 MB. The answer is read
leniently: unknown fields are ignored and an odd item is left out. A jam's `distance` is in metres and `delay` in
seconds, the national totals in kilometres and minutes (checked against a live answer). After a failure it waits 6
minutes, doubling up to 30; after three refusals in a row (a 4xx other than 408 and 429) it stops until the switch is
turned off and on, and `GET /api/traffic` reports `stopped`. The entry `traffic.default` holds, per widget query
(`roads:A2,A50` or `near:25`), at most 10 jams, roadworks (on now or starting within 24 hours), speed checks and fixed
cameras, the camera count and the national totals; a screen gets only its own queries. Fixed cameras come from the
Overpass API (`overpass-api.de`, OpenStreetMap, ODbL): one query for the Netherlands at most once a week, kept in
`speedcameras.json` in the data directory, retried at most every 6 hours, with a 2-minute timeout; its status is the
entry `traffic.cameras`, which exists while a widget shows cameras.

Measured weather comes from the KNMI EDR API (collection `10-minute-in-situ-meteorological-observations` on
`api.dataplatform.knmi.nl/edr/v1`) with the household's own key, sent bare in the `Authorization`
header as the KNMI documents. `PUT /api/knmi` tries the key on the station list first. While a Weather widget has
`"current": "knmi"`, a key and a weather location are set, the server reads the station list once a day and every
10 minutes asks `locations/<station>` for `ta,rh,ff` over the last hour, for the nearest station within 50 km that
reported a temperature, trying up to three. The entry `knmi.observation` is `{station, distanceKm, observedAt,
temperatureC, humidity, windKmh}` (null for what the station did not report; the wind converted from m/s); only
screens with such a widget get it. The key is stored encrypted with `secret.key`, never returned, logged or put in
an error, and is part of the source's keyed fingerprint only.

Changes apply at once: a changed source is fetched again, a removed one
disappears from the displays. `status` has an entry per configured source by
data key (`weather.current`, `airquality.current`, `news.default`,
`calendar.default`, `homeassistant.default`, `departures.default`). These sources exist only while a dashboard has a
widget for them:

- waste `waste.default`, energy price `energyprice.nl`, solar `solar.default`, fuel `fuel.default`
- holidays `holidays.nl`, rain `rain.default`, weather warnings `warnings.default`
- Homey `homey.default` (values) and `homey.users` (who is home)
- the albums `photos.immich` and `photos.synology`
- Now playing `nowplaying.default`, which GDash pushes itself from the Cast devices
- custom sources `custom.source.<id>`; a pushed custom source is there from the start, as it is never fetched

Place search sends the typed name to Open-Meteo's geocoding service, which
uses GeoNames data; the coordinates can also be entered by hand.

## Custom APIs

A custom source is any JSON HTTP API set up under Integrations. Its secret values never leave the server: they are sealed
with `secret.key` like calendar addresses, never returned, logged or put in an error or the audit log.

**custom source:** `{"id": 1, "name": "Weather in Oss", "mode": "fetch", "url": "https://api.open-meteo.com/v1/forecast?…", "method": "GET", "body": "", "auth": {"kind": "header", "name": "X-API-Key", "secretSet": true}, "headers": [{"name": "X-Tenant", "value": "", "secret": true}], "intervalSeconds": 900, "sample": {…} | null, "values": [named value], "lists": [named list], "preset": "", "unreadable": false, "createdAt": "…", "updatedAt": "…"}`.
A request body has the same fields, with `auth.secret` instead of `auth.secretSet` and `sample` as any JSON value,
and without `values` and `lists`, which change on their own (below) and are kept by a `PUT` of the source:

| Field | Rule |
|---|---|
| `name` | 1 to 60 characters, no leading or trailing spaces |
| `mode` | `fetch` (GDash fetches `url`; also when absent) or `push` (the household's system sends the answer, see Pushing below). A pushed source has no request: `url`, `body` and `auth.name` empty, `method` empty or `GET`, `auth.kind` empty or `none`, no `headers`, else a `422` on that field. Switching to `push` drops a fetched source's secrets |
| `url` | `http` or `https`, at most 2048 characters, without a user name, password or `#` |
| `method` | `GET`, or `POST` with an optional `body` of valid JSON up to 8 KB (sent as `application/json`) |
| `auth.kind` | `none`, `bearer` (`Authorization: Bearer <secret>`), `header` (`<name>: <secret>`), `query` (`?<name>=<secret>`, added when fetching, never stored in `url`; the parameter may not be in `url` already) `basic` (`name` is the user name) or `login` (log in first, below) |
| `auth.secret` | up to 2048 characters on one line; a bearer token has no spaces. Empty keeps the stored secret, if the kind is the same. For `login` it is the password |
| `auth.login` | for `login` only: `{url, body, tokenPath, place, validityPath}`. `url` is the login address, `http` or `https` on the same scheme, host and port as `url`; `body` is JSON of at most 8 KB with `{secret}` inside a string where the password goes, such as `{"password": "{secret}"}`; `tokenPath` is the path of the token in the login's answer, such as `session.sid`; `place` is `header` (`<name>: <token>`), `query` (`?<name>=<token>`) or `bearer` (`Authorization: Bearer <token>`, `name` empty); `validityPath` is the path of the seconds the token lasts, such as `session.validity`, or empty. Returned as it was saved: it holds no secret |
| `headers` | up to 10 `{name, value, secret}`; not `Host`, `Authorization`, `Content-Type`, `Content-Length` or another header that the request sets itself, no name twice. An empty `value` of a `secret` header keeps the stored one with that name |
| `intervalSeconds` | 30 to 86400: how often it is fetched, or for a pushed source how often an answer is expected |
| `sample` | an answer to design against, up to 48 KB of JSON, or `null` |
| `preset` | the id of the preset the web interface made the source from, such as `homewizard-p1` (the name of its file in `web/src/admin/presets`): up to 40 lowercase letters, digits and `-`, starting with a letter or digit. Empty or absent for the household's own API. The server checks the form only; the web interface shows a source with a preset under that preset's own card and offers it as a widget of its own. A `PUT` replaces it like the other fields, so a `PUT` without it makes the source the household's own |

The list adds `lastPushAt` to a pushed source once it sent an answer: when the last one arrived.

`unreadable` is true when the secrets cannot be decrypted (the database was moved without its key); such a source is
not fetched until its secrets are entered again. At most 20 sources exist.

**Named values.** The values and lists of a source that an admin named under Integrations, so the dashboard editor offers
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
| `POST /api/custom-sources/{id}/tests` | admin | `200 {"data", "truncated", "bytes"}`: one fresh answer, cut down for the editor (25 items per list, 200 characters per text, 12 levels, 2000 values); for a pushed source the last answer it sent, with `receivedAt` | `401`, `404`, `502 fetch_failed` with what went wrong, such as the status the API answered; `409 nothing_received` for a pushed source that sent nothing yet |
| `POST /api/custom-source-tests` with a source as for `POST /api/custom-sources`, and `id` of a stored source or none | admin | `200` as a test: one fresh answer of the source as given, which is not stored, so a form can try it before it saves. With `id` it is a change of that source: a secret left empty is its stored one | `401`, `404` for an unknown `id`, `422` naming each field, also `mode` for a pushed source, `502 fetch_failed` |
| `PUT /api/custom-sources/{id}/data` with the answer as the body | admin or API token | `204` | `401`, `403` cross-site, `404`, `409 not_pushed` for a source that GDash fetches, `413 too_large` over 1 MB, `415` without `Content-Type: application/json`, `422 invalid_json`, `429 rate_limited` within 5 seconds of its last accepted answer, with `Retry-After: 5` |
| `POST /api/custom-rule-checks` `{when, data, value?, row?}` | admin | `200 {"result"}`, or `{"error"}` when the condition does not apply to `data` | `401`, `422` naming `when` |

**Fetching.** Only sources that a stored dashboard's Custom widget names are fetched, each on its own interval, through
the same client as feeds and calendars: LAN and localhost addresses work, link-local and cloud metadata addresses are
refused when connecting. A request has 15 seconds; `Accept: application/json` and a `User-Agent` that names GDash are
sent. A redirect is followed only to the same host (at most 3, never from https to http), because a key in a custom
header would otherwise go along. The answer must be a 2xx with at most 1 MB of JSON (`application/json`, `*+json`,
`text/json`, `text/plain` or no type). The last good answer is kept on failure and on disk, like other sources; the
status is under `custom.source.<id>` in `GET /api/settings`. The raw answer stays on the server.

**Logging in first.** A `login` source posts `auth.login.body`, with the password escaped as JSON text in place of
`{secret}`, to `auth.login.url` (`Content-Type: application/json`, the same client, redirect and size rules) and reads
the token at `tokenPath`: text of at most 8 KB on one line. It then sends the token with every request, in the place
`place` names. The token is kept in memory only, sealed with `secret.key`, never stored, logged, returned or put in an
error, and one login is made at a time per source. GDash logs in again when the seconds at `validityPath` run out
(30 seconds early, at most 10%; a validity is taken as 10 seconds to 24 hours), when `auth.login`, `auth.name` or the
password changes, after a restart, and once when the API answers `401` or `403` to a token it accepted before. A
login answered with `401` or `403` gives `502 fetch_failed` "the login was refused …; check the password", without
retrying. A Test of the source shares the session with the screens.

**Pushing.** A system that cannot answer requests, such as a Homey Pro (Early 2019) running HomeyScript, Node-RED or a
Home Assistant `rest_command`, sends the answer of a `push` source itself:

```sh
curl -X PUT http://gdash.local:8420/api/custom-sources/3/data \
  -H "Authorization: Bearer $GDASH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"outside": 12.5, "home": {"Anna": true}}'
```

The body is held to the same rules as a fetched answer (at most 1 MB of JSON) and is then handled exactly like one: it
is the source's latest answer, kept on disk as the last known good one, its named values, rules and announcing rules
apply, and screens still get only what their widgets show. It takes an API token or an admin session, like
announcements, with the same cross-site protection. One answer per source per 5 seconds is accepted; a refused one
(wrong type, too large, not JSON) does not count. An answer counts as current for two intervals and 30 seconds, so one
late push does not show; after that the widget's title turns to the warning colour, as for a failing fetch, and keeps
the last values. Pushes are not in the audit log, as a script sends one every minute.

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

## Web pages

A web page is a page that a Web page widget shows in a frame on the TV, set up under Integrations. The editor picks one by
its `id`; a dashboard never holds an address.

**web page:** `{"id": 1, "name": "Energy in Grafana", "url": "http://192.168.1.10:3000/d/energy?orgId=1&kiosk", "refreshMinutes": 0, "zoom": 80, "allowScripts": true, "createdAt": "…", "updatedAt": "…"}`.
A request body has the fields `name`, `url`, `refreshMinutes`, `zoom` and `allowScripts`:

| Field | Rule |
|---|---|
| `name` | 1 to 60 characters on one line; leading and trailing spaces are removed |
| `url` | `http` or `https`, at most 2048 characters, without spaces, a user name or a password, on a host name or IPv4 address (a Content-Security-Policy cannot name an IPv6 address), and not on the origin the request reached GDash on |
| `refreshMinutes` | 0 (never, also when absent) or 1 to 1440: the TV reloads the page that often |
| `zoom` | 50 to 200 percent; 0 or absent is 100 |
| `allowScripts` | whether the page may run scripts; absent is `true` |

| Method and path | Needs | Success | Errors |
|---|---|---|---|
| `GET /api/web-pages` | admin | `200 {"pages": [web page]}`, oldest first | `401` |
| `POST /api/web-pages` | admin | `201` web page; audited as `web_page_added` with its name and host | `401`, `409` at 20 pages, `422` naming each field |
| `PUT /api/web-pages/{id}` | admin | `200` web page; audited as `web_page_changed` | `401`, `404`, `422` |
| `DELETE /api/web-pages/{id}` | admin | `204`; its widgets show that the page is not available; audited as `web_page_removed` | `401`, `404` |
| `POST /api/web-pages/{id}/checks` | admin | `200 {"status": 200, "blocked"?: "x-frame-options" \| "frame-ancestors", "header"?: "DENY"}`: GDash fetched the page once, through the same client as feeds (LAN allowed, link-local and metadata addresses refused, 10 seconds, redirects followed), and read the headers of the answer. `blocked` names the header that forbids a frame on GDash's origin as this request reached it: `X-Frame-Options` `DENY` or `SAMEORIGIN` (ignored when a `frame-ancestors` directive is present, as browsers do), or a `frame-ancestors` that does not list it; `header` is that header, up to 300 characters. The body is never read | `401`, `404`, `502 fetch_failed` naming the host, never the path or query |

**On the screens.** The `webpage` widget's data key `webpage.default` holds the pages by `id` as text, `{"1": {"name",
"url", "refreshMinutes", "zoom", "allowScripts"}}`, and a screen only receives the pages its own widgets show. The
display page `/d/{id}` answers with `frame-src` listing exactly the origins of the pages on every dashboard that
display shows (its own, its rotation and its schedule), and only to a screen paired with it or an admin; otherwise,
and for a display without web pages, `frame-src 'none'`. The preview page `/preview/{id}` lists the origins of every
page under Integrations for an admin, because an unsaved layout may use any of them, and `'none'` for anyone else. A page
on the origin of the request is never listed. The web interface keeps its own policy.

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

**cast:** `{"state": "showing", "since": "2026-10-09T08:31:00Z", "problem"?: "…", "reason"?: "not_found"}`, only for
a display on a Cast device. `state` is `connecting`, `showing`, `standby` (the receiver runs but the TV sleeps),
`other_app` (someone else is casting or the TV shows another input; GDash starts when it ends), `yielded` (someone took
the TV from GDash, or used it when GDash started; GDash waits until the TV has been in standby, its idle screen has
shown for 10 minutes, or a `cast-takeovers` request), `resting` (outside active hours) or `unreachable`, with `problem`
saying why in the words of the error and `reason` sorting it: `not_found` (discovery did not find the device) or
`no_answer` (anything else, such as a device that refused or dropped the connection).

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
| `moon` | 4x4 | none | |
| `airquality` | 5x4 | `airquality.current` | |
| `waste` | 6x4 | `waste.default` | `calendar`: the `id` of one of the waste calendars in the settings; without it the widget shows the only waste calendar, and asks for a choice when there are several |
| `energyprice` | 6x4 | `energyprice.nl` | |
| `solar` | 6x4 | `solar.default` | |
| `news` | 6x4 | `news.default` | `maxItems`: 1 to 20, `feeds`: up to 10 `id`s of news feeds in the settings, each once; absent or empty shows every feed, as dashboards from before the setting do |
| `calendar` | 6x4 | `calendar.default` | `maxItems`: 1 to 20 |
| `text` | 4x3 | none | `text`: 1 to 300 characters, line breaks allowed (required) |
| `photos` | 4x3 | `photos.default`, or `photos.immich` or `photos.synology` with `source` `immich` or `synology` | `seconds`: 5 to 600 per photo, default 30, `source`: `uploaded` (default), `immich` or `synology` |
| `countdown` | 4x4 | none | `label`: 1 to 60 characters, `date`: `YYYY-MM-DD` (both required) |
| `holidays` | 6x4 | `holidays.nl` | `region`: `north`, `middle` (default) or `south`, `show`: `both` (default), `school` or `public`, `maxItems`: 1 to 20, default 5 |
| `homeassistant` | 4x3 | `homeassistant.default` | `entities`: 1 to 8 entity IDs such as `sensor.living_room_temperature` (required) |
| `homey` | 4x3 | `homey.default` | `capabilities`: 1 to 8 keys `<device id>/<capability id>` such as `5f2c…/measure_temperature`, from the search (required) |
| `homeypresence` | 4x3 | `homey.presence` | `users`: 1 to 8 `{id, label}`, a Homey user's `id` and a label of 1 to 12 characters on one line, such as initials (required); screens receive this widget without its settings |
| `departures` | 6x4 | `departures.default` | `stop`: a stop area code such as `utrcsw`, from the stop search (required), `stopName`: 1 to 100 characters, shown as the title, `maxItems`: 1 to 20 |
| `rain` | 5x3 | `rain.default` | `place`: a place with `name`, `latitude` and `longitude`, default the weather location |
| `warnings` | 6x3 | `warnings.default` | `region`: `Groningen`, `Friesland`, `Drenthe`, `Overijssel`, `Flevoland`, `Gelderland`, `Utrecht`, `Noord-Holland`, `Zuid-Holland`, `Zeeland`, `Noord-Brabant`, `Limburg` or `Waddeneilanden`, default the province of the weather location, `announce`: `true` to put a code orange or red warning over every display when it starts, default `false` |
| `fuel` | 6x4 | `fuel.default` | `place`: a place with `name`, `latitude` and `longitude`, default the weather location, `radius`: 1 to 25 km, default 5, `fuel`: `EURO95` (default), `EURO98`, `DIESEL`, `AUTOGAS` or `CNG`, `maxItems`: 1 to 10, default 5 |
| `camera` | 4x3 | none: the screen loads `GET /api/cameras/{id}/image` | `camera`: the `id` of a camera (required), `seconds`: 5 to 300 between images, default 10, `fit`: `contain` (default, the whole image) or `cover` (fills the tile) |
| `nowplaying` | 6x4 | `nowplaying.default` | `devices`: up to 8 Cast device IDs (the 32 hex characters of `GET /api/cast-devices`); absent or empty follows any device. A TV that a display is linked to is never followed. Screens receive only the devices their widgets follow |
| `webpage` | 4x3 | `webpage.default` | `page`: the `id` of a web page under Integrations (required), `zoom`: 50 to 200 percent, default the page's own |
| `custom` | 4x3 | `custom.default` | `source`: the `id` of a custom source (required), `mode`: `value`, `list`, `keyValues` or `badges` (required), `title`: up to 60 characters; value mode: `path` (required), `label` (up to 40), `unit` (up to 12), `decimals` 0 to 6; list mode: `path` of the list and `fields`, 1 to 3 paths read from each item (both required); key-values and badges mode: `pairs`, 1 to 8 `{label, path}` (required); a badge is on when its value is `true`, a number other than 0, or the text `on`, `home`, `yes`, `open` or `true` in any case, and a screen only receives the labels of the badges that are on, as `badges`; `rules`: up to 6, see [Custom APIs](#custom-apis) |

A setting's `kind` is `integer`, `text`, `date`, `calendar`, `camera`, `entities`, `homeyCapabilities`, `homeyUsers`,
`castDevices`, `newsFeeds`, `stop`, `place`, `webPage`, `choice` or `boolean`, or for the Custom widget `customSource`, `path`, `paths`, `pairs` and `rules`;
`min` and `max` bound an integer, a text's length in characters, or the number of entities or items. A `choice` is one
of the strings in the setting's `options`; a `boolean` is `true` or `false`. A
`place` is exactly `{"name": "Oss", "latitude": 51.765, "longitude": 5.518}`: a name of 1 to 100 characters, a latitude
from -90 to 90 and a longitude from -180 to 180 as numbers, and no other field; it is what `GET /api/places` returns
for a result, without `region`. A Custom widget's settings are checked together, and an error names the setting, such
as `widgets[2]` with `rules[0].when: … at character 7`. Its `source`, like a `calendar`, is not checked against the
sources when a dashboard is saved or imported: a widget whose source is deleted, or that came from another GDash, says
on the screen that its API is not available. A `calendar` setting is a whole number from 1 up, the `id` of a
waste calendar; the server does not check that the calendar exists when a dashboard is saved, so a widget whose
calendar is deleted later, or that names a calendar of appointments, says on the screen that it cannot find it. A `newsFeeds` setting is a list of such `id`s of news
feeds, likewise not checked: a widget shows those of its feeds that still exist, and says on the screen when none
does. A `camera` setting is the same for a camera; IDs of removed
cameras are never given to a new one. A `stop` is one code of 1 to 16 letters and digits.
Text settings are shown as plain text on the screen, never as HTML.

**settings:** `{"language": "nl", "location": {"name": "Oss", "latitude": 51.77, "longitude": 5.52} | null, "newsFeeds": [{"id": 1, "name": "NOS", "url": "https://…"}], "calendars": [{"id": 2, "name": "Family", "host": "calendar.google.com", "color": "blue", "unreadable": false}], "wasteCalendars": [{"id": 3, "name": "Home", "host": "www.mijnafvalwijzer.nl", "color": "green", "unreadable": false}], "updateCheck": true}`.
`calendars` are the calendars of appointments, which the Calendar widget shows; `wasteCalendars` are the waste
collection calendars, which only Waste collection widgets show. Both are calendars under `/api/calendars`.
`updateCheck` is whether GDash asks `api.github.com` once a day for the latest release; it is on until turned off. See
[SECURITY.md](../SECURITY.md).
A calendar's `color` is one of `blue`, `green`, `orange`, `purple`, `red`, `teal`, `pink` and `yellow`; until one is
chosen it follows the calendar's place in the list. Its events carry the colour to the TV.
Names are 1 to 60 characters; addresses use http or https, and calendars also webcal.
The entry `news.default` holds the newest 10 headlines of every news feed by the feed's `id`:
`{"3": {"name": "NOS Nieuws", "items": [{"title": "…", "link": "https://…", "published": "2026-10-14T09:00:00Z"}]}}`.
A screen receives only the feeds its News widgets show (all of them when a widget has no `feeds`); the widget merges
its feeds newest first, names each headline's feed when it shows more than one, and takes the feed's name as its
title when it shows one. When one feed fails, the whole entry keeps its last value and is marked `failing`.

**news catalogue:** the news feeds GDash offers to choose from, in `internal/news/catalogue.json`, Dutch ones first:
`{"id": "nos-algemeen", "name": "NOS Nieuws", "language": "nl", "country": "NL", "category": "general",
"url": "https://feeds.nos.nl/nosnieuwsalgemeen", "note": "Listed by the NOS at nos.nl/feeds."}`. `language` is an
ISO 639-1 code, `country` an ISO 3166-1 alpha-2 code, `category` one of `general`, `international`, `regional`,
`sport`, `economy`, `tech`, `science`, `culture`, `offbeat` and `kids`, and `note`, when present, what is known of
the publisher's terms. Choosing one is `POST /api/news-feeds` with its `name` and `url`; the web interface knows a
chosen catalogue source by its address.

**place:** `{"name": "Oss", "region": "North Brabant, Netherlands", "latitude": 51.765, "longitude": 5.518}`

**settings.homeAssistant:** `{"url": "http://192.168.1.10:8123", "unreadable": false}`, or `null` when not connected.

**settings.solar:** `{"location": null, "arrays": [{"tilt": 35, "azimuth": -10, "kwp": 4.2}], "performanceRatio": 0.85, "source": "open-meteo"}`,
or `null` until the panels are set up. `location` is `null` for the weather location, or a place like **settings.location**.
`arrays` holds 1 or 2 groups of panels: `tilt` a whole number of degrees from 0 (flat) to 90, `azimuth` a whole number from
-180 to 180 (0 south, -90 east, 90 west, ±180 north, as Open-Meteo and Forecast.Solar count) and `kwp` the peak power from
0.1 to 100. `performanceRatio` (0.5 to 1.0) scales the Open-Meteo estimate. `source` is `open-meteo` or `forecast.solar`.

**settings.homey:** `{"mode": "local", "url": "http://192.168.1.20", "unreadable": false}`, `{"mode": "push", "url": "", "unreadable": false}` for a Homey that sends its data, or `null` when not connected.

**settings.immich:** `{"url": "http://192.168.1.10:2283", "albumId": "4b1c…", "albumName": "Holidays", "unreadable": false}`,
or `null` when not connected; `albumId` and `albumName` are empty until an album is chosen. The address is stored
without a trailing `/api`. GDash calls Immich with the key in the `x-api-key` header on `GET /api/albums`,
`POST /api/search/random` `{"albumIds": [id], "type": "IMAGE", "size": 100}` and
`GET /api/assets/{id}/thumbnail?size=preview`, paths that Immich v2 and v3 serve; Immich v3.2 marks `albumIds` and
`type` of the search as deprecated in favour of a `filter` object that v2 does not know. Redirects to another host
are refused, as the key would follow them.

**photos data (`photos.immich`):** the IDs of up to 100 images of the chosen album, picked at random by Immich and
shuffled, refreshed every 30 minutes, only while a Photos widget shows the album; videos are left out. A screen
loads each from `GET /api/immich/assets/{id}`.

**settings.synologyPhotos:** `{"host": "192.168.1.2:5001", "passwordSet": true, "certificate": "AB:01:…", "unreadable": false}`,
or `null` when no album is connected; `certificate` is the SHA-256 fingerprint of the accepted certificate, or empty
when the NAS's certificate is checked as browsers do. The link (which opens the album), its password and the
fingerprint are sealed together. GDash calls the NAS at `{link base}/{xx}/sharing/webapi/entry.cgi`, as the
Synology Photos page does for a share link: `SYNO.Core.Sharing.Login` (v1, `sharing_id` and the password
JSON-quoted) for a session cookie, `SYNO.Foto.Browse.Item` `list` (v1, `additional=["thumbnail"]`, `passphrase`, header
`X-SYNO-SHARING`) and `SYNO.Foto.Thumbnail` `get` (v2, `id`, `cache_key`, `type="unit"`, `size="xl"` or `"m"`,
`passphrase`, `_sharing_id`). Synology does not document these for share links; they follow the open-source clients
syno-photo-frame, MMM-SynPhotoSlideshow and py-synologydsm-api. Redirects to another host are refused.

**photos data (`photos.synology`):** up to 100 photos of the shared album, at random and shuffled, refreshed every 30
minutes while a Photos widget shows it, as `"<unit id>-<cache key>-<size>"`; videos and photos without a thumbnail
are left out. A screen loads each from `GET /api/synology/photos/{id}`.

**camera:** `{"id": 1, "name": "Front door", "url": "http://192.168.1.20:5000/api/front_door/latest.jpg", "auth": {"kind": "basic", "name": "admin", "secretSet": true}}`.
The input has `auth: {kind, name, secret}` with the kinds of a custom source except `login` (`none`, `bearer`, `header`, `query`,
`basic`) and the same rules. The address is http or https without a user name, password or `#` part. A camera's
image is a JPEG, PNG or WebP of at most 5 MB, judged by its bytes; the request times out after 10 seconds and
follows redirects only to the same host. GDash asks the camera only when a screen or admin asks for its image and the
kept one is at least the shortest `seconds` of the Camera widgets that the asking screen's display shows (5 seconds
for an admin); it keeps only the latest image of each camera, in memory, and drops it after 10 minutes without a
request. The cameras are stored as one sealed value in the settings.

**weather data (`weather.current`):** the current values, `days` for the forecast and `sun`, the sunrise and sunset
of yesterday and the coming days as local times, `{"date": "2026-10-09", "sunrise": "2026-10-09T07:50", "sunset": "2026-10-09T18:56"}`.
Either time is empty on a day without that sunrise or sunset. `utcOffsetSeconds` is how far the location's local time is ahead
of UTC, which the server needs to compare its clock with those local times (see `nightTheme`); a report cached by an older
version lacks it until the next fetch.

**air quality data (`airquality.current`):** `{"europeanAqi": 20, "observedAt": "2026-10-09T13:00", "pollen": [{"type": "birch", "grainsPerM3": 12.5}]}`.
The index runs from 0 to 100 and above (good, fair, moderate, poor, very poor, extremely poor at 20, 40, 60, 80, 100).
`pollen` only lists the kinds (alder, birch, grass, mugwort, olive, ragweed) that Open-Meteo has a value for; it is
empty outside Europe and, depending on the model, outside the pollen season.

**waste data (`waste.default`):** the pickups of the waste calendars that the waste widgets of the display show, by
calendar `id` as text: `{"3": [{"kind": "gft", "title": "GFT", "date": "2026-10-13"}]}`, and under `"default"` those of
the only waste calendar when a widget names none. `kind` is `gft`, `pmd`, `paper`, `rest`, `glass`, `textile`, `tree`
(Christmas trees) or `other`, recognised from the event's title (Dutch and English names); `title` is the title as the
calendar has it, which the screen shows for `other`; `date` is the local day the event starts. The server fetches the
next 15 days of each calendar every 30 minutes, at most 60 pickups per calendar, and the screen shows the next pickup of
each kind within 14 days. A calendar that is deleted, cannot be read or is not a waste calendar is left out, and so is
`"default"` with no waste calendar or several.

**holiday data (`holidays.nl`):** `{"public": [{"kind": "christmas", "date": "2026-12-25"}], "middle": [{"kind": "autumn", "name": "Herfstvakantie", "start": "2026-10-17", "end": "2026-10-25"}]}`.
`public` holds the Dutch public holidays from today to the end of next year, computed by the server: `kind` is
`newYear`, `goodFriday`, `easter`, `easterMonday`, `kingsDay` (27 April, or 26 April when the 27th is a Sunday),
`liberationDay`, `ascension`, `whitSunday`, `whitMonday`, `christmas` or `boxingDay`. `north`, `middle` and `south` hold
the school holidays of each region that have not ended, `start` and `end` being the first and last day off; holidays
of the whole country are in every region. `kind` is `autumn`, `christmas`, `spring`, `may`, `summer`, or `other` for a
holiday the feed names otherwise, which the screen shows by `name`, the feed's Dutch name. A region is only there while
a widget shows school holidays, and a screen only receives `public` and the regions its own widgets show; an empty list
means the feed has no holiday left for it. The school holidays are read once a day from the government's open data
(`opendata.rijksoverheid.nl/v1/infotypes/schoolholidays`, CC0, no key) with a `User-Agent` that names GDash, at most
1 MB; the first and last days are read as Dutch days. When the read fails the last complete answer stays and the entry
is marked `failing`.

**energy price data (`energyprice.nl`):** `{"days": [{"date": "2026-10-12", "hours": [{"start": "2026-10-12T00:00:00+02:00", "price": 0.22}], "cheapest": {"start": "2026-10-12T12:00:00+02:00", "end": "2026-10-12T15:00:00+02:00", "average": 0.1767}}]}`.
`days` holds today and, from the afternoon on, tomorrow, by Dutch calendar day (`Europe/Amsterdam`); a day has 23, 24 or 25
`hours`. `price` is euro per kWh including VAT as EnergyZero reports it, and can be negative. `cheapest` is the cheapest
stretch of three consecutive hours of that day (the earliest when several cost the same), `end` being when its last hour
ends, or `null` when the day has fewer than three hours. Refreshed hourly.

**solar data (`solar.default`):** `{"source": "open-meteo", "utcOffsetSeconds": 7200, "days": [{"date": "2026-10-13", "totalKWh": 14.2, "hours": [{"start": "2026-10-13T08:00:00+02:00", "kwh": 0.31}]}]}`.
`days` holds today and tomorrow at the panels' place, in its UTC offset; `hours` run from the first to the last hour
with production, `start` being when the hour begins. With `open-meteo` the server asks Open-Meteo for
`hourly=global_tilted_irradiance` with each group's `tilt` and `azimuth` (one request per group, `forecast_days=2`,
`timezone=auto`); a value is the mean over the hour before its time, and the hour's energy is value ÷ 1000 × `kwp` ×
`performanceRatio`, summed over the groups. With `forecast.solar` it asks `api.forecast.solar/estimate/{lat}/{lon}/{tilt}/{azimuth}/{kwp}`
per group and adds up `watt_hours_period`. Coordinates are rounded to two decimals. Refreshed every 30 minutes, only
while a dashboard has the widget and the panels have a place; without one the server sends no entry.

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

**rain data (`rain.default`):** maps the keys of the places that a screen's own widgets show to their forecast:
`{"51.77,5.52": {"source": "buienradar", "stepMinutes": 5, "points": [{"time": "2026-10-13T14:05:00+02:00", "mmPerHour": 0.52}]}}`.
A key is the place rounded to two decimals; a widget without a place has the key `here` and follows the weather location.
`source` is `buienradar` or `open-meteo`, `stepMinutes` how long each point lasts (5 or 15), and each point's `time` is
when its step starts, written in the place's time zone, with the intensity in mm/h rounded to two decimals. Without a
place and without a weather location there is no `rain.default` entry.

**warnings data (`warnings.default`):** maps the keys of the regions that a screen's own widgets show to their
warnings, soonest first: `{"Noord-Brabant": {"region": "Noord-Brabant", "warnings": [{"id": "2.49.0.0.528.0.NL.…_1",
"level": "orange", "kind": "wind", "event": {"nl": "Zware windstoten", "en": "Severe Wind"}, "headline": {"nl": "…",
"en": "…"}, "onset": "2026-10-13T12:00:00Z", "expires": "2026-10-13T22:00:00Z"}]}}`. A widget without a region has the
key `here`, under which `region` names the province of the weather location. `level` is `yellow`, `orange` or `red`;
`kind` is MeteoAlarm's awareness type (`wind`, `snow-ice`, `thunderstorm`, `fog`, `high-temperature`,
`low-temperature`, `coastal-event`, `forest-fire`, `avalanches`, `rain`, `flooding`, `rain-flood`) or empty; `event` and
`headline` are the KNMI's texts by language. A region without warnings has an empty list.

**departures data (`departures.default`):** maps the stop codes that a screen's own widgets show to their next
departures, soonest first and at most 20:
`{"utrcsw": [{"line": "28", "destination": "Science Park P+R", "expected": "2026-10-12T11:07:20+02:00", "delayMinutes": 3, "cancelled": true}]}`.
`expected` is when the vehicle leaves according to the real-time data, as an instant; the times of OVapi are Dutch local
times and GDash reads them as such. `delayMinutes` is how many whole minutes `expected` is later than the timetable and
`cancelled` is true for a cancelled trip; both are left out when zero or false. A stop without departures has an empty
list. Vehicles that have left, and journeys that end at the stop, are not in the list; the screen counts the minutes to
`expected` itself and drops a vehicle when it has gone.

**now playing data (`nowplaying.default`):** maps the Cast device IDs that a screen's own widgets follow (all of them
when one follows any device) to what plays there, for the devices that play, pause or buffer something:
`{"e542…": {"name": "Kitchen speaker", "group": true, "app": "Spotify", "title": "One More Time", "artist": "Daft Punk", "album": "Discovery", "image": "/api/now-playing/artwork/0123…", "state": "playing", "position": 63.5, "duration": 320.4, "at": "2026-10-13T08:00:00Z"}}`.
`state` is `playing`, `paused` or `buffering`; `position` and `duration` are seconds, and `position` was read at `at`,
so a playing track has moved on by the time since then. `duration` is 0 for a live stream. `artist` is the artist of a
track, the series of an episode or the subtitle (such as a YouTube channel); `group` is only present for a speaker
group, and `album` and `image` only when the app gives them. `image` is a GDash address, never the app's own. The
entry is pushed by GDash itself, which follows the devices over the Cast protocol without controlling them; an empty
object means that nothing plays.

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
