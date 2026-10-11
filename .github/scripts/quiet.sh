#!/usr/bin/env bash
# Steps for the "Daily news" workflow. The database lives on the `data` branch
# (one commit, force-pushed each run), so nothing accumulates in history.
#   quiet.sh restore            fetch quiet.db from the data branch into $STATE_DIR
#   quiet.sh run <cmd> [text]   update | add <text> | drop <id> | topics | render
#   quiet.sh save               push $STATE_DIR/quiet.db to the data branch
#   quiet.sh publish            deploy $SITE_DIR to Cloudflare Pages (skipped without a token)
set -euo pipefail

STATE_DIR="${STATE_DIR:-state}"
SITE_DIR="${SITE_DIR:-site}"
QUIET="${QUIET:-./quiet}"
export DB_PATH="$STATE_DIR/quiet.db"

case "${1:-}" in
  restore)
    mkdir -p "$STATE_DIR"
    rc=0
    git ls-remote --exit-code --heads origin data >/dev/null || rc=$?
    if [ "$rc" = 0 ]; then
      git fetch -q --depth=1 origin data
      git show FETCH_HEAD:quiet.db > "$DB_PATH"
      echo "restored database ($(wc -c < "$DB_PATH") bytes)"
    elif [ "$rc" = 2 ]; then
      echo "no data branch yet; starting with an empty database"
    else
      echo "couldn't reach the data branch; stopping so the database isn't overwritten" >&2
      exit 1
    fi
    ;;
  run)
    cmd="${2:-update}" text="${3:-}"
    case "$cmd" in
      update) "$QUIET" daily ;;
      add)    [ -n "$text" ] || { echo "add needs the topic text" >&2; exit 1; }; "$QUIET" add "$text" ;;
      drop)   [ -n "$text" ] || { echo "drop needs a topic id (run 'topics' to see them)" >&2; exit 1; }; "$QUIET" drop "$text" ;;
      topics) "$QUIET" topics ;;
      render) ;;
      *) echo "unknown command: $cmd" >&2; exit 1 ;;
    esac
    "$QUIET" topics
    "$QUIET" render -o "$SITE_DIR"
    ;;
  save)
    [ -f "$DB_PATH" ] || { echo "no database to save"; exit 0; }
    if [ -e "$DB_PATH-wal" ] && [ -s "$DB_PATH-wal" ]; then
      echo "database has an unflushed write-ahead log; not saving a partial copy" >&2
      exit 1
    fi
    blob="$(git hash-object -w "$DB_PATH")"
    tree="$(printf '100644 blob %s\tquiet.db\n' "$blob" | git mktree)"
    commit="$(git -c user.name="quiet-bot" -c user.email="quiet-bot@users.noreply.github.com" \
      commit-tree "$tree" -m "database $(date -u +%FT%TZ)")"
    git push -q -f origin "$commit:refs/heads/data"
    echo "saved database to the data branch"
    ;;
  publish)
    if [ -z "${CLOUDFLARE_API_TOKEN:-}" ] || [ -z "${CLOUDFLARE_ACCOUNT_ID:-}" ]; then
      echo "not publishing: add CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID secrets"
      exit 0
    fi
    project="${CF_PAGES_PROJECT:-nam-news}"
    npx --yes wrangler@4 pages project create "$project" --production-branch main >/dev/null 2>&1 || true
    npx --yes wrangler@4 pages deploy "$SITE_DIR" --project-name "$project" --branch main --commit-dirty=true
    ;;
  *)
    echo "usage: quiet.sh restore | run <cmd> [text] | save | publish" >&2
    exit 2
    ;;
esac
