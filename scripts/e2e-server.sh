#!/usr/bin/env bash
# Builds the real binary (with the embedded web app) and runs it on a fresh,
# disposable data directory for end-to-end tests. Port 6030 must be free.
#
# The data directory is deleted first, so it is only ever one this script made:
# a folder that is new, empty, or carries the marker file written below.
# Point E2E_DATA_DIR somewhere else only for a throwaway folder.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
data="${E2E_DATA_DIR:-$root/.e2e-data}"
marker=".keeps-e2e-disposable"

case "$(realpath -m "$data")" in
  / | "$HOME" | "$root")
    echo "e2e-server: refusing to use $data as disposable test data" >&2
    exit 1
    ;;
esac
if [ -d "$data" ] && [ -n "$(ls -A "$data")" ] && [ ! -f "$data/$marker" ]; then
  echo "e2e-server: $data has files and no $marker marker; refusing to delete it" >&2
  exit 1
fi
rm -rf "$data"
mkdir -p "$data"
touch "$data/$marker"
npm --prefix "$root/web" run build >/dev/null
CGO_ENABLED=0 go build -C "$root" -o "$data/keeps" ./cmd/keeps
export APP_ORIGIN=http://localhost:6030 KEEPS_DEV_LOOPBACK=1 KEEPS_DATA_DIR="$data"
exec "$data/keeps" serve
