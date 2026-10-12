# The Home Assistant app: the GDash binary of a release on the app base image.
#
# The Supervisor builds this file on the Home Assistant machine; the build is
# one copy, so it takes seconds. Bump GDASH_VERSION and "version" in
# config.yaml together, to a release whose image is public on ghcr.io.
ARG GDASH_VERSION=0.6.0

# The multi-arch GDash image: Docker takes the one for the machine's CPU.
# It has no shell, so only the binary is used from it.
FROM ghcr.io/stijnvandepol/gdash:${GDASH_VERSION} AS gdash

# The Supervisor no longer provides BUILD_FROM, so the base is named here.
# It brings s6-overlay (the init), bashio (reads the app's options) and
# s6-setuidgid. GDash is a static binary, so Alpine's musl is no problem.
FROM ghcr.io/home-assistant/base:3.23

COPY --from=gdash /gdash /usr/bin/gdash

# The Supervisor mounts /data owned by root. The service chowns it and then
# runs GDash as this user, the same id as in the GDash image, so GDash never
# runs as root.
RUN addgroup -g 65532 gdash \
 && adduser -D -H -u 65532 -G gdash -s /sbin/nologin gdash

COPY rootfs /
RUN chmod a+x /etc/services.d/gdash/run /etc/services.d/gdash/finish

ENV GDASH_DATA_DIR=/data

# The Supervisor watches Docker's health state (a "watchdog" key in
# config.yaml is obsolete). GDash asks its own /healthz.
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD ["/usr/bin/gdash", "healthcheck"]

LABEL \
    org.opencontainers.image.title="Home Assistant App: GDash" \
    org.opencontainers.image.description="Self-hosted home dashboard for TVs" \
    org.opencontainers.image.source="https://github.com/stijnvandepol/gdash" \
    org.opencontainers.image.licenses="AGPL-3.0-only"
