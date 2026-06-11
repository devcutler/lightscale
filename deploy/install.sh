#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${LIGHTSCALE_REPO:-https://github.com/devcutler/lightscale}"
BRANCH="${LIGHTSCALE_BRANCH:-main}"
BIN_DIR=/usr/local/bin
SERVICE_USER=lightscaled
SERVICE_GROUP=lightscale

for cmd in git go sudo; do
	command -v "$cmd" >/dev/null || { echo "need $cmd, it's not installed" >&2; exit 1; }
done

SRC_DIR="$(mktemp -d)"
trap 'rm -rf "$SRC_DIR"' EXIT

echo "cloning..."
git clone --depth 1 --branch "$BRANCH" "$REPO_URL" "$SRC_DIR" --quiet
cd "$SRC_DIR"

echo "building..."
go build -o "$SRC_DIR/lightscaled" ./daemon
go build -o "$SRC_DIR/lightscale" ./cli

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

sudo install -m 0644 "$SRC_DIR/deploy/lightscaled.service" /etc/systemd/system/lightscaled.service
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
