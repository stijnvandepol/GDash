# Using an own Cast receiver

By default GDash uses the public DashCast receiver, which opens the display
page on the TV. That needs no account, and it is what was tested at home (see
[progress.md](progress.md)). An own receiver is option A from the research: the
server pushes snapshots over the Cast message channel, so the TV never has to
reach the server over plain HTTP. This guide sets one up for testing.

## 1. Register a development receiver

1. Sign up at the [Cast SDK Developer Console](https://cast.google.com/publish)
   (one-time USD 5 fee, paid by you; GDash cannot do this for you).
2. **Add new application → Custom Receiver.**
   - Name: `GDash (dev)`
   - Receiver URL: `http://<server-ip>:8420/receiver/`

   An unpublished receiver may use plain HTTP and a LAN address. A published
   one must use HTTPS ([Google docs](https://developers.google.com/cast/docs/registration)).
3. Note the **application ID** (8 characters).
4. **Add new device**: enter the serial number of your Chromecast or Google
   TV (Settings → System → About, or on the box). Wait about 15 minutes, then
   restart the device. Unpublished receivers only run on registered devices.

## 2. Start GDash with the app ID

```bash
GDASH_LATITUDE=51.65 GDASH_LONGITUDE=5.95 GDASH_CAST_APP_ID=ABCD1234 gdash
```

With Docker, set `GDASH_CAST_APP_ID` in `compose.yaml` and keep
`network_mode: host`. Then link a display to the TV in the web interface.

GDash only launches the receiver when nothing else is on screen. If the TV
shows YouTube or another cast, it waits.

## 3. What to check and write down

| Check | How | Result |
|---|---|---|
| Dashboard appears | Watch the TV after linking the display | |
| Time until it appears | Stopwatch from linking to dashboard | |
| Still on screen after 30 min | Leave it; the keeper restarts the cast every 15 min | |
| Still on screen after 24 h | Leave it overnight | |
| Recovers after you cast YouTube and stop | Cast a video, stop it, wait up to 30 s | |
| Recovers after a power cut of the TV | Unplug, plug back in | |
| Receiver memory | Open `chrome://inspect` on a PC, add `<device-ip>:9222`, use the Memory tab | |

Remote debugging on port 9222 only works on devices registered in step 1.

Add the device model and firmware version to the results.
