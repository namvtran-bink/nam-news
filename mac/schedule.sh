#!/usr/bin/env bash
# Run mac/update.sh every day at 05:30 (Mac local time) using launchd.
# If the Mac is asleep at 05:30, it runs when the Mac wakes.
#   mac/schedule.sh on    install the schedule
#   mac/schedule.sh off   remove it
set -euo pipefail

LABEL="com.namnews.quiet"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$HOME/.quiet" "$HOME/Library/LaunchAgents"

case "${1:-}" in
  on)
    cat > "$PLIST" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>$LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/zsh</string><string>-lc</string><string>"$ROOT/mac/update.sh"</string>
  </array>
  <key>StartCalendarInterval</key>
  <dict><key>Hour</key><integer>5</integer><key>Minute</key><integer>30</integer></dict>
  <key>StandardOutPath</key><string>$HOME/.quiet/update.log</string>
  <key>StandardErrorPath</key><string>$HOME/.quiet/update.log</string>
</dict>
</plist>
PLIST
    launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
    launchctl bootstrap "gui/$(id -u)" "$PLIST"
    echo "scheduled daily at 05:30; log: ~/.quiet/update.log"
    ;;
  off)
    launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
    rm -f "$PLIST"
    echo "schedule removed"
    ;;
  *)
    echo "usage: mac/schedule.sh on|off" >&2
    exit 2
    ;;
esac
