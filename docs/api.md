# API

All endpoints are under `/api` and exchange JSON. Every route except the
setup, login and display event routes needs a session cookie
(`daycast_session`), which `POST /api/setup` and `POST /api/sessions` set.
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

| Method and path | Session | Success | Errors |
|---|---|---|---|
| `GET /api/setup` | no | `200 {"needed": bool}` | |
| `POST /api/setup` `{setupToken, username, password}` | no | `201` user, sets cookie | `403 invalid_setup_token`, `409 setup_done`, `422`, `429 rate_limited` |
| `POST /api/sessions` `{username, password}` | no | `201` user, sets cookie | `401 invalid_credentials`, `429 rate_limited` |
| `DELETE /api/sessions/current` | yes | `204` | `401` |
| `GET /api/me` | yes | `200` user | `401` |
| `GET /api/displays` | yes | `200 {"displays": [display]}` | `401` |
| `POST /api/displays` `{id, name, castDeviceId}` | yes | `201` display | `401`, `409 conflict`, `422` |
| `PATCH /api/displays/{id}` `{name?, castDeviceId?}` | yes | `200` display | `401`, `404 not_found`, `422` |
| `DELETE /api/displays/{id}` | yes | `204` | `401`, `404 not_found` |
| `GET /api/cast-devices` | yes | `200 {"devices": [device]}` | `401`, `500 discovery_failed` |
| `GET /api/displays/{id}/events` | no | `text/event-stream`, `snapshot` events | `404` for an invalid id |

The display event stream has no session yet: TV pages cannot log in. Display
pairing (planned for v0.5) will protect it.

## Objects

**user:** `{"id": 1, "username": "stijn"}`

**display:** `{"id": "kitchen", "name": "Kitchen TV", "castDeviceId": "e542…", "createdAt": "2026-10-08T17:00:00Z"}`.
`id` is 1 to 32 lowercase letters, digits or dashes and is fixed after creation;
`castDeviceId` is empty for browser and kiosk displays.

**device:** `{"id": "e542…", "name": "tv woonkamer", "model": "Chromecast HD", "addr": "192.168.1.160:8009", "hasScreen": true}`

**snapshot event:** `{"type": "snapshot", "version": 3, "serverTime": "…", "dashboard": {…}, "data": {"weather.current": {"value": {…}, "fetchedAt": "…", "staleAfter": "…", "failing": false}}}`.
See `internal/display/display.go` and `web/src/protocol.ts`.
