# PiDex

Event-driven home server watchman. Listens for SSH logins, Docker events, systemd failures, resource thresholds, netlink interface changes, and sudo commands — forwards them as Telegram notifications.

## Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/Shivamingale3/pi_dex/main/deploy/install.sh | sudo bash
sudo pidex setup
sudo systemctl enable --now pidex
```

**Prerequisites**: Debian 12 / Ubuntu 24.04+ / Raspberry Pi OS (64-bit), x86_64 or aarch64.

## Commands

| Command | Description |
|---------|-------------|
| `pidex run` | Start the daemon |
| `pidex setup` | Interactive configuration wizard |
| `pidex test <event>` | Send a test notification (ssh-login, ssh-fail, sudo-used, docker-down, reboot) |
| `pidex version` | Show version |

## Configuration

Run `sudo pidex setup` to configure interactively. Credentials are stored in `/etc/pidex/env` (mode 600), all other settings in `/etc/pidex/config.toml` (mode 640).

To configure manually:

```bash
# Credentials
sudo tee /etc/pidex/env <<< 'TELEGRAM_BOT_TOKEN=your_token
TELEGRAM_CHAT_ID=your_chat_id'
sudo chmod 600 /etc/pidex/env

# Monitor settings, thresholds, intervals, etc.
sudo cp config/config.toml.example /etc/pidex/config.toml
sudo nano /etc/pidex/config.toml
```

Environment variables (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`) take priority over config files. If a `.env` file exists alongside the binary (or in the current directory), it is loaded automatically via `python-dotenv`.

## Building from Source

Prebuilt binaries are shipped for linux/amd64 and linux/arm64. For other architectures (e.g. armv7l, riscv64):

```bash
# 1. Clone
git clone https://github.com/Shivamingale3/pi_dex.git
cd pi_dex

# 2. Install Python dependencies
pip install -r requirements.txt

# 3. Build with PyInstaller
pip install pyinstaller
pyinstaller pidex.spec
# Binary at: dist/pidex

# 4. Install manually
sudo install -m 0755 dist/pidex /usr/local/bin/pidex
sudo useradd --system --no-create-home --shell /usr/sbin/nologin pidex 2>/dev/null
sudo mkdir -p /etc/pidex
sudo cp config/config.toml.example /etc/pidex/config.toml
sudo cp deploy/pidex.service /etc/systemd/system/
sudo cp deploy/pidex-shutdown.service /etc/systemd/system/
sudo systemctl daemon-reload
```

For development: `pip install -e .` installs `pidex` and `pidex-shutdown` as editable console commands.

## Running Tests

```bash
pip install -e ".[dev]"
pytest -v
```

## Uninstall

```bash
sudo systemctl disable --now pidex pidex-shutdown
sudo rm -f /usr/local/bin/pidex /etc/systemd/system/pidex.service /etc/systemd/system/pidex-shutdown.service
sudo rm -rf /etc/pidex
sudo userdel pidex 2>/dev/null
sudo systemctl daemon-reload
```

Or run the bundled script: `sudo deploy/uninstall.sh`

## Architecture

See [docs/architecture.md](docs/architecture.md) for design principles, event model, and data flow.
