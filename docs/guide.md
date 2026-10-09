# Using Nookboard

This guide assumes Nookboard is installed and you created your account with
the setup token, see [install.md](install.md). The web interface is at
`http://<server>:8420/`.

## First steps

The **Getting started** card at the top lists what is left to do; it
disappears when the basics are done.

1. **Settings, Weather location:** type your town or postcode and choose the
   right result, or enter the coordinates. The weather service only receives
   the location rounded to about 1 km.
2. **Settings, News and Calendars:** add RSS or Atom feeds, and the secret
   iCal address of each calendar you want on the screen. In Google Calendar
   it is under the calendar's settings, "Secret address in iCal format"; in
   Outlook under "Publish a calendar"; in iCloud under "Public calendar".
   Nookboard stores the address encrypted and never shows it again. Each
   calendar gets its own colour on the TV, so you can see whose appointment
   it is; choose another one next to the calendar.
3. **Settings, Language on the TVs:** English or Nederlands.
4. **Displays, Add a display:** give it a name, such as Kitchen, and pick
   your TV from the list of Cast devices Nookboard found.

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
Small screen (three large widgets for a Nest Hub or tablet) and Family (a large
calendar, photos, a countdown to the next New Year and a note). Each card shows
the layout; the button adds it as a new dashboard and opens it in the editor,
where you can change anything. The calendar and news widgets fill up once you
add calendars and feeds under Settings, and the photos once you add photos.

- **Add** a widget with the buttons above the canvas, drag it to move it and
  drag its corner to resize it. Click a widget to change its settings, such
  as the number of headlines or the text of a note.
- **Themes:** Minimal dark (the default, best for OLED screens), Minimal
  light and Warm light for bright rooms, Glass, High contrast for reading
  from across the room or with poor eyesight, and Night, dim amber for a
  bedroom.
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
| Calendar | The coming week from your calendars |
| News | Headlines from your feeds |
| Note | A short text for the household |
| Countdown | The days until a date, such as a holiday |
| Photos | A slideshow of your photos |
| Home Assistant | Up to eight sensors or switches from Home Assistant |

**Photos** are added under Settings, Photos: JPEG, PNG or WebP, up to 8 MB
each and 50 in total. They stay on your server and only go to paired
screens and to you; the rest of the network gets nothing.

Every display shows the **Default** dashboard until you choose another one
in the list at the display.

**Export** saves a dashboard as a file, as a backup or to share a layout.
**Import a dashboard** adds such a file as a new dashboard, also in another
Nookboard. The file holds the layout, the theme and the widget settings, such
as the text of a note; it holds no data and no calendar addresses.

## Home Assistant

Nookboard can show what Home Assistant knows, such as the temperature
inside, whether the front door is open or the power used right now. It
only reads; it never switches anything.

1. In Home Assistant, open your profile, Security, and make a **long-lived
   access token**. A separate Home Assistant user for Nookboard, without
   admin rights, limits what the token can see.
2. In Nookboard, under Settings, Home Assistant, enter the address you
   open Home Assistant with, such as `http://192.168.1.10:8123`, and the
   token. Nookboard tries them before it saves them, and stores the token
   encrypted.
3. Add a **Home Assistant** widget to a dashboard and search for the
   entities it should show, up to eight. With one entity the widget shows
   its value large; with more, a list.

Numbers are shown with their unit, doors and windows as open or closed,
motion as detected or clear, people as home or away. The screen updates
once a minute. Home Assistant's own dashboards remain the place to control
things; Nookboard is the calm screen that shows the day.

## Small screens

A layout is made for a TV across the room. On a Nest Hub, a tablet or a
small monitor, choose a larger **text size** in the list at the display:
125, 150 or 200 %. Everything is drawn larger and lists show fewer items,
so nothing is cut off. The dashboard always fills the screen, also on
16:10 and 4:3 screens.

## Schedules

Choose **Schedule** at a display:

- **Active hours,** such as 06:30 to 23:00. Outside them Nookboard stops the
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
  when, whether the TV sleeps, another app is on it, or Nookboard cannot reach
  it.
- **Settings** shows per news feed, calendar and the weather when it last
  updated, or that it fails.
- **Settings, Recent activity** lists logins, failed logins, pairings and
  changes of the last 90 days.

## When Nookboard is not on the TV

Nookboard does not take over a TV that someone casts to, such as YouTube from
a phone: it waits, and the dashboard returns within about 30 seconds after
the cast stops. It also leaves a TV in standby alone, because starting a
cast could switch it on; when someone switches the TV on, the dashboard
appears within about 30 seconds. Google TV does not report its own apps, such as Netflix,
to Nookboard, so there it cannot tell; use active hours to keep the dashboard
away when the TV is mostly used for watching.

More answers are in the troubleshooting section of [install.md](install.md).
