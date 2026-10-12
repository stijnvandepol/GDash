# Security

## Reporting a vulnerability

Please report security problems privately through GitHub: on the
repository's **Security** tab, choose **Report a vulnerability** (available
once the repository is public). Do not open a public issue for them. The aim
is to answer within a week and to fix a critical or high finding within
seven days of confirming it.

Only the latest release gets security fixes.

## What GDash protects, and how

GDash runs on a home network and shows a household's calendar and other
data on its TVs. The design assumes the LAN can contain devices you do not
fully trust, such as a guest's phone.

- **Accounts:** passwords follow NIST SP 800-63B: at least 12 characters,
  up to 128, not the user name and not a well-known password, without rules
  about kinds of characters. They are hashed with argon2id. Logins are rate
  limited per address and per account from that address, and per account
  over all addresses, so a device on the network cannot lock the admin out.
  Sessions expire after 7 days idle or 30 days in total, and changing the
  password logs out every other browser.
- **Two-step login (optional):** a 6-digit code from an authenticator app
  (RFC 6238: HMAC-SHA-1, 30 seconds) after the password. The right password
  alone gives no session, only a pending login for five minutes. A code of
  the current, previous or next 30 seconds is accepted, and each time step
  only once, so a code seen over a shoulder cannot be replayed. Wrong codes
  count towards the login limits per address and per account, and a right
  password, at login or confirmed again in a session, does not clear them.
  On top of that, codes from the app count per account over all addresses,
  so changing address buys no extra guesses: a pending login ends after 5
  codes; after 5 wrong codes in a row the next one waits 2, 4, 8, 16 and 32
  minutes, then an hour each time (at most 10 in any hour); and after 50 in
  a row (NIST SP 800-63B allows up to 100) codes from the app are refused
  until a recovery code is used or `gdash reset-password` runs. A guesser
  with the password thus has at most a 0.015% chance, after about 40 hours.
  Only a right code, a recovery code or turning it off ends a run of wrong
  codes; the count is stored, so a restart does not. Codes are only checked
  after the right password, so nobody without it can cause a wait or the
  stop. Recovery codes cannot be guessed and are not held up, so the owner
  always has a way in. The secret is encrypted with the key file (see Secrets), never
  logged and only shown while it is set up; setting it up asks for the
  password again and only takes effect after a first code, and then logs out
  every other browser. Ten recovery codes of 80 bits are shown once and
  stored as SHA-256 hashes; each works once. Turning it off needs the
  password and a code. Wrong codes, recovery codes, reaching the stop and
  every change are in the audit log. `gdash reset-password` on the server turns it off; that is
  the way back for an admin who lost the phone and the recovery codes.
- **Screens:** a display only streams data to a screen that was paired with
  a code shown on it, or opened by GDash's own cast with a single-use token
  that expires after two minutes. Unpairing takes effect within 20 seconds,
  and a screen that has not shown its display for 90 days is unpaired.
  Each display only receives the data its own widgets show.
- **Secrets:** calendar addresses give read access to a calendar, and a
  Home Assistant token to everything its user can see, so they are
  encrypted (AES-256-GCM) with a key file next to the database, never
  returned by the API and never logged. GDash only reads from Home
  Assistant.
- **API tokens:** automations such as Home Assistant post announcements with
  a token made under Settings. Only its SHA-256 hash is stored, it is shown
  once, there are at most 10, and revoking one works at once. A token opens
  the announcement route only, wrong tokens block the sending address for 15
  minutes, each token is limited to 30 announcements a minute, and tokens
  and messages are never logged. Announcements are plain text, kept in memory
  and removed from the screen on time. Tokens keep working when two-step
  login is turned on: they only reach the announcement route, never the
  web interface or the rest of the API.
- **Photos:** only served to logged-in admins and paired screens; an
  upload is accepted by its content (JPEG, PNG or WebP), not by its name.
  Location, camera, date and other metadata (EXIF, XMP, IPTC, text chunks)
  are removed before a photo is stored; only a JPEG's orientation stays.
- **Outgoing requests:** feeds, calendars, Home Assistant and custom APIs are
  fetched with a client that refuses link-local addresses, where cloud hosts
  serve instance metadata. Departures are fetched from OVapi over plain HTTP, because its
  HTTPS certificate belongs to another domain; the request carries no secret
  and the answer is public timetable data.
- **Custom APIs:** an admin can point GDash at any http or https address,
  on the internet or the LAN (a device or Home Assistant at home), so an
  admin session can make the server send requests into the home network;
  only link-local and cloud metadata addresses are refused. A source is
  only fetched while a dashboard shows it, at most every 30 seconds, with
  15 seconds per request and an answer of at most 1 MB of JSON. Redirects
  are only followed to the same host, never from https to http. Tokens,
  API keys, passwords and secret headers are encrypted with the key file,
  never returned by the API, logged or put in errors or the audit log
  (which names a source by name and host); an API key for the address is
  added when fetching and never stored in it. Screens never get an API's
  answer, only the values their own widgets show, as text. Rules are not
  scripts: they are parsed into a fixed set of comparisons, calculations
  and seven functions, without loops, assignment or access to anything but
  the answer, bounded in length and nesting, and fuzz tested, so neither
  the server nor a TV ever runs code from the web interface. An announcing
  rule posts at most once every 10 minutes and is in the audit log.
- **Web interface:** a strict Content-Security-Policy without inline scripts,
  cross-site request forgery protection, and no third-party scripts except
  Google's Cast framework on the receiver page.
- **Audit log:** logins, failed logins, pairings and changes are recorded
  for 90 days and shown under Settings.
- **Backups:** the downloadable backup holds the encryption key, so it asks
  for the password again (rate limited like a login) and every download, or
  refusal, is in the audit log. The daily copies in the data directory have
  mode 0600.
- **Container and service:** the Docker image is a distroless static image
  (no shell, no package manager) that runs as the fixed user `65532`, never
  as root. Its data directory is mode 0700, and the example `compose.yaml`
  adds a read-only root filesystem, no capabilities and `no-new-privileges`.
  The systemd unit runs GDash as a throwaway user with only its data
  directory writable, no capabilities, a system call filter and restricted
  socket types. A data directory GDash cannot write to is reported at the
  start. See [docs/install.md](docs/install.md#data-directory-permissions).

## Supply chain

- Go modules, npm packages, GitHub Actions and the base images of the
  Dockerfile are pinned (by lock file, commit SHA and image digest), and
  Dependabot proposes updates weekly. CI scans Go and npm dependencies for
  known vulnerabilities, runs `staticcheck`, and scans the whole git history
  for secrets with gitleaks.
- Releases come with `SHA256SUMS` and a CycloneDX SBOM; the Docker image has
  an SBOM and BuildKit provenance too. Signed build provenance
  (`gh attestation verify`) is added by the release workflow as soon as the
  repository is public, because GitHub does not offer it for private
  repositories on a free plan. Releases are drafts until a maintainer
  publishes them.

## Known limits

- The web interface and the display pages use **plain HTTP on the LAN**. A TV
  that opens the dashboard through Cast cannot be given a certificate it
  trusts for a LAN address. Put a reverse proxy with HTTPS in front of the
  web interface if you open it from outside, set `GDASH_TRUSTED_PROXIES`
  to its address, and never expose port 8420 to the internet. Forwarding
  headers from any other address are ignored.
- GDash does not verify the TLS certificates of Cast devices: they use
  Google's device CA, which no trust store contains. Home Assistant and
  pychromecast do the same.
- Until the first account exists, anyone on the home network can create it in
  the first hour after GDash starts, without a token, as with Home
  Assistant's onboarding: the person who installs GDash opens it right away.
  After that hour, from an address that is not private, or through a reverse
  proxy that is not listed in `GDASH_TRUSTED_PROXIES` (a request with
  `X-Forwarded-For`, `Forwarded` or `X-Real-IP`), it takes the setup token
  printed in the log. Only the TCP peer address counts. A router port
  forward to Docker's port mapping can still make an internet client look
  private, so do not expose a fresh install to the internet before creating
  the account. Anyone who can read the server's log
  can create the first account with the setup token printed there, and anyone who can run
  commands on the server can reset a password with `gdash reset-password`.
  Both already control the server.
- There is one kind of account, the household's admin. Two-step login is
  optional and off by default: GDash is meant for a home network behind its
  router. Sessions last 7 days idle so a TV-side admin is not asked to log in
  daily; a session already open is not asked for a code again. Put GDash
  behind a reverse proxy with its own login if you expose it.
- Cast itself does not authenticate senders. With an own registered receiver
  (`GDASH_CAST_APP_ID`), a device on the network could send that receiver a
  made-up dashboard; it only accepts dashboards, shown as plain text, never
  an address to go to. The default DashCast route has no such channel.

The reasons for these choices are in [CODE_GUIDELINES.md](CODE_GUIDELINES.md)
under "Known oddities".
