# GDash

Your day on every TV in the house. Self-hosted, casts to the Chromecast or
Google TV you already own. No extra hardware, no subscription, no cloud.

This app runs the [GDash](https://github.com/stijnvandepol/gdash) server next
to Home Assistant:

- Clock, weather, forecast, calendar, news, photos and notes on the TV, with
  twenty-one styles and a drag-and-drop dashboard editor.
- Values from Home Assistant on the TV, with a long-lived access token. Read
  only.
- Announcements: an automation (a `rest_command`) puts "The washing machine is
  done" over the dashboard of every TV.
- Finds Cast devices by itself, so the app runs on the host network and its
  web interface is on port 8420.

The app is experimental until it has run on more Home Assistant installations.
Read the **Documentation** tab for the first start, the setup token and pairing
TVs.
