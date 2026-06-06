# PiDex

Event-driven home server watchman. Listens for SSH logins, Docker events, systemd failures, resource thresholds, netlink interface changes, and sudo commands — forwards them as Telegram notifications.

## Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/Shivamingale3/pi_dex/main/deploy/install.sh | sudo env "PATH=$PATH" bash
sudo pidex setup          # Interactive wizard
sudo systemctl enable --now pidex
```

**Prerequisites**: Go 1.22+ (for building), Debian 12 / Ubuntu 24.04+ / Raspberry Pi OS (64-bit), x86_64 or aarch64.

## Commands

| Command | Description |
|---------|-------------|
| `pidex run` | Start the daemon |
| `pidex setup` | Interactive configuration wizard (9 settings) |
| `pidex test <event>` | Send a test notification (ssh-login, ssh-fail, sudo, docker-down, reboot) |
| `pidex test <event> --dry-run` | Print event without sending |
| `pidex uninstall` | Remove PiDex, systemd services, and configuration |
| `pidex help` | Show full usage |

## Notification Format

```
ℹ️ INFO | SSH Login
shiv logged in from 192.168.1.100

  Server    asus
  Source    ssh
  Time      2026-06-03 15:04:05

— PiDex v1.1.0
```

Severity levels: `INFO`, `WARNING`, `CRITICAL`, `RECOVERED` — each with a distinct icon.

## Configuration

Run `pidex setup` for an interactive wizard, or manually edit:

| File | Contents | Mode |
|------|----------|------|
| `/etc/pidex/env` | `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID` | 600 |
| `/etc/pidex/config.toml` | All monitor settings, thresholds, intervals | 640 |

Environment variables (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`) take priority over config files.

## Building from Source

```bash
git clone https://github.com/Shivamingale3/pi_dex.git
cd pi_dex

go build -ldflags="-s -w" -o pidex ./cmd/pidex
go build -ldflags="-s -w" -o pidex-shutdown ./cmd/pidex-shutdown

sudo install -m 0755 pidex /usr/local/bin/pidex
sudo install -m 0755 pidex-shutdown /usr/local/bin/pidex-shutdown
sudo useradd --system --no-create-home --shell /usr/sbin/nologin pidex 2>/dev/null
sudo mkdir -p /etc/pidex
sudo cp config/config.toml.example /etc/pidex/config.toml
sudo cp deploy/pidex.service /etc/systemd/system/
sudo cp deploy/pidex-shutdown.service /etc/systemd/system/
sudo systemctl daemon-reload
```

## Running Tests

```bash
go test ./...
```

## Uninstall

```bash
sudo pidex uninstall
```

Or manually:

```bash
sudo systemctl disable --now pidex pidex-shutdown
sudo rm -f /usr/local/bin/pidex /usr/local/bin/pidex-shutdown /etc/systemd/system/pidex.service /etc/systemd/system/pidex-shutdown.service
sudo rm -rf /etc/pidex
sudo userdel pidex 2>/dev/null
sudo systemctl daemon-reload
```

## Architecture

See [docs/architecture.md](docs/architecture.md) for design principles, event model, and data flow.
