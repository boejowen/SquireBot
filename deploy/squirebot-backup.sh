#!/usr/bin/env bash
# SquireBot nightly off-box backup (BACKEND-06, D-11).
#
# Takes a CONSISTENT hot snapshot of the SQLite DB via the sqlite3 online-backup
# API (NOT a raw cp of the live WAL-mode .db — that misses recent commits and can
# corrupt; see 11-RESEARCH Pitfall 6), gzips it, and uploads it off-box to a
# private Cloudflare R2 bucket via rclone. The R2 remote ("r2") is configured in
# rclone.conf (mode 600, owned by the cron user) with a bucket-scoped Object
# Read & Write token. The .backup is done in shell, never from Go — modernc.org/
# sqlite has no C online-backup API.
#
# Install: /usr/local/bin/squirebot-backup.sh (chmod +x)
# Cron:    0 4 * * * /usr/local/bin/squirebot-backup.sh >> /var/log/squirebot-backup.log 2>&1
set -euo pipefail
DB=/var/lib/squirebot/squirebot.db
STAMP=$(date -u +%F)
SNAP=/tmp/squirebot-$STAMP.db
sqlite3 "$DB" ".backup '$SNAP'"               # consistent hot snapshot (online backup API)
gzip -f "$SNAP"
rclone copy "$SNAP.gz" r2:squirebot-backups/  # r2 remote in rclone.conf (mode 600, bucket-scoped token)
rm -f "$SNAP.gz"
