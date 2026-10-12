# GDash for Home Assistant

An app (formerly called add-on) that runs [GDash](https://github.com/stijnvandepol/gdash),
a self-hosted dashboard for the TVs in your house, on Home Assistant OS or
Home Assistant Supervised.

## Add the repository

[![Add this repository to your Home Assistant.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fstijnvandepol%2Fgdash-homeassistant)

Or by hand: in Home Assistant, go to **Settings, Apps** (older versions:
**Add-ons**), open the store, choose the menu at the top right, **Repositories**,
and add:

```text
https://github.com/stijnvandepol/gdash-homeassistant
```

Then open **GDash**, install it and start it. How to use it is in
[gdash/DOCS.md](gdash/DOCS.md).

## For maintainers

This folder is laid out as an app repository: `repository.yaml` at the root
and one app in `gdash/`. Home Assistant needs `repository.yaml` at the root of
a git repository, so this folder is published as a repository of its own,
`gdash-homeassistant`, with the content of this folder as its root. How and
why: [docs/publishing.md](../docs/publishing.md).

To release a new version, change `version` in `gdash/config.yaml` and `ARG
GDASH_VERSION` in `gdash/Dockerfile` to the new GDash version (CI checks that
they agree), add a line to `gdash/CHANGELOG.md`, and publish the folder again.
The GDash release must exist first: the app copies the program from
`ghcr.io/stijnvandepol/gdash:<version>`.
