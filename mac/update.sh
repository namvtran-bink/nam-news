#!/usr/bin/env bash
# Fetch today's news for every followed topic, build the page, and publish it.
#   mac/update.sh            fetch + build + publish
#   mac/update.sh --no-fetch rebuild and publish from what's already saved
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
QUIET_HOME="$HOME/.quiet"
mkdir -p "$QUIET_HOME"

if [ ! -f "$ROOT/mac/.env" ]; then
  echo "missing $ROOT/mac/.env: copy mac/env.example to mac/.env and fill it in" >&2
  exit 1
fi
set -a; . "$ROOT/mac/.env"; set +a
export DB_PATH="$QUIET_HOME/quiet.db"

# Rebuild the binary only when the code changed.
if [ ! -x "$QUIET_HOME/quiet" ] || [ -n "$(find "$ROOT/backend" -newer "$QUIET_HOME/quiet" -name '*.go' -o -newer "$QUIET_HOME/quiet" -name '*.html' -o -newer "$QUIET_HOME/quiet" -name '*.sql' | head -1)" ]; then
  (cd "$ROOT/backend" && go build -o "$QUIET_HOME/quiet" ./cmd/quiet)
fi

if [ "${1:-}" != "--no-fetch" ]; then
  "$QUIET_HOME/quiet" daily
fi
"$QUIET_HOME/quiet" render -o "$QUIET_HOME/site"

if [ -n "${CF_PAGES_PROJECT:-}" ]; then
  npx --yes wrangler@4 pages deploy "$QUIET_HOME/site" --project-name "$CF_PAGES_PROJECT" --branch main --commit-dirty=true
else
  echo "not publishing (CF_PAGES_PROJECT is empty); open $QUIET_HOME/site/index.html"
fi
