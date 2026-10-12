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
  as a Home Assistant entity or a transit stop nobody shows yet, appears after you save.
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
| Departures | The next buses, trams, metros and ferries at one stop in the Netherlands |
| Fuel prices | The cheapest petrol stations near a place, for one kind of fuel (Netherlands) |
| Custom API | A value, a list or labelled values from any JSON API you add, with rules; see [Custom APIs](#custom-apis) |

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
as the text of a note; it holds no data, no calendar addresses and no custom
API or its key (see [Custom APIs](#custom-apis)).

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

## Departures

The Departures widget shows when the next buses, trams, metros and ferries
leave one stop in the Netherlands, live, like the display at the stop. It
needs no account or key: the times come from [OVapi](https://v0.ovapi.nl),
a free service that collects the real-time data of the Dutch operators.

1. Add a **Departures** widget to a dashboard and select it.
2. Type the stop's name or the town under **Stop**, for example `utrecht cs`
   or `amsterdam bijlmer`, and choose it from the list. A stop in OVapi is a
   stop *area*, the stops of one place that belong together, so both
   directions of a street are in one list.
3. **Items** is how many departures to show at most, 1 to 20. The widget only
   shows the lines that fit, so no line is cut off.

Each line shows the line number, where it goes and the minutes until it
leaves; "now" is the minute it leaves. A vehicle that runs late is shown in
the warning colour with its delay, such as `14 min +3`, and one that is
cancelled is struck through and shows its planned time. A vehicle that ends
its journey at the stop is not a departure and is left out.

The stop's name is stored in the widget when you choose it, so the screen
shows it even before the first times arrive. If OVapi cannot be reached, the
last times stay on screen, and the stop's name turns to the warning colour
once they are more than a minute and a half old, so you can see that they may
be out of date.

Good to know:

- **Trains are not included.** OVapi only has buses, trams, metros and
  ferries; a station's bus stops are there, its platforms are not. Stops without
  any departures at the moment, such as most stations, say so.
- GDash asks OVapi once every 30 seconds, for all stops your dashboards show
  together, and at most ten different stops. The list of stops that the search
  uses is loaded once a day.
- The times are in Dutch time, whatever the time zone of the server or TV.

## Fuel prices

The Fuel prices widget lists the cheapest petrol stations within a few
kilometres of a place, for the fuel you choose. It needs no account or key:
the prices are the public price data of the [ANWB](https://www.anwb.nl),
the same that its route planner and app show.

1. Add a **Fuel prices** widget to a dashboard and select it.
2. **Place**: type a town or postcode and choose it from the list. Leave it
   empty and the widget uses the weather location from Settings, so a
   household with one home needs no setup at all.
3. **Radius (km)** is how far to look, 1 to 25, default 5. The stations are
   those within that circle around the place, not a square.
4. **Fuel** is Euro 95 / E10 (the default), Euro 98, Diesel, LPG or CNG.
5. **Items** is how many stations to show at most, 1 to 10, default 5. The
   widget only shows the rows that fit, so no row is cut off.

Each row shows the price, such as `€2.469`, the station with its town and
how far away it is. The cheapest station comes first, in the accent colour,
with how much it saves per litre against the most expensive station in the
list, such as `save €0.26/l`. Stations at the same price are sorted by
distance. CNG is priced per kilogram.

Good to know:

- **The prices are the ANWB's data and may lag a station's own sign.** Prices
  change during the day and reach the ANWB with a delay, so check the pump
  before you drive across town for a few cents. Stations without a price for
  the chosen fuel, or with a price below 10 cents, are left out.
- GDash asks the ANWB once every 15 minutes, once for each different place
  and radius that your dashboards show, at most ten. When the ANWB cannot be
  reached, the last prices stay on screen and the title turns to the warning
  colour once they are more than a quarter of an hour and a minute old.
- Only the place you choose in a widget, rounded to about 100 m, is sent to
  the ANWB, together with GDash's name and version. A widget without a place
  sends the weather location, as that is rounded already.
- The place search is the one the weather location uses (Open-Meteo, with
  data from GeoNames).

## Custom APIs

The Custom widget shows data from any API that answers with JSON: a public
service, a paid one with an API key, or a device on your own network. You
set it up in the web interface; no code and no restart.

### Add the API

Under **Settings, Data sources, Custom APIs**, choose **Add a custom API**:

- **Name** and **Address**, with `http://` or `https://`. The address is
  shown in the web interface, so put an API key under Authentication, not in
  the address.
- **Method**: GET, or POST with an optional JSON body.
- **Authentication**: none, a bearer token, an API key in a header (such as
  `X-API-Key`), an API key in the address (such as `appid`), or a user name
  and password. **Extra headers** can be marked secret.
- **Refresh**: from every 30 seconds to once a day. Mind the quota of a
  free plan: once a minute is 1440 requests a day.
- **Sample answer** (optional): paste an answer from the API's
  documentation to design a widget before calling the API.

Tokens, keys, passwords and secret header values are stored encrypted and
never shown again: the form says "Stored" and keeps them when you leave the
field empty. **Test** fetches the API once and shows the answer as a tree;
click a value to see its path. GDash only fetches an API while a dashboard
shows it.

### Example: the temperature from Open-Meteo, no key

1. Add a custom API named `Weather in Oss` with the address
   `https://api.open-meteo.com/v1/forecast?latitude=51.77&longitude=5.52&current=temperature_2m,wind_speed_10m`,
   no authentication, refreshed every 15 minutes. **Test** shows
   `{"current": {"time": "2026-10-12T10:00", "temperature_2m": 14.2, "wind_speed_10m": 11.3, …}, …}`.
2. Add a **Custom API** widget to a dashboard and select it. Choose the API
   and **Show: One value**, then **Fetch the API now**.
3. Click the value field, then `temperature_2m` in the tree: the field says
   `current.temperature_2m`. Set the label `Outside`, the unit `°C` and 1
   decimal.
4. Add a rule: when `value < 0`, colour **Bad**, message `Frost: mind the
   roads`. Add another: when `minutesSince(current.time) > 60`, message
   `The weather is more than an hour old`. (Open-Meteo gives times in UTC
   unless you ask for a time zone; a time without a zone counts as UTC.)
5. Save. The TV shows `14.2 °C`, in the warning colour with the message on
   frosty mornings.

### Example: OpenWeatherMap, with an API key

1. Make a free key at [openweathermap.org](https://openweathermap.org/api).
2. Add a custom API with the address
   `https://api.openweathermap.org/data/2.5/weather?q=Oss,NL&units=metric`,
   **Authentication: API key in the address**, parameter name `appid` and
   your key. GDash adds `appid=<key>` to the address when it fetches; the
   key never appears in the address that is stored or shown.
3. In a Custom widget choose **Show: Labelled values** and add lines:
   `Temperature` with `main.temp`, `Feels like` with `main.feels_like`,
   `Humidity` with `main.humidity` and `Sky` with
   `weather[0].description`.

### Badges: who is home

**Show: Badges** shows a round badge with a short label, such as initials,
for every value that is on, and nothing for the others. A value is on when
it is `true`, a number other than 0, or the text `on`, `home`, `yes`, `open`
or `true` (in any case), so it works with Home Assistant's `home` and `on`
states as well as with plain true and false.

For an API that answers
`{"home": {"aanwezigheid_anna": true, "aanwezigheid_bram": false, …}}`, add a
badge per person: label `AN` with `home.aanwezigheid_anna`, label `BR` with
`home.aanwezigheid_bram`, and so on, up to eight. The TV shows `AN` while
Anna is home; when nobody is home it shows a dash. Only the badges that are
on are sent to the screens, so a TV never learns who is away.

### Lists

**Show: A list** shows one row per item of a list in the answer, with up to
three values per row. Click **Use this list** in the tree, or type the list's
path, such as `data.items`. Then click a value inside the list: a field gets
the part below the item, such as `name`. A rule that reads `row`, such as
`row.price > 2` or `row.stock == 0`, is checked for every item and colours or
hides that row. For example, the latest Home Assistant releases from GitHub's
open API: address `https://api.github.com/repos/home-assistant/core/releases?per_page=5`,
list `$` (the answer itself is the list), fields `tag_name` and
`published_at`, and a rule `minutesSince(row.published_at) < 1440` with
colour Good.

### Paths

A path picks a value out of the answer:

| Path | Means |
|---|---|
| `current.temperature_2m` | the key `temperature_2m` inside `current` |
| `items[0].name` | the name of the first item; `items[-1]` is the last |
| `items[*].name` | the names of all items, shown joined with commas |
| `["wind speed"]` | a key with a space, a dot or a dash, in quotes |
| `$` | the whole answer; `$[0]` is the first item of an answer that is a list |

A path that finds nothing shows a dash.

### Rules

A rule is a condition and what to do while it holds: give the value or row
a colour (good, warning or bad, in the theme's colours), show a message in
the widget, hide the data, or **announce** the message on every display. A
widget has up to six rules. The most serious colour of all matching rules
wins; the first matching rule with a message decides the message.

Conditions use:

- numbers (`25`, `0.5`), text in quotes (`"open"`), `true`, `false`, `null`;
- paths into the answer, plus `value` (the value shown, in value mode) and
  `row` (one item, in list mode);
- `==`, `!=`, `<`, `<=`, `>`, `>=`, `and`, `or`, `not`, `+`, `-`, `*`, `/` and
  parentheses;
- `len(x)`, `contains(text or list, x)`, `lower(text)`, `upper(text)`,
  `round(n)` or `round(n, decimals)`, `number(text)` for an API that sends
  numbers as text, and `minutesSince(time)` for an ISO 8601 time or a Unix
  time.

For example `value > 25 and value < 30`, `contains(lower(status), "error")`
or `number(row.delay) >= 5`. A condition that does not apply to the data,
such as `value > 3` while the value is missing, simply does not match.
**Check** in the editor evaluates a condition against the sample or the
fetched answer.

An announcing rule posts its message over every display when it *starts* to
hold, not as long as it holds, and at most once every ten minutes, so a
value that goes back and forth cannot flood the screens. Each announcement
is in the activity log. After GDash starts, a rule that already holds does
not announce until it has stopped holding and starts again.

### Why expressions, not scripts

GDash could have let you write JavaScript. It does not, on purpose. GDash
runs on your home network, often reachable from every device on it; a script
that the server or the TVs run is code from the web interface executed on
your server, and one stolen admin session or one bug would turn into
running anything on that machine. A rule in GDash is parsed into a fixed set
of comparisons and calculations: it cannot loop, store anything, open a
file or a connection, or read anything but the API's answer, and it always
finishes quickly. The TVs only get the finished values, as text.

### Good to know

- An API on your network works, such as `http://192.168.1.20/status`, like
  Home Assistant does. Link-local and cloud metadata addresses
  (`169.254.x.x`) are refused.
- An answer may be at most 1 MB and must be JSON; GDash keeps the last good
  answer when a refresh fails, and the widget's title turns to the warning
  colour while it is out of date.
- A redirect is only followed to the same host, so a key in a header never
  travels to another server.
- An **exported dashboard** keeps the Custom widgets' settings, which name
  their API by its number. The API itself, and its key, are not in the file.
  After importing it into another GDash, add the API there and choose it in
  each Custom widget; until then the widget says its API is not available.
  If that GDash has a different API with the same number, the widget shows
  that one: check the widgets after an import.

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

## Two-step login

With two-step login on, GDash asks for a 6-digit code from an authenticator
app on your phone after your password, so a password that leaks is not
enough to get in. Any app that makes time-based codes works, such as Google
Authenticator, Microsoft Authenticator, Aegis or the passwords app of your
phone.

1. Under **Settings, Account, Two-step login**, enter your password and
   choose **Set up two-step login**.
2. Scan the QR code with the app, or add an account by hand with the key
   shown below it (time-based).
3. Enter the code the app shows and choose **Turn on two-step login**.
4. Save the ten recovery codes: download them as a text file, print them or
   put them in a password manager. They are shown only this once. Each logs
   in once when your phone is not at hand.

Turning it on logs out every other browser. From then on, logging in asks
for the code after the password; **Use a recovery code** takes one of the
saved codes instead. After five wrong codes the login starts again with the
password; more wrong codes in a row make the next one wait, up to an hour,
and after 50 the app's codes are refused until you log in with a recovery
code. That only happens to someone who knows your password: if you did not
type those codes, change it. Every wrong code and every use of a recovery
code is in the activity log. To turn it off, or to move to a new phone, enter your
password and a code under the same heading and choose **Turn off two-step
login**, then set it up again.

Lost the phone and the recovery codes? Someone who can run commands on the
server can run `gdash reset-password <user>`: it sets a new password and turns
two-step login off. See [Troubleshooting](install.md#troubleshooting).

## When GDash is not on the TV

GDash does not take over a TV that someone casts to, such as YouTube from
a phone: it waits, and the dashboard returns within about 30 seconds after
the cast stops. It also leaves a TV in standby alone, because starting a
cast could switch it on; when someone switches the TV on, the dashboard
appears within about 30 seconds. Google TV does not report its own apps, such as Netflix,
to GDash, so there it cannot tell; use active hours to keep the dashboard
away when the TV is mostly used for watching.

More answers are in the troubleshooting section of [install.md](install.md).
