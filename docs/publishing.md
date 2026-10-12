# Making the repository public

A checklist for the day the repository goes from private to public. The order
matters: some steps are hard to undo (everything in the git history becomes
public) and some only work once the repository is public (signed build
provenance, private vulnerability reporting for outsiders, the Home Assistant
app repository).

Nothing here has been done yet.

## 1. Before anything is public

### Personal data

The git history is public too, so look at the files **and** the history. A
search of the current files on 12 October 2026 found the following. Decide per
item whether to change it; `docs/progress.md` is the owner's own notebook and
was not edited for this list.

| Where | What | Suggestion |
|---|---|---|
| `docs/progress.md:101` | the Google Home name of the kitchen TV, "tv woonkamer", and the first name of the owner (also at `:74` and `:105`) | Say "the kitchen TV"; "the owner" instead of the name |
| `docs/progress.md:103` | the Nest Hub's name ("Keuken") and its home IP address 192.168.1.33 | Drop the address, keep "a Nest Hub" |
| `docs/progress.md:105` | the program path and Windows Firewall detail of the home machine (`daycast.exe`), and "left to Stijn" | Shorten to what was learned |
| `docs/progress.md:41` and `:43` | a Tailscale route at home is mentioned as the likely cause of a network hiccup | Harmless, but it says how the home network is built |
| `docs/api.md:272` | an example device with the real name "tv woonkamer", model and 192.168.1.160:8009 | Use "Kitchen TV" and `192.168.1.20:8009` |
| `docs/api.md:189` | the user name `stijn` in an example | Use `admin` |
| `internal/pairing/pairing_test.go`, `internal/cast/address_test.go` and others | 192.168.1.160 (a real TV), 192.168.1.33, 192.168.1.109, 192.168.1.66 and a Tailscale address (100.75.74.66) as test data; the user name `stijn` in many tests | Private addresses are not secret, but a test that says what the home network looks like is better with `192.0.2.x`/`198.51.100.x` (documentation addresses) and a neutral name; this touches test code only |
| `README.md:66` and `CODE_GUIDELINES.md:235` | a link to the older `GoogleHome-Dashboard` repository with the research | Make that repository public first (check it for personal data in the same way), or remove the link |
| every commit | the author address `Stijnvdpol0@gmail.com` in all 240 commits | Public once the repository is. GitHub can hide it for new commits: Settings, Emails, "Keep my email addresses private", then use the `ID+stijnvandepol@users.noreply.github.com` address in `git config user.email`. Old commits keep the address unless the history is rewritten, which changes every commit hash and the tags |
| `docs/images/*.jpg` | looked at on 12 October 2026: made-up values ("Sam", "Living room"), user name `ha-test` | Nothing to do; check again if a screenshot is replaced |

Commands to repeat the search (adapt the patterns to what you want to find):

```bash
grep -rnE "192\.168\.[0-9]+\.[0-9]+|100\.(6[4-9]|[7-9][0-9]|1[01][0-9]|12[0-7])\.[0-9]+\.[0-9]+" --exclude-dir=node_modules --exclude-dir=.git .
grep -rniE "stijn|woonkamer|keuken" --exclude-dir=node_modules --exclude-dir=.git . | grep -v "github.com/stijnvandepol"
git log --format='%an <%ae>' | sort | uniq -c
```

### Secrets and licenses

- [ ] The CI job **secrets** (gitleaks over the whole history) passes. Anything
  it reports must be rotated, not just deleted.
- [ ] No `.env`, database, key or binary is committed (`git ls-files` shows
  none today).
- [ ] `LICENSE` (AGPL-3.0) and `THIRD_PARTY_LICENSES` are current: CI checks the
  second.
- [ ] The name: `docs/progress.md` still lists a trademark check (BOIP, EUIPO)
  as open, and the names `ripienaar/gdash` and `kadalu/gdash` exist on GitHub.
- [ ] Put a way to contact you on the GitHub profile, as `CODE_OF_CONDUCT.md`
  points there for reports.

## 2. Rename the repository

- [ ] On GitHub: Settings, General, Repository name: `gdash`. Old links to
  `daycast` keep redirecting.
- [ ] Update the local clone's remote (a clone does not change by itself):

  ```bash
  git remote set-url origin https://github.com/stijnvandepol/gdash.git
  git remote -v
  ```

- [ ] Check that the name is the same everywhere: the Go module is already
  `github.com/stijnvandepol/gdash`, the Dockerfile label, `docs/install.md`,
  `compose.yaml`, `SECURITY.md` and the issue templates all say
  `stijnvandepol/gdash`.
- [ ] Rename before the next release: `release.yml` pushes the image to
  `ghcr.io/<owner>/<repository>` in lower case, so it is `ghcr.io/stijnvandepol/daycast`
  until the rename and `ghcr.io/stijnvandepol/gdash` after it.

## 3. The container image

The 0.6.0 image was built before the rename, so it exists as
`ghcr.io/stijnvandepol/daycast:0.6.0`, **not** as `ghcr.io/stijnvandepol/gdash`
that `docs/install.md`, the example compose file and the Home Assistant app
use. A package keeps its old name when a repository is renamed. Fix it in this
order, after the repository is renamed (step 2) and public (the first item of
step 4), so the new image and archives also get signed provenance. So in
practice: steps 1 and 2, the first item of step 4, then step 3, then the rest of
step 4 and step 5.

- [ ] Remove the 0.6.0 draft release and its tag, then push the same tag again
  so the Release workflow builds the archives and the image under the new name
  and, as the repository is public by then, attaches signed provenance:

  ```bash
  commit=$(git rev-list -n 1 v0.6.0)       # the released commit, "release: 0.6.0"
  gh release delete v0.6.0 --yes --cleanup-tag
  git tag -d v0.6.0
  git tag -a v0.6.0 "$commit" -m "GDash 0.6.0"
  git push origin v0.6.0
  ```

  The tag points at the same commit as before (70f128c at the time of
  writing), so the code is the same; only the name of the image and the
  attestations are new. The archives of the second build may differ byte for
  byte from the first, so use the checksums of the published one.
- [ ] Check the result: Actions, **Release**, green; the new draft release has
  four archives, `SHA256SUMS`, the SBOM and attestations; and
  `docker pull ghcr.io/stijnvandepol/gdash:0.6.0` works.
- [ ] Make the new package public: your profile, Packages, `gdash`, Package
  settings, Change visibility, Public. A package made by a workflow is private
  by default, and a private image cannot be pulled by anyone else, or by the
  Home Assistant app. Link it to the repository (Package settings, Connect
  repository) if it is not linked.
- [ ] Remove the old `daycast` package once nothing uses it.
- [ ] `docker run --rm ghcr.io/stijnvandepol/gdash:0.6.0 version` on a machine
  that is not logged in to ghcr confirms it is public.

## 4. Settings that need a public repository (or are free on one)

Do these right after the switch, in Settings of the repository.

- [ ] General, Danger zone: **Change visibility** to public. Read the list GitHub
  shows. Do it only when sections 1 to 3 are done.
- [ ] Code security: turn on **Private vulnerability reporting**.
  `SECURITY.md` tells people to use it.
- [ ] Code security: turn on **Dependabot alerts** and **Dependabot security
  updates**. The weekly version updates already run from
  `.github/dependabot.yml`.
- [ ] Code security: turn on **Secret scanning** and **Push protection**
  (free on public repositories).
- [ ] Actions, General: "Fork pull request workflows", choose to require
  approval for first-time contributors (the safest option is for all outside
  collaborators), and keep the default workflow permissions on read-only.
  The workflows already ask for `contents: read` and pin every action by commit.
- [ ] Branch protection for `main`, as a ruleset or classic rule: block force
  pushes and deletion of the branch, and require the CI checks to pass
  (**web**, **go**, **e2e**, **licenses**, **secrets**, **docker**, **addon**).
  If you keep pushing straight to `main`, add yourself as a bypass actor, or
  required checks will block your own pushes; require a pull request only if
  you want one for your own changes too.
- [ ] Tag protection (a ruleset on `v*` tags): only you may create or delete them.
  A tag builds a release.
- [ ] About box: a description ("Self-hosted TV dashboard that casts to a
  Chromecast or Google TV"), topics (`dashboard`, `chromecast`, `home-assistant`,
  `self-hosted`, `go`, `svelte`), and a social preview image.
- [ ] Optional: enable Discussions for questions, and add it as a contact link
  in `.github/ISSUE_TEMPLATE/config.yml`.

## 5. The first public release

- [ ] Publish the 0.6.0 release from its draft: read the notes (they come from
  the changelog and the pull requests), the checksums and the assets, then
  Publish. Mark it "latest".
- [ ] Delete the old 0.5.0 draft release, the one still named Daycast. The tag
  `v0.5.0` can stay or go; `latest` on ghcr is moved by 0.6.0.
- [ ] Verify what `docs/install.md` promises:

  ```bash
  gh attestation verify gdash_0.6.0_linux_arm64.tar.gz --repo stijnvandepol/gdash
  gh attestation verify oci://ghcr.io/stijnvandepol/gdash:0.6.0 --repo stijnvandepol/gdash
  ```

- [ ] Update the "Status: beta (0.5)" line in the README to the version that is
  released.
- [ ] Put the Docker Compose file of `docs/install.md` on a clean machine and
  check that it starts from the public image.

## 6. The Home Assistant app repository

Home Assistant needs `repository.yaml` at the **root of a git repository**,
and a user adds that repository's URL. The files in `homeassistant/` have it
one level down, so the main repository cannot be added as it is. Recommended:
a separate small public repository, `stijnvandepol/gdash-homeassistant`, whose
root is the content of `homeassistant/`. Reasons: it is what Home Assistant
supports and documents; the Supervisor clones the repository and re-reads it
regularly, so a repository with only the app is light and its store listing
shows nothing else; and the app can be released on its own schedule.
`homeassistant/README.md` and `docs/install.md` already point at that name;
change them if you choose another one.

Until the main repository is public and the image is public, the app cannot
be installed by anyone but you: the app builds on the user's machine from
`ghcr.io/stijnvandepol/gdash:<version>`. The separate repository can be
created in advance, but give it its README "experimental" status and do not
announce it before step 3 is done.

- [ ] Create the repository on GitHub (public, empty, no README).
- [ ] Publish the folder as its root, from a clone of the main repository:

  ```bash
  git subtree split --prefix=homeassistant -b ha-app
  git push https://github.com/stijnvandepol/gdash-homeassistant.git ha-app:main
  ```

  Copy `LICENSE` (AGPL-3.0) to the new repository as well, as the app contains
  GDash's license obligations; that file is not part of the subtree.
- [ ] Repeat the `git subtree split` and push at every release, after the GDash
  image of that version is public and `version` in `config.yaml` and
  `GDASH_VERSION` in the Dockerfile are changed to it (CI checks they agree).
  A workflow in the main repository could do this with a deploy key; that
  needs a secret, so it is left out.
- [ ] Add the repository in a real Home Assistant (Settings, Apps, store,
  Repositories), install, start and read `homeassistant/gdash/DOCS.md` against
  what you see. The app has only been tested here with a fake Supervisor.
  Check in particular: the app builds, GDash starts as user `gdash` with
  `/data` writable, the log shows the setup token, the "Open Web UI" button
  works, Cast devices are found, and a backup and restore of the app keep
  the data.
- [ ] When it works on a real installation, change `stage: experimental` in
  `homeassistant/gdash/config.yaml` to `stable` and remove the word
  "experimental" from the README and the docs.
- [ ] Optional later: a pre-built image for the app (`image:` in `config.yaml`)
  makes installs faster, but needs a release workflow of its own.

An alternative that avoids a second repository: a `repository.yaml` at the root
of this repository, so the main repository is the app repository. Home Assistant
reads the whole clone, and it has not been tested here whether the Supervisor
finds an app in a subfolder two levels down; try that first on a spare
installation if you prefer one repository.
