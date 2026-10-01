#!/usr/bin/env bash
set -euo pipefail

# Install only into the caller's disposable tool directory, never the Docker plugin directory.
destination=${1:?Pass a temporary output directory}
mkdir -p "$destination"
case "$(uname -s)-$(uname -m)" in
  Linux-x86_64)
    asset=linux-x86_64
    checksum=db1889184726840f75c4f9c001048430d4f25b3be3cb084d3ddd762bc0aed576
    ;;
  Linux-aarch64)
    asset=linux-aarch64
    checksum=732e3a84c1a0f67256ce80bc2598a24546b10ca05f9faa97efceb1171ece2ef7
    ;;
  Darwin-arm64)
    asset=darwin-aarch64
    checksum=998735c9b6fe68a4f05895e6ea73d71ad06f9fc7046383ad89e47346781b6af5
    ;;
  *) echo 'Unsupported Compose test runner platform' >&2; exit 1 ;;
esac
curl --fail --silent --show-error --location \
  "https://github.com/docker/compose/releases/download/v5.5.1/docker-compose-$asset" \
  -o "$destination/docker-compose"
printf '%s  %s\n' "$checksum" "$destination/docker-compose" | shasum -a 256 -c -
chmod +x "$destination/docker-compose"
"$destination/docker-compose" version
