#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/usr/local/x-ui"
DATABASE_DIR="/etc/x-ui"
DATABASE_PATH="$DATABASE_DIR/x-ui.db"
SERVICE_FILE="/etc/systemd/system/x-ui.service"

die() {
  echo "Error: $*" >&2
  exit 1
}

[[ "${EUID:-$(id -u)}" -eq 0 ]] || die "This uninstaller must be run as root."
[[ "$INSTALL_DIR" == "/usr/local/x-ui" ]] || die "Refusing to remove an unexpected installation path."

if command -v systemctl >/dev/null 2>&1; then
  systemctl stop x-ui.service 2>/dev/null || true
  systemctl disable x-ui.service 2>/dev/null || true
fi

if [[ -f "$SERVICE_FILE" ]]; then
  rm -f -- "$SERVICE_FILE"
fi
systemctl daemon-reload 2>/dev/null || true

if [[ -d "$INSTALL_DIR" ]]; then
  rm -rf -- "$INSTALL_DIR"
fi

echo "x-ui service and application files were removed."
if [[ -f "$DATABASE_PATH" ]]; then
  echo "Database preserved at $DATABASE_PATH"
  if [[ -r /dev/tty ]]; then
    printf 'Delete the database and its backup files too? [y/N] ' > /dev/tty
    read -r answer < /dev/tty || answer=""
    case "$answer" in
      y|Y|yes|YES)
        rm -f -- "$DATABASE_PATH" "$DATABASE_PATH".backup.*
        rmdir "$DATABASE_DIR" 2>/dev/null || true
        echo "Database files deleted with explicit confirmation."
        ;;
      *) echo "Database kept." ;;
    esac
  else
    echo "Non-interactive session detected; database kept."
  fi
fi
