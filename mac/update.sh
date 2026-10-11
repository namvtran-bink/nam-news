#!/usr/bin/env bash
# Fetch today's news for every followed topic, build the page, and publish it.
#   mac/update.sh            fetch + build + publish
#   mac/update.sh --no-fetch rebuild and publish from what's already saved
set -euo pipefail

MAC="$(cd "$(dirname "$0")" && pwd)"
SITE="$HOME/.quiet/site"

if [ "${1:-}" != "--no-fetch" ]; then
  "$MAC/quiet" daily
fi
"$MAC/quiet" render -o "$SITE"

set -a; . "$MAC/.env"; set +a
if [ -n "${CF_PAGES_PROJECT:-}" ]; then
  npx --yes wrangler@4 pages deploy "$SITE" --project-name "$CF_PAGES_PROJECT" --branch main --commit-dirty=true
else
  echo "not publishing (CF_PAGES_PROJECT is empty); open $SITE/index.html"
fi
