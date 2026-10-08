# Testing the Cast receiver on your own TV

This is the v0.1 hardware test from the research (chapter 24). It answers
three questions:

1. Does a dashboard pushed over the Cast message channel render and stay on
   screen?
2. Does Google TV's ambient mode cover it after about 10 minutes?
3. How much memory does the receiver use?

## 1. Register a development receiver

1. Sign up at the [Cast SDK Developer Console](https://cast.google.com/publish)
   (one-time USD 5 fee, paid by you; Daycast cannot do this for you).
2. **Add new application → Custom Receiver.**
   - Name: `Daycast (dev)`
   - Receiver URL: `http://<server-ip>:8420/receiver/`

   An unpublished receiver may use plain HTTP and a LAN address. A published
   one must use HTTPS ([Google docs](https://developers.google.com/cast/docs/registration)).
3. Note the **application ID** (8 characters).
4. **Add new device**: enter the serial number of your Chromecast or Google
   TV (Settings → System → About, or on the box). Wait about 15 minutes, then
   restart the device. Unpublished receivers only run on registered devices.

## 2. Run Daycast on the same network as the TV

```bash
daycast discover
```

This lists Cast devices with their address. Then start the server with the
device and the app ID:

```bash
DAYCAST_LATITUDE=51.65 DAYCAST_LONGITUDE=5.95 \
DAYCAST_CAST_DEVICE=192.168.1.40 DAYCAST_CAST_APP_ID=ABCD1234 \
daycast
```

With Docker, set the same variables in `compose.yaml` and keep
`network_mode: host`.

Daycast only launches the receiver when nothing else is on screen. If the TV
shows YouTube or another cast, it waits.

## 3. What to check and write down

| Check | How | Result |
|---|---|---|
| Dashboard appears | Watch the TV after starting Daycast | |
| Time until it appears | Stopwatch from start to dashboard | |
| Still on screen after 15 min | Leave it | |
| Still on screen after 24 h | Leave it overnight | |
| Ambient mode on Google TV | Same, with "Stay awake" off and on (Developer options) | |
| Recovers after you cast YouTube and stop | Cast a video, stop it, wait up to 30 s | |
| Recovers after a power cut of the TV | Unplug, plug back in | |
| Receiver memory | Open `chrome://inspect` on a PC, add `<device-ip>:9222`, use the Memory tab | |

Remote debugging on port 9222 only works on devices registered in step 1.

Fill in the table and add the device model and firmware version. Those
results decide whether the Android TV app moves forward to v1.0.
