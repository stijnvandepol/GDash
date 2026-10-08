# API

All endpoints are under `/api` and exchange JSON. Routes marked "admin" need
the session cookie (`daycast_session`) that `POST /api/setup` and
`POST /api/sessions` set. Display streams need a paired screen's cookie
(`daycast_display`) or an admin session.
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
| `GET /api/displays` | admin | `200 {"displays": [display + pairedScreens]}` | `401` |
| `POST /api/displays` `{id, name, castDeviceId}` | admin | `201` display | `401`, `409 conflict`, `422` |
| `PATCH /api/displays/{id}` `{name?, castDeviceId?}` | admin | `200` display | `401`, `404 not_found`, `422` |
| `DELETE /api/displays/{id}` | admin | `204` | `401`, `404 not_found` |
| `GET /api/cast-devices` | admin | `200 {"devices": [device]}` | `401`, `500 discovery_failed` |
| `GET /api/displays/{id}/events` | paired screen or admin | `text/event-stream`, `snapshot` events | `401 not_paired`, `404` for an invalid id |
| `POST /api/pairing-requests` `{displayId}` | no | `201 {code, pollToken, expiresAt}` | `404`, `429 rate_limited` |
| `POST /api/display-sessions` `{pollToken}` or `{launchToken}` | no | `201 {displayId}`, sets `daycast_display`; `202` while pending | `400`, `401 invalid_token` |
| `GET /api/display-sessions/current` | paired screen | `200 {displayId}` | `401 not_paired` |
| `POST /api/displays/{id}/pairings` `{code}` | admin | `201 {displayId}` | `401`, `422 invalid_code` |
| `DELETE /api/displays/{id}/pairings` | admin | `200 {removed}` | `401` |

## Pairing

A display page that is not paired asks `POST /api/pairing-requests` for a code
and shows it. An admin enters the code in the web interface, which calls
`POST /api/displays/{id}/pairings`. Meanwhile the page polls
`POST /api/display-sessions {pollToken}` every few seconds; once approved it
gets the `daycast_display` cookie. Codes are valid for 10 minutes.

A page opened by a cast keeper carries a single-use `launch` token in its URL,
valid for 2 minutes, and exchanges it with
`POST /api/display-sessions {launchToken}`. A display keeps one such cast token,
replaced on every launch.

Open streams re-check their access every 20 seconds, so unpairing stops a
screen within that time. Deleting a display unpairs all its screens.

## Objects

**user:** `{"id": 1, "username": "stijn"}`

**display:** `{"id": "kitchen", "name": "Kitchen TV", "castDeviceId": "e542…", "createdAt": "2026-10-08T17:00:00Z"}`.
`id` is 1 to 32 lowercase letters, digits or dashes and is fixed after creation;
`castDeviceId` is empty for browser and kiosk displays.

**device:** `{"id": "e542…", "name": "tv woonkamer", "model": "Chromecast HD", "addr": "192.168.1.160:8009", "hasScreen": true}`

**snapshot event:** `{"type": "snapshot", "version": 3, "serverTime": "…", "dashboard": {…}, "data": {"weather.current": {"value": {…}, "fetchedAt": "…", "staleAfter": "…", "failing": false}}}`.
See `internal/display/display.go` and `web/src/protocol.ts`.
