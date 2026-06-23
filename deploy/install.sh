#!/usr/bin/env bash
set -euo pipefail

REPO="${LIGHTSCALE_REPO:-devcutler/lightscale}"
VERSION="${LIGHTSCALE_VERSION:-latest}"
BIN_DIR=/usr/local/bin
SERVICE_USER=lightscaled
SERVICE_GROUP=lightscale

command -v sudo >/dev/null || { echo "need sudo, it's not installed" >&2; exit 1; }

if command -v curl >/dev/null; then
	fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null; then
	fetch() { wget -qO "$2" "$1"; }
else
	echo "need curl or wget, neither is installed" >&2; exit 1
fi

case "$(uname -m)" in
	x86_64|amd64) ARCH=amd64 ;;
	aarch64|arm64) ARCH=arm64 ;;
	*) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

if [[ "$VERSION" == "latest" ]]; then
	BASE="https://github.com/$REPO/releases/latest/download"
else
	BASE="https://github.com/$REPO/releases/download/$VERSION"
fi

SRC_DIR="$(mktemp -d)"
trap 'rm -rf "$SRC_DIR"' EXIT

echo "downloading $VERSION ($ARCH)..."
fetch "$BASE/lightscaled-linux-$ARCH" "$SRC_DIR/lightscaled"
fetch "$BASE/lightscale-linux-$ARCH" "$SRC_DIR/lightscale"
fetch "$BASE/lightscaled.service" "$SRC_DIR/lightscaled.service"

echo "installing..."
sudo install -m 0755 "$SRC_DIR/lightscaled" "$BIN_DIR/lightscaled"
sudo install -m 0755 "$SRC_DIR/lightscale" "$BIN_DIR/lightscale"

getent group "$SERVICE_GROUP" >/dev/null || sudo groupadd --system "$SERVICE_GROUP"
getent passwd "$SERVICE_USER" >/dev/null || \
	sudo useradd --system --no-create-home --shell /usr/sbin/nologin \
		--gid "$SERVICE_GROUP" "$SERVICE_USER"

if getent group docker >/dev/null; then sudo usermod -aG docker "$SERVICE_USER"; fi

sudo install -d -m 0755 /etc/lightscale
if [[ ! -f /etc/lightscale/lightscale.toml ]]; then
	sudo "$BIN_DIR/lightscaled" init
fi
sudo chgrp "$SERVICE_GROUP" /etc/lightscale/lightscale.toml
sudo chmod 0640 /etc/lightscale/lightscale.toml

sudo install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_GROUP" /var/lib/lightscale
sudo chown -R "$SERVICE_USER:$SERVICE_GROUP" /var/lib/lightscale

sudo install -m 0644 "$SRC_DIR/lightscaled.service" /etc/systemd/system/lightscaled.service
sudo systemctl daemon-reload
sudo systemctl enable lightscaled
sudo systemctl try-restart lightscaled

INSTALL_USER="${SUDO_USER:-$USER}"
if [[ -n "$INSTALL_USER" && "$INSTALL_USER" != "root" ]]; then
	sudo usermod -aG "$SERVICE_GROUP" "$INSTALL_USER"
fi

echo
echo "done. next:"
echo "  1. set public_endpoint:  sudo nano /etc/lightscale/lightscale.toml"
echo "  2. start it:             sudo systemctl start lightscaled"
echo
echo "start a new shell to run without sudo (or run newgrp $SERVICE_GROUP)"

if [[ $- == *i* && -t 0 ]]; then
	exec newgrp "$SERVICE_GROUP"
fi
