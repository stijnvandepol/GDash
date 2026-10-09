# Builds one small image with the Go binary and the frontend embedded in it.
# Build stages run on the build machine's platform and cross-compile, so a
# multi-arch build (linux/amd64, linux/arm64) does not need emulation.
#
# Base images are pinned by the digest of their multi-arch index, with the
# tag they were taken from in the comment above. Dependabot proposes a new
# tag and digest together; a tag alone can change under the build.

# node:24-alpine
FROM --platform=$BUILDPLATFORM node:24-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1 AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# golang:1.26-alpine (Go 1.26.9, the version go.mod asks for)
FROM --platform=$BUILDPLATFORM golang:1.26-alpine@sha256:c95332c2af86b6d89b91bd0500f4b9529ccbd090a0d1855c6d1ceaa142ae8615 AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG TARGETOS
ARG TARGETARCH
# Set by the release workflow, for example v1.0.0; empty reports "dev".
ARG VERSION=""
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/gdash ./cmd/gdash \
 && mkdir -m 0700 /out/data

# gcr.io/distroless/static-debian12:nonroot. It has no shell or package
# manager, so there is nothing to exploit besides GDash itself, and it
# carries the CA certificates for HTTPS to the weather service and the
# calendars. The time zone database is compiled into the binary.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
ARG VERSION="dev"
ARG REVISION=""
LABEL org.opencontainers.image.title="GDash" \
      org.opencontainers.image.description="Self-hosted home dashboard for TVs" \
      org.opencontainers.image.source="https://github.com/stijnvandepol/gdash" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$REVISION"
COPY --from=server /out/gdash /gdash
COPY LICENSE THIRD_PARTY_LICENSES /usr/share/doc/gdash/
# GDash runs as this fixed user and group, never as root. The data
# directory belongs to it and nobody else can read it; a new named volume
# copies both from the image. A volume or folder made by something else
# needs the chown in docs/install.md.
COPY --from=server --chown=65532:65532 --chmod=0700 /out/data /data
USER 65532:65532
ENV GDASH_DATA_DIR=/data
VOLUME /data
EXPOSE 8420
# No shell or curl in the image: GDash asks its own /healthz.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD ["/gdash", "healthcheck"]
ENTRYPOINT ["/gdash"]
