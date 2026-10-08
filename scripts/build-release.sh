#!/usr/bin/env bash
# Builds the release archives for one version into dist-release/: daycast
# for Linux (amd64, arm64, and armv7 for 32-bit Raspberry Pi OS) and for
# Windows (amd64), each with the license files and the README, plus
# SHA256SUMS. Expects web/dist to be built first.
#
# Usage: scripts/build-release.sh v1.2.3
set -euo pipefail
export LC_ALL=C
cd "$(dirname "$0")/.."

version=${1:-}
if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
  echo "usage: $0 vMAJOR.MINOR.PATCH[-PRERELEASE]" >&2
  exit 2
fi
if [ ! -f web/dist/admin/index.html ]; then
  echo "build the web interface first: (cd web && npm ci && npm run build)" >&2
  exit 1
fi

out=dist-release
rm -rf "$out"
mkdir -p "$out"

# os/arch[/arm version]
targets=(linux/amd64 linux/arm64 linux/arm/7 windows/amd64)
for target in "${targets[@]}"; do
  IFS=/ read -r goos goarch goarm <<< "$target"
  name="daycast_${version#v}_${goos}_${goarch}${goarm:+v$goarm}"
  dir="$out/$name"
  mkdir -p "$dir"
  binary=daycast
  [ "$goos" = windows ] && binary=daycast.exe
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch GOARM=$goarm \
    go build -trimpath -ldflags="-s -w -X main.version=$version" -o "$dir/$binary" ./cmd/daycast
  cp LICENSE THIRD_PARTY_LICENSES README.md "$dir/"
  if [ "$goos" = windows ]; then
    (cd "$out" && zip -qr "$name.zip" "$name")
  else
    cp deploy/daycast.service "$dir/"
    tar -czf "$out/$name.tar.gz" -C "$out" "$name"
  fi
  rm -rf "$dir"
done

(cd "$out" && sha256sum -- *.tar.gz *.zip > SHA256SUMS)
ls -l "$out"
