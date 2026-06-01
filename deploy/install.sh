#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/opt/pidex"
CONFIG_DIR="/etc/pidex"
SERVICE_DIR="/etc/systemd/system"
USER="pidex"
GROUP="pidex"

echo "=== PiDex Installer ==="

# Create user if missing
if ! id -u "$USER" &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin "$USER"
    echo "Created system user: $USER"
fi

# Install Python package
if command -v pip &>/dev/null; then
    pip install --quiet --upgrade "$(dirname "$0")/.."
    echo "Installed pidex Python package"
else
    echo "ERROR: pip not found. Install Python 3 + pip first." >&2
    exit 1
fi

# Create config directory
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/config.toml" ]; then
    cp "$(dirname "$0")/../config/config.toml" "$CONFIG_DIR/config.toml"
    echo "Created default config at $CONFIG_DIR/config.toml"
    echo "  >>> EDIT $CONFIG_DIR/config.toml WITH YOUR TELEGRAM CREDENTIALS <<<"
fi

# Install systemd services
cp "$(dirname "$0")/pidex.service" "$SERVICE_DIR/pidex.service"
cp "$(dirname "$0")/pidex-shutdown.service" "$SERVICE_DIR/pidex-shutdown.service"
systemctl daemon-reload

echo ""
echo "=== Installation Complete ==="
echo ""
echo "Next steps:"
echo "  1. Edit $CONFIG_DIR/config.toml with your Telegram bot token and chat ID"
echo "  2. Enable the service:  sudo systemctl enable pidex"
echo "  3. Start the service:   sudo systemctl start pidex"
echo "  4. Check status:        sudo systemctl status pidex"
echo ""
echo "To enable shutdown notifications:"
echo "  sudo systemctl enable pidex-shutdown"
