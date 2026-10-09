# Security

## Reporting a vulnerability

Please report security problems privately through GitHub: on the
repository's **Security** tab, choose **Report a vulnerability** (available
once the repository is public). Do not open a public issue for them. The aim
is to answer within a week and to fix a critical or high finding within
seven days of confirming it.

Only the latest release gets security fixes.

## What Nookboard protects, and how

Nookboard runs on a home network and shows a household's calendar and other
data on its TVs. The design assumes the LAN can contain devices you do not
fully trust, such as a guest's phone.

- **Accounts:** passwords are hashed with argon2id. Logins are rate limited
  per address and per username. Sessions expire after 7 days idle or 30 days
  in total, and changing the password logs out every other browser.
- **Screens:** a display only streams data to a screen that was paired with
  a code shown on it, or opened by Nookboard's own cast with a single-use token
  that expires after two minutes. Unpairing takes effect within 20 seconds,
  and a screen that has not shown its display for 90 days is unpaired.
  Each display only receives the data its own widgets show.
- **Secrets:** calendar addresses give read access to a calendar, and a
  Home Assistant token to everything its user can see, so they are
  encrypted (AES-256-GCM) with a key file next to the database, never
  returned by the API and never logged. Nookboard only reads from Home
  Assistant.
- **API tokens:** automations such as Home Assistant post announcements with
  a token made under Settings. Only its SHA-256 hash is stored, it is shown
  once, there are at most 10, and revoking one works at once. A token opens
  the announcement route only, wrong tokens block the sending address for 15
  minutes, each token is limited to 30 announcements a minute, and tokens
  and messages are never logged. Announcements are plain text, kept in memory
  and removed from the screen on time.
- **Photos:** only served to logged-in admins and paired screens; an
  upload is accepted by its content (JPEG, PNG or WebP), not by its name.
- **Outgoing requests:** feeds, calendars and Home Assistant are fetched with a
  client that refuses link-local addresses, where cloud hosts serve instance
  metadata.
- **Web interface:** a strict Content-Security-Policy without inline scripts,
  cross-site request forgery protection, and no third-party scripts except
  Google's Cast framework on the receiver page.
- **Audit log:** logins, failed logins, pairings and changes are recorded
  for 90 days and shown under Settings.

## Known limits

- The web interface and the display pages use **plain HTTP on the LAN**. A TV
  that opens the dashboard through Cast cannot be given a certificate it
  trusts for a LAN address. Put a reverse proxy with HTTPS in front of the
  web interface if you open it from outside, set `NOOKBOARD_TRUSTED_PROXIES`
  to its address, and never expose port 8420 to the internet. Forwarding
  headers from any other address are ignored.
- Nookboard does not verify the TLS certificates of Cast devices: they use
  Google's device CA, which no trust store contains. Home Assistant and
  pychromecast do the same.
- Anyone who can read the server's log during the first start can create the
  first account with the setup token printed there.

The reasons for these choices are in [CODE_GUIDELINES.md](CODE_GUIDELINES.md)
under "Known oddities".
