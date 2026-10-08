# Builds one small image with the Go binary and the frontend embedded in it.
# Build stages run on the build machine's platform and cross-compile, so a
# multi-arch build (linux/amd64, linux/arm64) does not need emulation.

FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/daycast ./cmd/daycast \
 && mkdir /out/data

# distroless has no shell or package manager: nothing to exploit besides
# Daycast itself.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/daycast /daycast
COPY --from=server --chown=65532:65532 /out/data /data
ENV DAYCAST_DATA_DIR=/data
VOLUME /data
EXPOSE 8420
ENTRYPOINT ["/daycast"]
