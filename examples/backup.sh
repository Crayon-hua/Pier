#!/usr/bin/env sh
set -eu
# Place this file in the registered project's root, then chmod +x backup.sh.
mkdir -p "$UPY_BACKUP_DIR"
if [ ! -d "$UPY_PROJECT_ROOT/shared" ]; then
  echo "shared/ does not exist; nothing to back up" >&2
  exit 1
fi
tar -czf "$UPY_BACKUP_DIR/${UPY_PROJECT_NAME}-${UPY_TIMESTAMP}.tar.gz" \
  -C "$UPY_PROJECT_ROOT" shared
