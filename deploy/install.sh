#!/usr/bin/env bash
set -euo pipefail

REPO="Shivamingale3/pi_dex"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/pidex"
SERVICE_DIR="/etc/systemd/system"
USER="pidex"
GROUP="pidex"

ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ASSET_ARCH="amd64"  ;;
    aarch64) ASSET_ARCH="arm64"  ;;
    *)
        echo "Unsupported architecture: $ARCH"
        echo "PiDex ships prebuilt binaries for linux/amd64 and linux/arm64 only."
        echo "See 'Building from Source' in the README:"
        echo "  https://github.com/$REPO?tab=readme-ov-file#building-from-source"
        exit 0
        ;;
esac

echo "=== PiDex Installer ==="
echo "Architecture: $ARCH"
RELEASE=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest")
TAG=$(echo "$RELEASE" | grep '"tag_name"' | cut -d'"' -f4)
echo "Release: $TAG"

TMP="/tmp/pidex-install"
mkdir -p "$TMP"
cd "$TMP"
BINARY="pidex-$TAG-linux-$ASSET_ARCH"
curl -sSLO "https://github.com/$REPO/releases/download/$TAG/$BINARY"
curl -sSLO "https://github.com/$REPO/releases/download/$TAG/SHA256SUMS"

sha256sum -c SHA256SUMS --ignore-missing 2>/dev/null || {
    echo "ERROR: SHA256 checksum mismatch"
    rm -f "$BINARY"
    exit 1
}

install -m 0755 "$BINARY" "$INSTALL_DIR/pidex"
ln -sf pidex "$INSTALL_DIR/pidex-shutdown"
echo "Installed: $INSTALL_DIR/pidex"

if command -v apt-get &>/dev/null; then
    apt-get install -y --no-install-recommends python3-systemd 2>/dev/null || true
fi

if ! id -u "$USER" &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin "$USER"
    echo "Created system user: $USER"
fi

mkdir -p "$CONFIG_DIR"
touch "$CONFIG_DIR/env"
chmod 600 "$CONFIG_DIR/env"

cat > "$CONFIG_DIR/config.toml" << 'CONFIG'
[monitor]
ssh = true
sudo = true
docker = true
systemd = true
network = true
cpu = true
ram = true
disk = true
temperature = true

[services]
watch = ["cloudflared", "docker", "nginx", "caddy", "tailscale"]

[containers]
watch = []

[pollers]
cpu_interval = 15
ram_interval = 30
temp_interval = 30
disk_interval = 300

[thresholds]
cpu_warn = 80
cpu_critical = 95
ram_warn = 85
ram_critical = 95
disk_warn = 85
disk_critical = 95
temp_warn = 75
temp_critical = 85

[cooldowns]
# Use defaults
CONFIG
chmod 640 "$CONFIG_DIR/config.toml"

cat > "$SERVICE_DIR/pidex.service" << SERVICE
[Unit]
Description=PiDex - Home Server Watchman
Documentation=https://github.com/$REPO
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/pidex run
Restart=on-failure
RestartSec=5
User=$USER
Group=$GROUP
RuntimeDirectory=pidex
StateDirectory=pidex
ConfigurationDirectory=pidex
StandardOutput=journal
StandardError=journal
EnvironmentFile=-$CONFIG_DIR/env

[Install]
WantedBy=multi-user.target
SERVICE

cat > "$SERVICE_DIR/pidex-shutdown.service" << SERVICE
[Unit]
Description=PiDex Shutdown Notification
Documentation=https://github.com/$REPO
DefaultDependencies=no
Before=shutdown.target reboot.target halt.target

[Service]
Type=oneshot
ExecStart=$INSTALL_DIR/pidex-shutdown
RemainAfterExit=true
User=$USER
Group=$GROUP
StandardOutput=journal
StandardError=journal
EnvironmentFile=-$CONFIG_DIR/env

[Install]
WantedBy=shutdown.target reboot.target halt.target
SERVICE

systemctl daemon-reload
echo "Installed systemd services"

echo ""
echo "=== PiDex $TAG installed ==="
echo ""
echo "Next steps:"
echo "  1. Configure:  sudo pidex setup"
echo "  2. Enable daemon:  sudo systemctl enable --now pidex"
echo "  3. (Optional) Enable shutdown notifications:"
echo "     sudo systemctl enable pidex-shutdown"
echo ""
echo "Add your user to the pidex group for config write access:"
echo "  sudo usermod -aG $GROUP \$USER"
