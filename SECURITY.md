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
  and removed from the screen on time.
- **Photos:** only served to logged-in admins and paired screens; an
  upload is accepted by its content (JPEG, PNG or WebP), not by its name.
  Location, camera, date and other metadata (EXIF, XMP, IPTC, text chunks)
  are removed before a photo is stored; only a JPEG's orientation stays.
- **Outgoing requests:** feeds, calendars and Home Assistant are fetched with a
  client that refuses link-local addresses, where cloud hosts serve instance
  metadata.
- **Web interface:** a strict Content-Security-Policy without inline scripts,
  cross-site request forgery protection, and no third-party scripts except
  Google's Cast framework on the receiver page.
- **Audit log:** logins, failed logins, pairings and changes are recorded
  for 90 days and shown under Settings.
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
- Anyone who can read the server's log during the first start can create the
  first account with the setup token printed there, and anyone who can run
  commands on the server can reset a password with `gdash reset-password`.
  Both already control the server.
- There is one kind of account, the household's admin, without two-factor
  authentication: GDash is meant for a home network behind its router, and
  sessions last 7 days idle so a TV-side admin is not asked to log in daily.
  Put it behind a reverse proxy with its own login if you expose it.
- Cast itself does not authenticate senders. With an own registered receiver
  (`GDASH_CAST_APP_ID`), a device on the network could send that receiver a
  made-up dashboard; it only accepts dashboards, shown as plain text, never
  an address to go to. The default DashCast route has no such channel.

The reasons for these choices are in [CODE_GUIDELINES.md](CODE_GUIDELINES.md)
under "Known oddities".
