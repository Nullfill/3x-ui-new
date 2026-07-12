#!/usr/bin/env bash
set -euo pipefail
XUI_VERSION="${VERSION:-v3.4.2-multiplier-1}"
REPOSITORY="${XUI_REPOSITORY:-Nullfill/3x-ui-new}"
INSTALL_DIR="/usr/local/x-ui"
DATABASE_PATH="/etc/x-ui/x-ui.db"
BINARY_PATH="$INSTALL_DIR/x-ui"
die() {
  echo "Error: $*" >&2
  exit 1
}
[[ "${EUID:-$(id -u)}" -eq 0 ]] || die "This updater must be run as root."
[[ -d "$INSTALL_DIR" && -f "$BINARY_PATH" ]] || die "Existing x-ui installation not found at $BINARY_PATH."
[[ -r /etc/os-release ]] || die "Cannot detect the operating system."
# shellcheck disable=SC1091
source /etc/os-release
os_version_major="${VERSION_ID%%.*}"
case "${ID:-}" in
  ubuntu) [[ "$os_version_major" =~ ^[0-9]+$ && "$os_version_major" -ge 20 ]] || die "Ubuntu 20.04 or newer is required." ;;
  debian) [[ "$os_version_major" =~ ^[0-9]+$ && "$os_version_major" -ge 11 ]] || die "Debian 11 or newer is required." ;;
  *) die "Unsupported operating system: ${PRETTY_NAME:-${ID:-unknown}}." ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "Unsupported architecture: $(uname -m). Supported architectures: amd64, arm64." ;;
esac
for command in curl tar sha256sum install systemctl mktemp; do
  command -v "$command" >/dev/null 2>&1 || die "Required command is not installed: $command"
done
ARCHIVE="x-ui-linux-${ARCH}.tar.gz"
CHECKSUM="x-ui-linux-${ARCH}.sha256"
DOWNLOAD_BASE="https://github.com/${REPOSITORY}/releases/download/${XUI_VERSION}"
tmp_dir="$(mktemp -d)"
service_stopped=false
cleanup() {
  status=$?
  trap - EXIT
  rm -rf "$tmp_dir"
  if [[ $status -ne 0 && "$service_stopped" == true ]]; then
    systemctl start x-ui.service 2>/dev/null || true
  fi
  exit "$status"
}
trap cleanup EXIT
curl --fail --location --retry 3 --proto '=https' --tlsv1.2 \
  --output "$tmp_dir/$ARCHIVE" "$DOWNLOAD_BASE/$ARCHIVE"
curl --fail --location --retry 3 --proto '=https' --tlsv1.2 \
  --output "$tmp_dir/$CHECKSUM" "$DOWNLOAD_BASE/$CHECKSUM"
(
  cd "$tmp_dir"
  sha256sum --check "$CHECKSUM"
) || die "SHA256 verification failed. Update stopped."
tar -xzf "$tmp_dir/$ARCHIVE" -C "$tmp_dir"
NEW_BINARY="$tmp_dir/x-ui-linux-${ARCH}/x-ui"
[[ -f "$NEW_BINARY" ]] || die "Release archive does not contain the expected x-ui binary."
systemctl cat x-ui.service >/dev/null 2>&1 || die "x-ui.service was not found."
systemctl stop x-ui.service
service_stopped=true
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
cp --preserve=mode,timestamps "$BINARY_PATH" "$BINARY_PATH.backup.$timestamp"
echo "Binary backup: $BINARY_PATH.backup.$timestamp"
if [[ -f "$DATABASE_PATH" ]]; then
  cp --preserve=mode,timestamps "$DATABASE_PATH" "$DATABASE_PATH.backup.$timestamp"
  echo "Database backup: $DATABASE_PATH.backup.$timestamp"
fi
install -m 0755 "$NEW_BINARY" "$INSTALL_DIR/x-ui.new"
mv -f "$INSTALL_DIR/x-ui.new" "$BINARY_PATH"
chmod +x "$BINARY_PATH"
systemctl restart x-ui.service
service_stopped=false
systemctl --no-pager --full status x-ui.service
echo "3X-UI Multiplier updated successfully to $XUI_VERSION."
