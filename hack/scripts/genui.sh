#!/usr/bin/env bash
set -euo pipefail

version=${1:-dev}
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root/web"

yarn install --frozen-lockfile --non-interactive
DISABLE_ESLINT_PLUGIN=true VITE_APP_VERSION="$version" yarn build
