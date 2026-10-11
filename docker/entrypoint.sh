#!/usr/bin/env bash
# update     fetch news for followed topics, render the page, publish it
# render     render and publish without fetching
# schedule   run "update" once a day at $RUN_AT (default 05:30, in $TZ); catches up after sleep
# anything else is passed to the quiet CLI: add "<text>", topics, drop <id>, run "<text>", daily
set -euo pipefail

publish() {
  quiet render -o "$SITE_DIR"
  if [ -n "${CF_PAGES_PROJECT:-}" ]; then
    wrangler pages deploy "$SITE_DIR" --project-name "$CF_PAGES_PROJECT" --branch main --commit-dirty=true
  else
    echo "not publishing (CF_PAGES_PROJECT is empty); open data/site/index.html on your Mac"
  fi
}

case "${1:-schedule}" in
  update)
    quiet daily
    publish
    ;;
  render)
    publish
    ;;
  schedule)
    stamp="/data/.last-run"
    echo "scheduler: running daily at $RUN_AT ($TZ)"
    while true; do
      today="$(date +%F)"
      if [ "$(date +%H:%M)" \> "$RUN_AT" ] || [ "$(date +%H:%M)" = "$RUN_AT" ]; then
        if [ "$(cat "$stamp" 2>/dev/null || true)" != "$today" ]; then
          echo "scheduler: update $(date)"
          if "$0" update; then echo "$today" > "$stamp"; else echo "scheduler: update failed, retrying in 30 min"; sleep 1500; fi
        fi
      fi
      sleep 300
    done
    ;;
  *)
    exec quiet "$@"
    ;;
esac
