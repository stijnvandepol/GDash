# Using GDash

This guide assumes GDash is installed and you created your account with
the setup token, see [install.md](install.md). The web interface is at
`http://<server>:8420/`.

## First steps

The **Getting started** card at the top lists what is left to do; it
disappears when the basics are done.

1. **Integrations, Weather:** type your town or postcode and choose the
   right result, or enter the coordinates. The weather service only receives
   the location rounded to about 1 km.
2. **Integrations, News feeds and Calendars:** add RSS or Atom feeds, and the secret
   iCal address of each calendar you want on the screen. In Google Calendar
   it is under the calendar's settings, "Secret address in iCal format"; in
   Outlook under "Publish a calendar"; in iCloud under "Public calendar".
   GDash stores the address encrypted and never shows it again. Each
   calendar gets its own colour on the TV, so you can see whose appointment
   it is; choose another one next to the calendar.
3. **Settings, Screens, Language on the TVs:** English, Nederlands, Deutsch or Français.
4. **Displays, Add a display:** pick your TV from the list of Cast devices
   GDash found and give the display a name, such as Kitchen; without one it
   takes the TV's name. **Shows** is the dashboard it starts with: the
   newest one you made, or the default dashboard (clock, weather, calendar
   and news) when you have not made one yet.

Within a few seconds the TV shows the dashboard, and the display's row says
what the TV is doing, such as "On Tv Keuken since 13:40" or "Cannot find Tv
Keuken on the network". Change what it shows at any time under **Shows**.

## Integrations

Everything GDash shows data from is under **Integrations**, apart from the
other settings: the weather location, news feeds, calendars, Home
Assistant, Homey, photos, custom APIs and the API tokens that automations
use. Settings keeps the language on the TVs, the test announcement and your
account.

- **Search** finds a service by its name, what it shows, its category or a
  label, such as "solar", "train" or "no key". The search stays in the
  address, so a link or a reload keeps it. The filters show only what is
  **Connected**, useful in the **Netherlands**, runs on your own network
  (**Local**) or needs **No key**.
- Each card says whether it is **Connected** and when it last updated, **Not
  set up**, or why it fails. **Set up** or **Manage** opens its setup right
  under the card; **Close** folds it away again.
- **Built in · no setup** cards, such as fuel prices, departures and the
  electricity price, only need a widget: **Add to a dashboard** goes to the
  dashboards.
- **Preset · Custom API** cards make a custom API for a known device or
  service, such as a HomeWizard P1 meter or Tibber, see
  [Start from a preset](#start-from-a-preset).

Nothing matches? If the service has an API that answers with JSON, **Use a
Custom API**.

## A screen without Cast

Any browser can show a display: a tablet on the wall, a laptop, or a
Raspberry Pi connected to a monitor.

1. Add a display and choose **None** as the Cast TV.
2. The display's row says "No screen yet" and shows its address, for example
   `http://192.168.1.10:8420/d/kitchen`. Open that address on the screen. It
   shows a code.
3. Enter the code in the row and choose **Connect**. The screen shows the
   dashboard at once and stays connected; the row then says "Shown on 1
   screen". **Add another screen…** connects one more.

**Raspberry Pi as a kiosk:** with Raspberry Pi OS (desktop), let Chromium
start full-screen at login. On releases with the labwc desktop, add a line
like this to `~/.config/labwc/autostart` (older releases call the program
`chromium-browser`):

```bash
chromium --kiosk --noerrdialogs --disable-infobars http://192.168.1.10:8420/d/kitchen
```

Connect it once with the code; a reboot keeps it connected.

A display on a Cast TV can be open on other screens too: **Open on another
screen…** at the display works the same way, and the row then says "Also open
on 1 other screen". The TV itself needs no code; GDash connects it.

**Disconnect screens** at a display logs out every screen connected with a
code, for example a tablet that was lost. They show a new code within 20
seconds. A Cast TV stays connected.

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
add calendars and feeds under Integrations, and the photos once you add photos.

- **Add** a widget from the panel beside the canvas (below it on a phone):
  the widgets are grouped by what they are about, such as Time & date or
  Travel, and the search field on top finds one by its name or what it
  shows (Escape clears it, Enter adds the first match). Drag a widget to
  move it and drag its corner to resize it. Click a widget to change its
  settings, such as the number of headlines or the text of a note; the
  panel then shows them, and **← Add widgets** goes back. The editor is
  about how a dashboard looks: addresses, keys and the values an API gives
  are set up under Integrations.
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
  stays. The switch follows the sun at the weather location from Integrations
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
| Moon | The phase of the moon, drawn, how much of it is lit, and the dates of the next full and new moon |
| Air quality | The European air quality index in words, and the pollen in the air (Europe only) |
| Rain | The rain in the next two hours as bars, and one sentence such as "Rain from 14:20 (light)"; see [Rain](#rain) |
| Weather warnings | The KNMI's code yellow, orange and red warnings for a province (Netherlands); see [Weather warnings](#weather-warnings) |
| Waste collection | The next pickup of each kind of household waste in the coming 14 days, from one of your calendars |
| Energy price | The electricity price now, whether it is low, average or high today, today's prices per hour and the cheapest three hours (Netherlands) |
| Calendar | The coming week from your calendars |
| News | Headlines from your feeds |
| Note | A short text for the household |
| Countdown | The days until a date, such as a holiday |
| Holidays | The coming school holidays of your region and the public holidays in the Netherlands; see [Holidays](#holidays) |
| Photos | A slideshow of your photos |
| Home Assistant | Up to eight sensors or switches from Home Assistant |
| Homey | Up to eight sensors or switches from a Homey Pro; see [Homey](#homey) |
| Who's home (Homey) | A round badge with initials for each chosen person who is home |
| Departures | The next buses, trams, metros and ferries at one stop in the Netherlands |
| Fuel prices | The cheapest petrol stations near a place, for one kind of fuel (Netherlands) |
| Custom API | A value, a list or labelled values from any JSON API you add, with rules; see [Custom APIs](#custom-apis) |

**Photos** are added under Integrations, Photos: JPEG, PNG or WebP, up to 8 MB
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
2. In GDash, under Integrations, Calendars, add that link as a
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

## Holidays

The **Holidays** widget lists the coming school holidays and public holidays
in the Netherlands, the soonest first, in the language of the screens:
"Autumn holiday · Starts in 4 days (17–25 Oct)", "Christmas holiday · 19
Dec – 3 Jan", "King's Day · Tue 27 Apr". A holiday that is on now comes
first, in the accent colour, with when it ends: "Now, until Sunday".

- **School holiday region** is North, Middle or South, as the government
  divides the country; Middle when you do not choose. Holidays the whole
  country shares, such as Christmas, are in every region. Look up your
  region on rijksoverheid.nl if you are not sure; GDash does not guess it
  from the weather location, as the borders follow municipalities.
- **Show** is school and public holidays, school holidays only, or public
  holidays only.
- **Holidays at most** is how many lines, 5 when not set; a small tile shows
  what fits.

The school holidays come from the open data of the Dutch government, read
once a day and only while a dashboard shows them; when that fails, the
screen keeps the last list it had. Schools may move the days of the autumn,
spring and May holidays, for which the government only gives advice; check
with your school. The public holidays are worked out by GDash itself, with
no network: New Year's Day, Good Friday, Easter, King's Day (27 April, or
the Saturday before when that is a Sunday), Liberation Day, Ascension Day,
Whitsun and Christmas. Liberation Day is listed every year, although many
employers only give the day off once every five years. The widget needs at
least 6 by 4 cells.

## Moon

The **Moon** widget needs no setting and no network: it works out the
phase on the screen, draws the moon as it looks from the Netherlands (lit
on the right while it grows), and says how much of it is lit and when the
next full and new moon are. On the day of a full moon, new moon or quarter
it says so all day. It needs at least 4 by 4 cells; a wider tile adds the
next full and new moon.

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
2. In GDash, under Integrations, Home Assistant, enter the address you
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

## Homey

GDash can show what a Homey Pro knows, such as the temperature in the
kitchen or whether the back door is open, and who is home. It only reads;
it never switches anything. It works with a Homey Pro (2023 or later) and
with Homey Self-Hosted Server on your own network. A Homey Cloud or Homey
Bridge has no local API and cannot be connected. A Homey Pro (2016–2019)
has no API keys: it sends its data itself with HomeyScript, see
[Homey Pro (2016–2019)](#homey-pro-20162019) below.

1. Open the Homey web app at [my.homey.app](https://my.homey.app), choose
   your Homey, then **Settings**, **API Keys**, **New API Key**.
2. Give it a name, such as GDash, and tick **View devices** and **View
   zones**. Tick **View users** too if you want a Who's home widget; leave
   everything else off, so the key cannot change anything.
3. Confirm, and copy the key Homey shows. It is shown only once; if you
   lose it, make a new one.
4. Find the address of your Homey on your network, for example in your
   router's list of devices, where it is called Homey. Give it a fixed
   address in the router, so it does not change.
5. In GDash, under Integrations, Homey, enter the address, such as
   `http://192.168.1.20` (for Homey Self-Hosted Server add its port:
   `http://192.168.1.20:4859`), and paste the key. GDash tries them before
   it saves them, says which of the two is wrong, and stores the key
   encrypted. GDash does not search the network for a Homey; enter the
   address yourself.
6. Add a **Homey** widget to a dashboard and search for the values it
   should show, by room, device or name, such as "kitchen temperature",
   up to eight. With one value the widget shows it large; with more, a list
   named after the device and the value, such as "Thermostat · Temperature".

Numbers are shown with Homey's units and decimals, switches as on or off,
doors and windows as open or closed, motion as detected or clear, water as
wet or dry, locks as locked or unlocked and other alarms as alarm or OK, in
the language of the screens. A device that is offline shows a dash. The
screen updates once a minute.

The **Who's home (Homey)** widget shows a round badge for each chosen Homey
user who is home, in the accent colour of the style, like the badges of the
Custom widget. It suggests the initials of each name as the label; change
it to anything of up to 12 characters, such as a nickname. When nobody is
home it says so. Homey decides who is home, from the location of each
user's phone. The screens only receive the labels of the people who are
home: a TV never learns who is away, not even whom the widget looks for.

### Homey Pro (2016–2019)

A Homey Pro from before 2023, such as the Homey Pro (Early 2019), has no API
keys, so GDash cannot ask it for anything. HomeyScript can send its data to
GDash instead, every minute; the Homey and Who's home widgets then work as
with a newer Homey.

1. On your Homey, install **HomeyScript** from the Homey App Store.
2. In GDash, under Integrations, Homey, choose **Homey Pro
   (2016–2019): your Homey sends its data (HomeyScript)** and **Use
   HomeyScript**. GDash shows the steps and the script.
3. Choose **Make an API token**. GDash fills it into the script; copy the
   script now, as the token is shown once. (You can also make one under
   Integrations, API tokens.)
4. At [my.homey.app](https://my.homey.app), open **HomeyScript**, make a new
   script, paste the script and save it.
5. Make a Flow: When **Every 1 minute** (Date & time), Then **Run a script**
   (HomeyScript) with that script.

The Homey card then says when the last data arrived. The script sends every
device with its values and zone, and every user with whether they are home,
to `PUT /api/homey/snapshot`; the widget editor finds values and people in
the last one. When nothing arrives for three minutes, the Homey widgets mark
their values as old and keep the last ones; after a restart GDash shows the
last data it received until the next one arrives. Not every Homey version
lets a script read its users; the script then sends the devices only, and
Who's home has nobody to choose.

HomeyScript can do anything on your Homey, and the API token sits in the
script. Use a token for this script alone, so you can revoke it without
touching anything else; whoever has it can send GDash other values or
announcements, but cannot read or change anything else. With this setup
GDash keeps the whole last snapshot on disk, not only the values the screens
show, so the editor can list them; the screens still get only their own
widgets' values and the labels of the people who are home.

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
   empty and the widget uses the weather location from Integrations, so a
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

## Rain

The Rain widget shows whether it will rain in the next two hours: a bar for
every five minutes, from now on the left to two hours ahead on the right, and
one sentence in the screen's language:

- "Dry for the next 2 hours";
- "Rain from 14:20 (light)" when the first rain is still to come;
- "Rain until 15:05 (moderate)" while it rains and a dry spell follows;
- "Rain for the next 2 hours (heavy)" when it does not stop.

The word in brackets is the heaviest moment of that spell of rain:

| Word | Millimetres per hour |
|---|---|
| (dry) | below 0.1 |
| light | 0.1 to 2.5 |
| moderate | 2.5 to 10 |
| heavy | 10 and more |

A taller bar is heavier rain; a dry moment is a flat line.

1. Add a **Rain** widget to a dashboard and select it.
2. **Place**: leave it empty to use the weather location from Settings, or
   choose another town with the place search.

Good to know:

- In the Netherlands and Belgium the forecast is
  [Buienradar](https://www.buienradar.nl)'s, in steps of five minutes. Its
  data is free for non-commercial use with a credit, so the widget always
  shows "Buienradar.nl" under the bars. Elsewhere, or when Buienradar cannot
  be reached, GDash uses Open-Meteo's forecast per quarter of an hour instead,
  and the widget says "Open-Meteo.com".
- GDash asks once every 5 minutes, once for each different place your
  dashboards show (rounded to about 1 km), at most ten places. When neither
  service answers, the last forecast stays on screen and "Buienradar.nl"
  turns to the warning colour once it is more than 6 minutes old.
- Only the place, with two decimals, and GDash's name and version are sent.

## Weather warnings

The Weather warnings widget shows the warnings of the
[KNMI](https://www.knmi.nl), the Dutch weather service, for one province:
a banner per warning with its code, what it is about and when, such as
"Code orange Wind until 22:00", and the KNMI's own headline on a larger
tile. Without warnings it says "No warnings for Utrecht" in a calm style.

- **Code yellow** has a thin bar in the theme's warning colour, **code
  orange** a thicker bar and a tinted banner, **code red** a stronger tint and
  a warning sign, so it is found without telling the colours apart.
- A warning that has been issued but not started yet shows its start and end,
  such as "Wed 08:00 – 14:00". An expired warning disappears.
- The headline is the KNMI's text, in Dutch on a Dutch screen and in English
  on the others; the code and the kind of weather are in the screen's
  language.

1. Add a **Weather warnings** widget to a dashboard and select it.
2. **Province**: leave it on "From the weather location", or choose one of the
   twelve provinces or the Wadden Islands. GDash finds the province of the
   weather location by the nearest of about 150 towns; within a few kilometres
   of a provincial border, choose the province yourself to be sure.
3. **Announce code orange and red on every screen**: when a code orange or red
   warning starts, GDash shows it over all displays for five minutes, like an
   announcement, with the code and province as the title and the KNMI's
   headline as the message. Each warning is announced once; if the KNMI raises
   it from orange to red, it is announced again. A warning that is already in
   effect when GDash starts, or when you turn this on, is not announced. The
   activity log records each announcement.

Good to know:

- The warnings come from the public MeteoAlarm feed of the Netherlands, which
  carries the KNMI's warnings; GDash reads it once every 15 minutes while a
  dashboard has the widget. When it cannot be reached, the last warnings stay
  and the title turns to the warning colour once they are more than 16 minutes
  old. The widget credits "KNMI · MeteoAlarm".
- Only the Netherlands for now. MeteoAlarm publishes a feed in the same shape
  for every European country, so other countries can be added with their own
  feed address and regions.
- Warnings for the sea, the IJsselmeer and the harbours are left out: they are
  for shipping.

## Custom APIs

The Custom widget shows data from any API that answers with JSON: a public
service, a paid one with an API key, or a device on your own network. You
set it up in the web interface; no code and no restart.

### Add the API

Under **Integrations, Custom APIs**, choose **Add a custom API**:

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

### Start from a preset

For a number of devices and services GDash knows the address, the
authentication and the values, so you only fill in what is yours. Under
**Integrations**, the cards marked **Preset · Custom API** do this:

| Preset | You fill in |
|---|---|
| HomeWizard P1 meter, kWh meter, water meter | The meter's IP address; turn on Local API in the HomeWizard Energy app |
| P1 Monitor, DSMR-reader | The IP address; DSMR-reader also its API key |
| Fronius inverter | The inverter's IP address |
| SolarEdge | The site ID and the API key of the monitoring portal; refreshes every 15 minutes, as an account may make 300 requests a day |
| Tibber, Electricity Maps | A personal token or free key; Electricity Maps also the zone, such as NL |
| Quatt heat pump | The CIC's IP address; Quatt calls this address beta, so an update may change it |
| AdGuard Home, Speedtest Tracker | The address, and the login or an API token |
| Jellyfin, Plex | The server's address and an API key or X-Plex-Token |
| Mealie, Todoist | The address and an API token; Todoist also a filter, such as `today` |

GDash checks what you type before it saves, makes the custom API with its
named values and asks it once, so a wrong address shows at once. A preset
may suggest a rule for the widget, such as Power now in the warning colour
above 3000 W. Change anything later under Custom APIs. The presets are JSON
files in `web/src/admin/presets`; each names the documentation it was
checked against.

### Send data to GDash (push)

Some systems cannot answer requests, but can send data out on a schedule: a
Homey Pro (Early 2019) with HomeyScript, Node-RED, a Home Assistant
automation or a shell script on a timer. Let them send their JSON to GDash:

1. Under **Integrations, Custom APIs**, choose **Add a custom
   API**, and under **How does GDash get the data?** choose **Your system
   sends it to GDash**. There is no address or authentication to fill in.
2. Choose **Expected every**: how often your system sends, such as every
   minute. When nothing arrives for twice that and 30 seconds, the widget's
   title turns to the warning colour and it keeps the last values.
3. Save. GDash shows the address to send to, such as
   `http://192.168.1.2:8420/api/custom-sources/3/data`, and examples for
   curl, HomeyScript and Home Assistant. Choose **Make an API token**: it is
   filled into the examples and shown once, so copy the example now. You
   can also make one under Integrations, API tokens.
4. Let your system send the JSON with `PUT`, the header
   `Authorization: Bearer <API token>` and `Content-Type: application/json`.
   GDash answers `204` when it has it. **How to send** shows the steps
   again later, and the list says when the last data arrived.
5. **Last data received** shows what your system sent as a tree; name its
   values and use them in a widget as with any other API.

The data may be any JSON of up to 1 MB, and GDash takes at most one per
source every 5 seconds; once a minute is plenty for a TV. What arrives is
handled exactly like a fetched answer: it is kept on disk, rules and
announcing rules apply to it, and the screens only get what their widgets
show.

**Homey Pro (Early 2019).** This Homey has no API keys, so GDash cannot ask
it; HomeyScript can send instead. For the Homey and Who's home widgets, set
it up under Homey instead, see [Homey Pro (2016–2019)](#homey-pro-20162019);
a custom API suits a shape of your own, such as Logic variables, for the
Custom widget and its rules.

1. Install **HomeyScript** from the Homey App Store.
2. In GDash, add a custom API that your system sends, as above, and make an
   API token. Copy the HomeyScript example: it sends every device's values
   by device name, with its zone, and who is home by user name, such as
   `{"devices": {"Thermostat": {"zone": "Living room", "measure_temperature": 20.5}}, "home": {"Anna": true}}`.
3. At [my.homey.app](https://my.homey.app), open **HomeyScript**, make a new
   script, paste the example and save it.
4. Make a Flow: When **Every 1 minute** (Date & time), Then **Run a script**
   (HomeyScript) with that script.

Not every Homey version lets a script read who is home
(`Homey.users.getUsers()`); the example then sends the devices only.
Presence can also come from a Logic variable or a Flow tag that you add to
the script. HomeyScript can do anything on your Homey, and the token sits
in the script: use a token for this script alone, so you can revoke it
without touching anything else.

**Node-RED.** An **inject** node that repeats every minute, a **function** or
**change** node that builds `msg.payload` as an object, and an **http
request** node with method PUT, the address from GDash, and under headers
`Authorization` set to `Bearer <API token>`. Node-RED sends an object as
JSON with the right content type.

**Home Assistant.** A `rest_command` with `method: put`, the address, the
`Authorization` header and `content_type: "application/json"`, and an
automation with a `time_pattern` trigger that runs it every minute; GDash
shows the full example. Keep the token in `secrets.yaml`, as in the
[announcements example](#home-assistant-example).

### Name its values

Still under Custom APIs, **Values** next to the API names the values a
dashboard can show, so the dashboard editor offers "Outside" instead of
`current.temperature_2m`:

1. **Use the sample** or **Fetch the API now** shows the answer as a tree.
2. Click a value to add it. It gets a name from its key, such as
   `Temperature 2m`; change it to what you want to read in the editor, and
   add a unit (`°C`) or a number of decimals if you like.
3. For a list of items, choose **Use this list** next to it, then choose
   **A column to** that list and click up to three values inside one item,
   such as its name and price.
4. **Save values.**

A value can also be added by typing its path (see [Paths](#paths)). Names
are up to 40 characters and different for each value of an API. Renaming or
removing a value later does not change a dashboard: a widget keeps the path
it was given, and the editor then shows "(custom path)".

### Example: the temperature from Open-Meteo, no key

1. Add a custom API named `Weather in Oss` with the address
   `https://api.open-meteo.com/v1/forecast?latitude=51.77&longitude=5.52&current=temperature_2m,wind_speed_10m`,
   no authentication, refreshed every 15 minutes. **Test** shows
   `{"current": {"time": "2026-10-12T10:00", "temperature_2m": 14.2, "wind_speed_10m": 11.3, …}, …}`.
2. Under **Values**, fetch the API and click `temperature_2m`. Name it
   `Outside`, with the unit `°C` and 1 decimal, and save the values.
3. In a dashboard, add a **Custom API** widget (under Home) and select it.
   Choose the API, **Show: One value** and **Value: Outside**, and give it
   the label `Outside`.
4. Add a rule: **When** the value **is below** `0`, colour **Bad**, message
   `Frost: mind the roads`.
5. Save. The TV shows `14.2 °C`, in the warning colour with the message on
   frosty mornings.

A rule that the sentence cannot say, such as "the weather is more than an
hour old", goes under **Advanced: write the condition** of a rule:
`minutesSince(current.time) > 60`. (Open-Meteo gives times in UTC unless you
ask for a time zone; a time without a zone counts as UTC.)

### Example: OpenWeatherMap, with an API key

1. Make a free key at [openweathermap.org](https://openweathermap.org/api).
2. Add a custom API with the address
   `https://api.openweathermap.org/data/2.5/weather?q=Oss,NL&units=metric`,
   **Authentication: API key in the address**, parameter name `appid` and
   your key. GDash adds `appid=<key>` to the address when it fetches; the
   key never appears in the address that is stored or shown.
3. Under **Values**, click `main.temp`, `main.feels_like`, `main.humidity`
   and `weather[0].description`, and name them `Temperature`, `Feels like`,
   `Humidity` and `Sky`.
4. In a Custom widget choose **Show: Labelled values** and add a line for
   each: pick the value, and its label is filled in with its name.

### Badges: who is home

**Show: Badges** shows a round badge with a short label, such as initials,
for every value that is on, and nothing for the others. A value is on when
it is `true`, a number other than 0, or the text `on`, `home`, `yes`, `open`
or `true` (in any case), so it works with Home Assistant's `home` and `on`
states as well as with plain true and false.

For an API that answers
`{"home": {"aanwezigheid_anna": true, "aanwezigheid_bram": false, …}}`, name
`home.aanwezigheid_anna` `Anna`, `home.aanwezigheid_bram` `Bram` and so on
under Values. In the widget, **Add a badge** per person and pick the value:
the label becomes the initials, `AN`, which you can change. Up to eight. The TV shows `AN` while
Anna is home; when nobody is home it shows a dash. Only the badges that are
on are sent to the screens, so a TV never learns who is away.

### Lists

**Show: A list** shows one row per item of a list in the answer, with up to
three values per row: the columns of a list named under Values. A rule on
**each item's** column, such as "each item's Price is above 2", is checked
for every item and colours or hides that row. For example, the latest Home
Assistant releases from GitHub's open API: address
`https://api.github.com/repos/home-assistant/core/releases?per_page=5`; under
Values, **Use this list** on the answer itself (its path is `$`) and add the
columns `tag_name` and `published_at`. An advanced rule
`minutesSince(row.published_at) < 1440` with colour Good marks the releases
of the last day.

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

In the editor a rule reads as a sentence: **When** the value, one of the
API's named values or each item's column, **is above**, **is below**,
**equals**, **is not**, **contains** (ignoring case), **is on** or **is
off** (as for a badge), and the number or text to compare with. Above and
below also read numbers that an API sends as text. Anything more goes under
**Advanced: write the condition**, in a small expression language.

Written conditions use:

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
or `number(row.delay) >= 5`; `isOn(x)` is what **is on** writes. A
condition that does not apply to the data, such as `value > 3` while the
value is missing, simply does not match. **Check against the answer** under
Advanced evaluates a condition against the sample, or a fresh answer when
the API has no sample.

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
time you choose. The same API token can also send the data of a custom API
that your system sends to GDash (see [Send data to GDash](#send-data-to-gdash-push))
and a Homey's snapshot ([Homey Pro (2016–2019)](#homey-pro-20162019));
nothing else in GDash can be changed with it.

1. In GDash, under Integrations, **API tokens**, make a token with the name of
   the automation, for example "Home Assistant". Copy it now: it is shown once,
   and GDash keeps only a fingerprint of it. You can make up to 10 and
   revoke any of them later; the list shows when each was last used.
2. Try it under Settings, Screens, **Send a test announcement**. The message shows on
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

If nothing shows up, look at the token under Integrations, API tokens:
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
  when, whether the TV sleeps, something else is on it, someone is using it
  (see [When GDash is not on the TV](#when-gdash-is-not-on-the-tv)), or
  GDash cannot reach it.
- **Integrations** shows on every card whether it is connected and when it
  last updated, or why it fails.
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

GDash does not fight for the TV. Once it has put the dashboard on screen,
it notices when someone takes the TV away from it:

- the dashboard is closed with Back or Home on the remote, or an app such
  as Netflix or YouTube is opened on a Google TV;
- someone casts to the TV, such as YouTube from a phone, or another GDash
  server or DashCast user opens its own page;
- the TV switches to another input, such as a game console (only TVs with
  HDMI-CEC say so).

The display list then says **Someone is using the TV; GDash waits until it
is switched off**. GDash starts nothing and does not restart the dashboard
against the screensaver. When the TV has been in standby and is switched
on again, the dashboard appears within about 30 seconds. The same happens
when the device shows its own idle screen, such as the photo frame of a
Nest Hub or the ambient screen of a Chromecast, for 10 minutes without a
break (on an awake TV, not on another input): whatever played has ended.
Switching back to the TV's Cast input while the dashboard still runs there
also counts as giving it back. To have it back at once, choose **Show
dashboard now** at
the display: it stops whatever is on the TV and switches the TV to the
dashboard.

Two GDash servers that cast to the same TV therefore settle on one: the
one that cast last keeps it, the other waits.

Some situations GDash cannot tell apart:

- **When GDash starts**, for example after an update, it remembers which
  cast it started itself (in `cast-sessions.json` in the data directory)
  and keeps that dashboard on screen as before. Something else on the TV,
  or a cast it does not recognise, such as one from before the first start
  of this version, it leaves alone and waits as above, until the TV has
  been off or you choose **Show dashboard now**.
- **A TV without HDMI-CEC, or a Nest Hub,** never reports standby, so a
  wait there ends after 10 minutes of the device's idle screen, or with
  **Show dashboard now**. A Google TV that reports no apps at all does not
  count as idle, because it reports the same while Netflix plays.
- **Google TV does not report its own apps** to GDash. When the dashboard
  is not on screen, for example when the TV is switched on, a free Google
  TV and one playing Netflix look the same, and GDash starts. Use active
  hours to keep the dashboard away when the TV is mostly used for watching.

While nothing is on the TV and GDash is not waiting, it starts the
dashboard within about 30 seconds; when someone else's cast was already on
screen, it starts after that cast ends. It never starts while the TV shows
another input, and leaves a TV in standby alone, because starting a cast
could switch it on.

More answers are in the troubleshooting section of [install.md](install.md).
