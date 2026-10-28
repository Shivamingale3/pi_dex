# PiDex

Event-driven home server watchman. Listens for SSH logins, Docker events, systemd failures, resource thresholds, netlink interface changes, and sudo commands, and runs active health checks — forwards them as Telegram notifications.

PiDex starts at boot without waiting for the network (or Tailscale, or Docker). Notifications that cannot be delivered are kept in a persistent offline queue and sent, in order, once Telegram is reachable.

## Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/Shivamingale3/pi_dex/main/deploy/install.sh | sudo bash
sudo pidex setup          # Interactive wizard
sudo systemctl enable --now pidex
```

**Prerequisites**: Debian 12 / Ubuntu 24.04+ / Raspberry Pi OS (64-bit), x86_64 or aarch64.

## Commands

| Command | Description |
|---------|-------------|
| `pidex run` | Start the daemon |
| `pidex setup` | Interactive configuration wizard (credentials, monitors, thresholds, custom services, health checks) |
| `pidex test <event>` | Send a test notification (ssh-login, ssh-fail, sudo-used, docker-down, reboot) |
| `pidex test <event> --dry-run` | Print event without sending |
| `pidex test --health <name>` | Run one health check now and print the result and diagnosis (no Telegram) |
| `pidex update` | Update binaries and systemd units to the latest release |
| `pidex uninstall` | Remove PiDex, systemd services, and configuration |
| `pidex help` | Show full usage |

## Notification Format

```
ℹ️ INFO | SSH Login
shiv logged in from 192.168.1.100

  Server    asus
  Source    ssh
  Time      2026-06-03 15:04:05

— PiDex v2.0.0
```

Severity levels: `INFO`, `WARNING`, `CRITICAL`, `RECOVERED` — each with a distinct icon.

A notification delivered from the offline queue has an extra line, `Delayed: originally at <time>`.

## Offline Queue

If Telegram cannot be reached (no internet at boot, DNS down, API errors), PiDex appends the notification to `/var/lib/pidex/queue.jsonl` instead of dropping it.

- Every 30s the queue is flushed oldest first; flushing stops at the first failure.
- While anything is queued, new notifications go behind it, so order is preserved.
- The queue survives PiDex restarts and reboots. On shutdown, a notification that cannot be sent is saved.
- At most 500 entries are kept. Older ones are dropped and reported once: `N alerts dropped while offline`.

## Health Checks

Health checks actively probe services and alert when they stop working. Each check runs on its own interval. After `failures` consecutive failures a single CRITICAL alert is sent; the next success sends RECOVERED with the downtime.

| type | healthy when |
|---|---|
| `http` | GET `address` returns `expect_status` (default 200) within `timeout` |
| `tcp` | TCP connect to `address` succeeds |
| `redis` | connect, `AUTH` if `password` is set, `PING` returns `PONG` |
| `systemd` | `systemctl is-active <unit>` is `active` |
| `docker` | container is running, and healthy if it has a healthcheck |
| `command` | `command` (run via `/bin/sh -c`) exits 0 within `timeout` |
| `interface` | `interface` exists, is UP and has an IPv4 address |

```toml
[[health_checks]]
name = "tailscale-ip"
type = "interface"
interface = "tailscale0"
failures = 6          # 3 min grace at boot

[[health_checks]]
name = "redis-cache"
type = "redis"
address = "100.83.1.25:6379"
unit = "redis-cache.service"   # optional: adds unit state + logs to the alert
interface = "tailscale0"       # optional: checks the interface and its IP
```

Defaults: `interval = 30`, `timeout = 5`, `failures = 3`. See `config/config.toml.example` for every field.

A DOWN alert includes a diagnosis: the raw error (connection refused, timeout, HTTP 500, AUTH failed, exit code), the unit's `ActiveState/SubState/Result`, the container state, whether the target IP currently exists on an interface, which address the port is actually listening on (for example `listening on 127.0.0.1:6379 only`), and the last 10 log lines.

Manage checks with `sudo pidex setup` → **11. Manage health checks**, or try one with `sudo pidex test --health <name>`.

## Permissions

The daemon runs as the `pidex` system user. The installer (and `pidex update`) add it to:

| Group | Needed for |
|---|---|
| `systemd-journal` | reading the journal (SSH/sudo/systemd events, health check logs) |
| `docker` | the Docker socket (container events, `docker` checks, container logs) |
| `adm` | reading log files on some distributions |

Systemd unit state (`systemctl is-active`, `systemctl show`) and `/proc/net/tcp` need no extra permissions.

## Configuration

Run `pidex setup` for an interactive wizard, or manually edit:

| File | Contents | Mode |
|------|----------|------|
| `/etc/pidex/env` | `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID` | 600 |
| `/etc/pidex/config.toml` | All monitor settings, thresholds, intervals, health checks | 640 `root:pidex` |
| `/var/lib/pidex/queue.jsonl` | Undelivered notifications | 600 `pidex` |

Environment variables (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`) take priority over config files.

Health check secrets (redis `password`, HTTP `headers`) live in `config.toml`, which is why it must stay `root:pidex` mode 640. The wizard and *View current config* mask them.

The `[services] watch` list accepts plain names (`"cloudflared"`), unit names (`"cloudflared.service"`) and globs (`"redis-*"`).

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
