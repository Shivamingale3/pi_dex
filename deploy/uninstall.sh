#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/pidex"
SERVICE_DIR="/etc/systemd/system"
USER="pidex"

echo "=== PiDex Uninstall ==="

for svc in pidex pidex-shutdown; do
    systemctl disable --now "$svc" 2>/dev/null || true
done

rm -f "$INSTALL_DIR/pidex" "$INSTALL_DIR/pidex-shutdown"
rm -f "$SERVICE_DIR/pidex.service" "$SERVICE_DIR/pidex-shutdown.service"

if [ -d "$CONFIG_DIR" ]; then
    read -r -p "Remove $CONFIG_DIR? [y/N]: " answer
    if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
        rm -rf "$CONFIG_DIR"
        echo "Removed $CONFIG_DIR"
    fi
fi

if id -u "$USER" &>/dev/null; then
    read -r -p "Remove $USER system user? [y/N]: " answer
    if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
        userdel "$USER"
        echo "Removed user: $USER"
    fi
fi

systemctl daemon-reload
echo "PiDex uninstalled."
