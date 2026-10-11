#!/usr/bin/env bash
# Daily SQLite backup, keeps the newest 7. Run from cron as the quiet user:
#   15 4 * * * /opt/quiet/deploy/backup.sh
set -euo pipefail

DB_PATH="${DB_PATH:-/var/lib/quiet/quiet.db}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/quiet/backups}"
KEEP=7

mkdir -p "$BACKUP_DIR"
target="$BACKUP_DIR/quiet-$(date +%F).db"
sqlite3 "$DB_PATH" ".backup '$target'"
ls -1t "$BACKUP_DIR"/quiet-*.db | tail -n +$((KEEP + 1)) | xargs -r rm --
echo "backup written: $target"
