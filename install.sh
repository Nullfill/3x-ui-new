#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-v3.4.2-multiplier-1}"
REPOSITORY="${XUI_REPOSITORY:-Nullfill/3x-ui-new}"
INSTALL_DIR="/usr/local/x-ui"
DATABASE_DIR="/etc/x-ui"
DATABASE_PATH="$DATABASE_DIR/x-ui.db"
BINARY_PATH="$INSTALL_DIR/x-ui"
SERVICE_FILE="/etc/systemd/system/x-ui.service"

die() {
  echo "Error: $*" >&2
  exit 1
}

[[ "${EUID:-$(id -u)}" -eq 0 ]] || die "This installer must be run as root. Use sudo -i first."

[[ -r /etc/os-release ]] || die "Cannot detect the operating system (/etc/os-release is missing)."
# shellcheck disable=SC1091
source /etc/os-release
os_version_major="${VERSION_ID%%.*}"
case "${ID:-}" in
  ubuntu)
    [[ "$os_version_major" =~ ^[0-9]+$ && "$os_version_major" -ge 20 ]] || \
      die "Ubuntu ${VERSION_ID:-unknown} is unsupported. Ubuntu 20.04 or newer is required."
    ;;
  debian)
    [[ "$os_version_major" =~ ^[0-9]+$ && "$os_version_major" -ge 11 ]] || \
      die "Debian ${VERSION_ID:-unknown} is unsupported. Debian 11 or newer is required."
    ;;
  *) die "Unsupported operating system: ${PRETTY_NAME:-${ID:-unknown}}. Use Ubuntu 20.04+ or Debian 11+." ;;
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
DOWNLOAD_BASE="https://github.com/${REPOSITORY}/releases/download/${VERSION}"
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

echo "Downloading 3X-UI Multiplier ${VERSION} for ${ARCH}..."
curl --fail --location --retry 3 --proto '=https' --tlsv1.2 \
  --output "$tmp_dir/$ARCHIVE" "$DOWNLOAD_BASE/$ARCHIVE"
curl --fail --location --retry 3 --proto '=https' --tlsv1.2 \
  --output "$tmp_dir/$CHECKSUM" "$DOWNLOAD_BASE/$CHECKSUM"

(
  cd "$tmp_dir"
  sha256sum --check "$CHECKSUM"
) || die "SHA256 verification failed. Installation stopped."

tar -xzf "$tmp_dir/$ARCHIVE" -C "$tmp_dir"
NEW_BINARY="$tmp_dir/x-ui-linux-${ARCH}/x-ui"
[[ -f "$NEW_BINARY" ]] || die "The release archive does not contain x-ui-linux-${ARCH}/x-ui."

install -d -m 0755 "$INSTALL_DIR" "$DATABASE_DIR"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"

if systemctl cat x-ui.service >/dev/null 2>&1; then
  systemctl stop x-ui.service
  service_stopped=true
fi

if [[ -f "$BINARY_PATH" ]]; then
  cp --preserve=mode,timestamps "$BINARY_PATH" "$BINARY_PATH.backup.$timestamp"
  echo "Binary backup: $BINARY_PATH.backup.$timestamp"
fi

if [[ -f "$DATABASE_PATH" ]]; then
  cp --preserve=mode,timestamps "$DATABASE_PATH" "$DATABASE_PATH.backup.$timestamp"
  echo "Database backup: $DATABASE_PATH.backup.$timestamp"
fi

install -m 0755 "$NEW_BINARY" "$INSTALL_DIR/x-ui.new"
mv -f "$INSTALL_DIR/x-ui.new" "$BINARY_PATH"
chmod +x "$BINARY_PATH"

if [[ ! -f "$SERVICE_FILE" ]]; then
  cat > "$SERVICE_FILE" <<'SERVICE'
[Unit]
Description=3X-UI Multiplier Service
After=network.target
Wants=network.target

[Service]
Type=simple
WorkingDirectory=/usr/local/x-ui
ExecStart=/usr/local/x-ui/x-ui
ExecReload=/bin/kill -USR1 $MAINPID
Restart=on-failure
RestartSec=5s
Environment="XRAY_VMESS_AEAD_FORCED=false"

[Install]
WantedBy=multi-user.target
SERVICE
fi

systemctl daemon-reload
systemctl enable x-ui.service
# The application performs its idempotent database migration during startup.
# Any existing database has already been backed up above.
systemctl restart x-ui.service
service_stopped=false

cat <<EOF
=================================
3X-UI Multiplier Installation Done
=================================

Version:
$VERSION

Binary:
$BINARY_PATH

Database:
$DATABASE_PATH

Service:
systemctl status x-ui

=================================
EOF
