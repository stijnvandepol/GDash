# Using GDash

This guide assumes GDash is installed and you created your account with
the setup token, see [install.md](install.md). The web interface is at
`http://<server>:8420/`.

## First steps

The **Getting started** card at the top lists what is left to do; it
disappears when the basics are done.

1. **Settings, Data sources, Weather location:** type your town or postcode and choose the
   right result, or enter the coordinates. The weather service only receives
   the location rounded to about 1 km.
2. **Settings, Data sources, News and Calendars:** add RSS or Atom feeds, and the secret
   iCal address of each calendar you want on the screen. In Google Calendar
   it is under the calendar's settings, "Secret address in iCal format"; in
   Outlook under "Publish a calendar"; in iCloud under "Public calendar".
   GDash stores the address encrypted and never shows it again. Each
   calendar gets its own colour on the TV, so you can see whose appointment
   it is; choose another one next to the calendar.
3. **Settings, Screens, Language on the TVs:** English, Nederlands, Deutsch or Français.
4. **Displays, Add a display:** pick your TV from the list of Cast devices
   GDash found and give the display a name, such as Kitchen; without one it
   takes the TV's name.

Within a few seconds the TV shows the default dashboard: clock, weather,
calendar and news.

## A screen without Cast

Any browser can show a display: a tablet on the wall, a laptop, or a
Raspberry Pi connected to a monitor.

1. Add a display and choose "No Cast: I open the page myself".
2. Open `http://<server>:8420/d/<display>` on the screen, for example
   `http://192.168.1.10:8420/d/kitchen`. It shows a code.
3. In the web interface, choose **Pair a screen** at that display and enter
   the code. The screen shows the dashboard at once and stays paired.

**Raspberry Pi as a kiosk:** with Raspberry Pi OS (desktop), let Chromium
start full-screen at login. On releases with the labwc desktop, add a line
like this to `~/.config/labwc/autostart` (older releases call the program
`chromium-browser`):

```bash
chromium --kiosk --noerrdialogs --disable-infobars http://192.168.1.10:8420/d/kitchen
```

Pair it once with the code; a reboot keeps it paired.

**Unpair screens** at a display logs out every screen paired with it, for
example a tablet that was lost. They ask for a new code within 20 seconds.

## Dashboards

Under **Dashboards** you edit layouts on the same 16:9 grid as the TV.

**Start from a template** gives you a finished layout in one click instead of
an empty grid: Kitchen (clock, weather, forecast, calendar and news), Hallway
(a big clock, the weather, the next appointments and a note), Bedroom / night
(a calm clock and the weather for today and tomorrow in the Night theme),
Small screen (three large widgets for a Nest Hub or tablet), Family (a large
calendar, photos, a countdown to the next New Year and a note), Living room (in
the Frost style), Study (in Swiss) and Movie night (in Galaxy). Each card shows
the layout; the button adds it as a new dashboard and opens it in the editor,
where you can change anything. The calendar and news widgets fill up once you
add calendars and feeds under Settings, Data sources, and the photos once you add photos.

- **Add** a widget with the buttons above the canvas, drag it to move it and
  drag its corner to resize it. Click a widget to change its settings, such
  as the number of headlines or the text of a note.
- **Style:** the Style button opens a gallery of twenty-one themes in three
  groups, each shown as a small screen. Minimal dark (the default, best for OLED screens),
  Minimal light and Warm light for bright rooms, Glass, High contrast for
  reading from across the room or with poor eyesight, and Night, dim amber
  for a bedroom. Ten more have a character of their own: Frost (frosted
  tiles on a pastel wallpaper, like a phone's widgets), Walnut (dark wood and
  brass), Swiss (square tiles and heavy rules), Tonal (soft tonal colours and
  very round tiles), Gilded (black and gold), Riviera (cream and terracotta
  in an Italian serif), Frost night (Frost in dark mode) and Candy (hot
  pink, the roundest of all). Under Film and series: Galaxy (a field of
  stars and yellow letters), Terminal (green phosphor), Parchment (aged
  paper and ink), Neon (a synthwave night), Upside down (red letters
  glowing on black), Noir (black and white, typed out) and Mars (rust-red
  dust). Each theme has its own typeface; under **Typeface** you can pick
  another of the eleven for this dashboard. All of them ship with
  GDash and work offline. Under **After sunset** the dashboard can switch to
  another theme between sunset and sunrise, for instance Frost to Frost
  night, Minimal light to Minimal dark or Warm light to Night; the typeface
  stays. The switch follows the sun at the weather location from Settings
  and reaches the screen within a minute; without a location the style never
  changes. The default is Keep this style, and the preview shows what a
  screen shows right now.
- **Preview** shows the dashboard the way a TV draws it, with your real
  weather, news and calendar, without a display or a paired screen. It
  includes changes you have not saved yet; nothing is stored and no screen
  changes until you press Save. Choose a text size (100 to 200 %) and a screen
  shape (16:9 TV, 16:10 tablet or a 1024 × 600 Nest Hub) to see how the
  layout holds up on a small screen. Data that only a new widget needs, such
  as a Home Assistant entity nobody shows yet, appears after you save.
- **Save** sends the dashboard to every screen that shows it within seconds.

| Widget | Shows |
|---|---|
| Clock | The time and date |
| Weather | Now, today's range and wind |
| Forecast | Up to seven days, with the chance of rain |
| Sun | Sunrise, sunset, the length of the day and how it compares with yesterday |
| Air quality | The European air quality index in words, and the pollen in the air (Europe only) |
| Waste collection | The next pickup of each kind of household waste in the coming 14 days, from one of your calendars |
| Energy price | The electricity price now, whether it is low, average or high today, today's prices per hour and the cheapest three hours (Netherlands) |
| Calendar | The coming week from your calendars |
| News | Headlines from your feeds |
| Note | A short text for the household |
| Countdown | The days until a date, such as a holiday |
| Photos | A slideshow of your photos |
| Home Assistant | Up to eight sensors or switches from Home Assistant |

**Photos** are added under Settings, Screens, Photos: JPEG, PNG or WebP, up to 8 MB
each and 50 in total. They stay on your server and only go to paired
screens and to you; the rest of the network gets nothing. GDash removes
the location and other details a phone stores in a photo before it keeps
it; the picture itself is not changed.

Every display shows the **Default** dashboard until you choose another one
in the list at the display.

**Export** saves a dashboard as a file, as a backup or to share a layout.
**Import a dashboard** adds such a file as a new dashboard, also in another
GDash. The file holds the layout, the theme (with its night theme and
typeface, if any) and the widget settings, such
as the text of a note; it holds no data and no calendar addresses.

## Waste collection

Many municipalities, and the sites mijnafvalwijzer and afvalkalender, offer
the pickup dates as an **iCal link**. GDash can show the next pickups from it.

1. Find the iCal (or "ICS" or "agenda") link of your address on the site of
   your waste collector. It is an address that ends in `.ics` or is called
   "subscribe to calendar".
2. In GDash, under Settings, Data sources, Calendars, add that link as a
   calendar and give it a name such as "Waste". It is stored encrypted, like
   any calendar.
3. Add a **Waste collection** widget to a dashboard. It asks which calendar to
   use: choose the one you just added. A choice is required; GDash does not
   guess, as your other calendars hold appointments and not pickups.

The widget lists the next pickup of each kind in the coming 14 days, the
soonest first, with the day as Today, Tomorrow or the weekday, in the
language of the screens. GDash recognises the usual names in the titles of
the events, Dutch and English: GFT, groente, green or organic as organic
waste; PMD, plastic or packaging; papier or paper; restafval, rest or
general waste; glas or glass; textiel or textile. Any other title is shown
as it is, with a yellow dot. The colours are green for organic waste, orange
for plastic, blue for paper, grey for general waste, teal for glass and purple
for textile.

A waste calendar can also be shown in the Calendar widget; that is a
different widget and lists every event of the week. If the calendar is
removed later, the widget says that it cannot find it, until you choose
another. The pickups follow the calendar's own refresh, every 30 minutes.

## Energy price

The **Energy price** widget shows the dynamic electricity price in the
Netherlands per hour, in euro per kWh including VAT. It needs no account and
no setting: GDash reads the prices from the free EnergyZero API once an
hour, only while a dashboard has the widget. It shows the price of the hour
that is on now in large figures, whether that is low, average or high
compared with the other hours of the day (the lowest, middle and highest
third of the day's range), a bar for every hour of today with the current
hour marked, and the cheapest stretch of three hours, for example
"Cheapest 13:00 to 16:00". The prices for tomorrow are published around
15:00; once the cheapest hours of today are over, the widget names the
cheapest hours of tomorrow as soon as they are known. Days start and end in
Dutch time. The figures are the prices that EnergyZero publishes, with VAT;
a contract can add a supplier surcharge, so what you pay can differ, but
which hours are cheap does not. The widget needs at least 6 by 4 cells.

## Home Assistant

GDash can show what Home Assistant knows, such as the temperature
inside, whether the front door is open or the power used right now. It
only reads; it never switches anything.

1. In Home Assistant, open your profile, Security, and make a **long-lived
   access token**. A separate Home Assistant user for GDash, without
   admin rights, limits what the token can see.
2. In GDash, under Settings, Data sources, Home Assistant, enter the address you
   open Home Assistant with, such as `http://192.168.1.10:8123`, and the
   token. GDash tries them before it saves them, and stores the token
   encrypted.
3. Add a **Home Assistant** widget to a dashboard and search for the
   entities it should show, up to eight. With one entity the widget shows
   its value large; with more, a list.

Numbers are shown with their unit, doors and windows as open or closed,
motion as detected or clear, people as home or away. The screen updates
once a minute. Home Assistant's own dashboards remain the place to control
things; GDash is the calm screen that shows the day.

## Announcements from Home Assistant or any automation

Home Assistant, a script or any other automation can put a short message on
the TVs, such as "The washing machine is done". It appears in a card over the
dashboard, in the colours of the display's theme, and goes away after the
time you choose. Nothing else in GDash can be changed with it.

1. In GDash, under Settings, Automations, **API tokens**, make a token with the name of
   the automation, for example "Home Assistant". Copy it now: it is shown once,
   and GDash keeps only a fingerprint of it. You can make up to 10 and
   revoke any of them later; the list shows when each was last used.
2. Try it under Settings, Automations, **Send a test announcement**. The message shows on
   every display that is inside its active hours.
3. Send one from a command line, with your own address and token:

   ```sh
   curl -X POST http://192.168.1.10:8420/api/announcements \
     -H "Authorization: Bearer gdk_your_token" \
     -H "Content-Type: application/json" \
     -d '{"title": "Laundry", "message": "The washing machine is done", "seconds": 60}'
   ```

   `message` is required (up to 280 characters of plain text), `title` is
   optional (up to 60), `seconds` is how long it stays (5 to 3600, default 60)
   and `displays` is a list of display IDs such as `["kitchen"]` (default: all
   displays). Anything wrong comes back as a list of fields to correct, see
   [the API](api.md#announcements).

A newer announcement replaces the older one on the displays it is aimed at, so
the screen never shows a backlog. Announcements are not stored: after a restart
of GDash they are gone. A display outside its active hours stays dark.

### Home Assistant example

Keep the token out of your configuration, in `secrets.yaml`. The word
`Bearer` and the space belong to the value:

```yaml
# secrets.yaml
gdash_authorization: "Bearer gdk_your_token"
```

Then add a `rest_command` to `configuration.yaml` (restart Home Assistant, or
reload the REST commands, after changing it):

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

The address is the one you open GDash with, reachable from Home Assistant.
An automation that announces when the washing machine is done, which is when
the power of its smart plug stays below 3 W for two minutes:

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

The trigger fires when the power drops below the limit, so it does not repeat
while the machine stands idle. Raise the limit if your machine idles at a few
watts. Older versions of Home Assistant spell `triggers:` and `trigger:` as
`trigger:` and `platform:`, and `action:` as `service:`.

If nothing shows up, look at the token under Settings, Automations, API tokens:
**Never used** means the request does not reach GDash, so check the address
and port. In Home Assistant, Developer tools, Actions, run the command with a
`message` and read its answer: `401` means the token is wrong or revoked, `422`
names the field to correct, and `429` means more than 30 announcements in a
minute.

## Small screens

A layout is made for a TV across the room. On a Nest Hub, a tablet or a
small monitor, choose a larger **text size** in the list at the display:
125, 150 or 200 %. Everything is drawn larger and lists show fewer items,
so nothing is cut off. The dashboard always fills the screen, also on
16:10 and 4:3 screens.

## Schedules

Choose **Schedule** at a display:

- **Active hours,** such as 06:30 to 23:00. Outside them GDash stops the
  cast and the screen stays dark, so the TV can go to standby.
- **Time slots** show another dashboard during part of the day, such as a
  morning dashboard with the calendar from 06:30 to 09:00 on weekdays, or a
  weekend dashboard on Saturday and Sunday. Pick no days for every day.
  When slots overlap, the first one wins.
- **Pages in turn** show other dashboards one after another with the
  display's own, for example the overview, then the calendar, then the news,
  each for 30 seconds. During a time slot only the slot's dashboard shows.

## Keeping an eye on things

- The display list says per TV whether the dashboard is on screen and since
  when, whether the TV sleeps, another app is on it, or GDash cannot reach
  it.
- **Settings, Data sources** shows per news feed, calendar and the weather when it last
  updated, or that it fails.
- **Settings, Account, Recent activity** lists logins, failed logins, pairings and
  changes of the last 90 days.

## When GDash is not on the TV

GDash does not take over a TV that someone casts to, such as YouTube from
a phone: it waits, and the dashboard returns within about 30 seconds after
the cast stops. It also leaves a TV in standby alone, because starting a
cast could switch it on; when someone switches the TV on, the dashboard
appears within about 30 seconds. Google TV does not report its own apps, such as Netflix,
to GDash, so there it cannot tell; use active hours to keep the dashboard
away when the TV is mostly used for watching.

More answers are in the troubleshooting section of [install.md](install.md).
