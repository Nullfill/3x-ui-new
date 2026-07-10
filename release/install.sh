#!/usr/bin/env bash
set -euo pipefail

REPOSITORY="${XUI_RELEASE_REPOSITORY:-Nullfill/3x-ui-new}"
RELEASE_TAG="${1:-${XUI_RELEASE_TAG:-v3.4.2-multiplier-1}}"
INSTALL_DIR="${XUI_INSTALL_DIR:-/usr/local/x-ui}"
BINARY_PATH="$INSTALL_DIR/x-ui"
SERVICE_NAME="${XUI_SERVICE_NAME:-x-ui}"
ARCHIVE_NAME="x-ui-linux-amd64.tar.gz"
CHECKSUM_NAME="x-ui-linux-amd64.sha256"

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "Run this installer as root (for example: sudo $0 $RELEASE_TAG)." >&2
  exit 1
fi

for command in curl sha256sum tar install; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "Required command not found: $command" >&2
    exit 1
  }
done

if [[ ! -d "$INSTALL_DIR" && ! -f "$BINARY_PATH" ]]; then
  echo "No existing x-ui installation was found at $INSTALL_DIR." >&2
  exit 1
fi

service_exists=false
if command -v systemctl >/dev/null 2>&1 && systemctl cat "$SERVICE_NAME.service" >/dev/null 2>&1; then
  service_exists=true
fi

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

base_url="https://github.com/$REPOSITORY/releases/download/$RELEASE_TAG"
echo "Downloading $REPOSITORY release $RELEASE_TAG..."
curl --fail --location --retry 3 --output "$tmp_dir/$ARCHIVE_NAME" "$base_url/$ARCHIVE_NAME"
curl --fail --location --retry 3 --output "$tmp_dir/$CHECKSUM_NAME" "$base_url/$CHECKSUM_NAME"
(
  cd "$tmp_dir"
  sha256sum --check "$CHECKSUM_NAME"
)
tar -xzf "$tmp_dir/$ARCHIVE_NAME" -C "$tmp_dir"
test -x "$tmp_dir/x-ui-linux-amd64/x-ui"

if [[ "$service_exists" == true ]]; then
  echo "Stopping $SERVICE_NAME.service..."
  systemctl stop "$SERVICE_NAME.service"
fi

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_path="$INSTALL_DIR/x-ui.backup.$timestamp"
if [[ -f "$BINARY_PATH" ]]; then
  cp --preserve=mode,timestamps "$BINARY_PATH" "$backup_path"
  ln -sfn "$(basename "$backup_path")" "$INSTALL_DIR/x-ui.backup"
  echo "Previous binary backed up to $backup_path"
fi

install -m 0755 "$tmp_dir/x-ui-linux-amd64/x-ui" "$INSTALL_DIR/x-ui.new"
mv -f "$INSTALL_DIR/x-ui.new" "$BINARY_PATH"
chmod +x "$BINARY_PATH"

if [[ "$service_exists" == true ]]; then
  systemctl restart "$SERVICE_NAME.service"
  systemctl --no-pager --full status "$SERVICE_NAME.service"
else
  echo "No $SERVICE_NAME.service unit was found; binary replaced without restarting a service."
fi

echo "Installed $RELEASE_TAG successfully."
