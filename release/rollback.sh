#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${XUI_INSTALL_DIR:-/usr/local/x-ui}"
BINARY_PATH="$INSTALL_DIR/x-ui"
BACKUP_PATH="${1:-$INSTALL_DIR/x-ui.backup}"
SERVICE_NAME="${XUI_SERVICE_NAME:-x-ui}"

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "Run this rollback script as root." >&2
  exit 1
fi

if [[ ! -f "$BACKUP_PATH" ]]; then
  echo "Backup binary not found: $BACKUP_PATH" >&2
  exit 1
fi

service_exists=false
if command -v systemctl >/dev/null 2>&1 && systemctl cat "$SERVICE_NAME.service" >/dev/null 2>&1; then
  service_exists=true
  systemctl stop "$SERVICE_NAME.service"
fi

install -m 0755 "$BACKUP_PATH" "$INSTALL_DIR/x-ui.rollback"
mv -f "$INSTALL_DIR/x-ui.rollback" "$BINARY_PATH"
chmod +x "$BINARY_PATH"

if [[ "$service_exists" == true ]]; then
  systemctl restart "$SERVICE_NAME.service"
  systemctl --no-pager --full status "$SERVICE_NAME.service"
fi

echo "Restored x-ui from $BACKUP_PATH"
