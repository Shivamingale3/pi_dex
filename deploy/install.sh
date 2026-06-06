#!/usr/bin/env bash
set -euo pipefail

REPO="Shivamingale3/pi_dex"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/pidex"
SERVICE_DIR="/etc/systemd/system"
USER="pidex"
GROUP="pidex"

echo "=== PiDex Installer ==="

ARCH="unknown"
case "$(uname -m)" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
esac

# Try downloading pre-built binary
installed=false

echo "Finding latest release..."
TAG=$(curl -sfL "https://api.github.com/repos/$REPO/releases/latest" \
    | tr ',' '\n' | grep '"tag_name"' | cut -d'"' -f4)

if [ -n "$TAG" ] && [ "$ARCH" != "unknown" ]; then
    echo "Downloading PiDex $TAG (linux/$ARCH)..."

    download_ok=true
    for bin in pidex pidex-shutdown; do
        url="https://github.com/$REPO/releases/download/$TAG/$bin-$TAG-linux-$ARCH"
        if ! curl -sfLo "$INSTALL_DIR/$bin" "$url"; then
            download_ok=false
            break
        fi
        chmod 755 "$INSTALL_DIR/$bin"
    done

    if [ "$download_ok" = true ]; then
        installed=true
        echo "Installed: $INSTALL_DIR/pidex, $INSTALL_DIR/pidex-shutdown"
    fi
fi

# Fall back to building from source
if [ "$installed" = false ]; then
    echo "Pre-built binary not available for this system."

    if ! command -v go &>/dev/null; then
        echo "ERROR: Go is required to build PiDex from source."
        echo "Install Go: https://go.dev/doc/install"
        exit 1
    fi

    TMP_DIR="$(mktemp -d)"
    trap 'rm -rf "$TMP_DIR"' EXIT
    echo "Building from source..."
    git clone --depth 1 "https://github.com/$REPO.git" "$TMP_DIR"
    cd "$TMP_DIR"

    go build -ldflags="-s -w" -o "$INSTALL_DIR/pidex" ./cmd/pidex
    go build -ldflags="-s -w" -o "$INSTALL_DIR/pidex-shutdown" ./cmd/pidex-shutdown
    echo "Installed: $INSTALL_DIR/pidex, $INSTALL_DIR/pidex-shutdown"
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
echo "=== PiDex installed ==="
echo ""
echo "Next steps:"
echo "  1. Set Telegram credentials:"
echo "     echo 'TELEGRAM_BOT_TOKEN=your_token' >> $CONFIG_DIR/env"
echo "     echo 'TELEGRAM_CHAT_ID=your_chat_id' >> $CONFIG_DIR/env"
echo "  2. Enable daemon:  sudo systemctl enable --now pidex"
echo "  3. (Optional) Enable shutdown notifications:"
echo "     sudo systemctl enable pidex-shutdown"
echo ""
echo "Add your user to the pidex group for config write access:"
echo "  sudo usermod -aG $GROUP \$USER"
