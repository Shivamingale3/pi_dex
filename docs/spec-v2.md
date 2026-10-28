# PiDex v2.0.0 Spec

Status: approved, ready to implement. Base: v1.3.3 (`8d2bece`).
Target host: Raspberry Pi 5 (arm64, Raspberry Pi OS Bookworm, systemd, Docker, Tailscale).

## 0. Ground rules
- Go 1.22, keep dependencies minimal (only `BurntSushi/toml` today). Prefer stdlib.
- PiDex must be fully independent of Tailscale and of network readiness.
- Add unit tests (`go test ./...`) for every new package. None exist today.
- Bump `core.Version` to `2.0.0`, update `docs/changelog.md` and README.
- Do not commit secrets (see section 7).

## 1. Boot independence
- `deploy/pidex.service`: replace `After=network-online.target docker.service` and
  `Wants=network-online.target` with `After=network.target`. Must not depend on
  `network-online.target`, `tailscaled.service` or `docker.service`.
  (Host will later make `network-online.target` wait for a Tailscale IP. PiDex must not be blocked by that.)
- `waitForDNS()` must no longer gate anything. Startup events go through the offline queue (section 2).
- Docker source already retries every 5s, keep that (Docker may start later than PiDex).
- `install.sh` / `update` must install the new unit and run `systemctl daemon-reload`.

## 2. Persistent offline queue
- When `TelegramNotifier.Send` fails after its retries, append the event to
  `/var/lib/pidex/queue.jsonl` (StateDirectory, already owned by pidex user).
- Background flusher: every 30s, if queue non empty, try to send in FIFO order.
  Stop at first failure (still offline). On success remove that entry (clear on send).
  Rewrite file atomically (write temp + rename).
- New events while queue is non empty go to the back of the queue (preserve order).
- Queued messages get a line: `Delayed: originally at <timestamp>`.
- Cap: 500 entries, drop oldest, and when flushing send one summary: `N alerts dropped while offline`.
- Load queue on startup (survives PiDex restart and reboot).
- Shutdown: on SIGTERM, if offline, persist in-flight event instead of losing it.

## 3. Fix service watch matching (bug)
- `parser.isWatched` uses `path.Match(pattern, "cloudflared.service")`, so plain names like
  `"cloudflared"` never match. Live config on the Pi uses plain names, so systemd alerts never fire.
- Fix: normalise both sides. If pattern has no `.service` suffix, match against the name with
  suffix stripped. Glob still supported. Add tests: `cloudflared`, `cloudflared.service`, `redis-*`.
- Also catch `Dependency failed for` and `<unit>: Failed with result` journal lines as SERVICE_FAILED
  (dependency failures use the Description, so map via `UNIT` journal field when present).

## 4. Health check module (new: `internal/health`)
Each check runs in its own goroutine on its own interval.

### Config
```toml
[[health_checks]]
name = "redis-cache"            # unique, required
type = "redis"                  # http | tcp | redis | systemd | docker | command | interface
address = "100.83.1.25:6379"    # http: full URL; tcp/redis: host:port
unit = "redis-cache.service"    # optional, used for diagnosis + logs
container = ""                  # optional, docker container name, same purpose
interface = "tailscale0"        # optional, used for diagnosis
interval = 30                   # seconds, default 30
timeout = 5                     # seconds, default 5
failures = 3                    # consecutive failures before alert, default 3
password = ""                   # redis AUTH (see section 7)
username = ""                   # redis ACL user, optional
expect_status = 200             # http only, default 200
headers = { }                   # http only, optional
command = ""                    # command type: run via /bin/sh -c, exit 0 = healthy
```

### Check types
| type | healthy when |
|---|---|
| http | GET `address` returns `expect_status` within timeout |
| tcp | TCP connect to `address` succeeds |
| redis | connect, `AUTH` if set, `PING` returns `PONG` (handwritten RESP, no lib) |
| systemd | `systemctl is-active <unit>` = `active` |
| docker | container state running, and health = healthy if it has a healthcheck (docker socket API) |
| command | `command` exits 0 within timeout |
| interface | `interface` exists, is UP and has an IPv4 address |

Connecting to the exact configured address proves the service is bound on the intended interface.

### State machine
- States: unknown, healthy, failing, down.
- `failures` consecutive failures -> DOWN alert (CRITICAL). One alert only, no repeats.
- First success after DOWN -> RECOVERED alert with downtime duration.
- Startup: first result sets state silently, except an already-failing check alerts after `failures`.
- Event Source = `health:<name>` so dedup/cooldown work per check.

### Diagnosis on failure (put into the alert as "Cause")
Run in order, report every finding:
1. Raw error: connection refused / timeout after Ns / HTTP 500 / AUTH failed / exit code N.
2. If `unit` set: `systemctl show -p ActiveState,SubState,Result,ExecMainStatus <unit>`.
3. If `container` set: state, exit code, health status from docker API.
4. If `interface` set or address IP is local: does that IP exist on any interface right now?
5. For tcp/redis/http on a local IP: is the port listening on a different address?
   (parse `/proc/net/tcp` and `/proc/net/tcp6`; example: "listening on 127.0.0.1:6379 only").
6. Last 10 log lines: `journalctl -u <unit> -n 10 --no-pager` or docker logs tail 10.
   Truncate so the Telegram message stays under 4096 chars, HTML-escape.

### Permissions
PiDex runs as user `pidex`. Installer must ensure group membership needed for:
journal read (`systemd-journal`), docker socket (`docker`). Document this.

## 5. Setup wizard
Add menu item `Manage health checks`: list, add (prompt per field by type), edit, delete,
test now (run once, print result + diagnosis, no Telegram). Validate: unique name, required
fields per type, interval >= 5. Save via existing `saveConfig`.
Also add `pidex test --health <name>` CLI.

## 6. Example checks to ship in config.toml.example
```toml
[[health_checks]]
name = "tailscale-ip"
type = "interface"
interface = "tailscale0"
interval = 30
failures = 6          # 3 min grace at boot

[[health_checks]]
name = "redis-cache"
type = "redis"
address = "100.83.1.25:6379"
unit = "redis-cache.service"
interface = "tailscale0"
```

## 7. Secrets
- Owner decision: health check secrets (redis password, http headers) live in `config.toml`.
- Installer must set `/etc/pidex/config.toml` to `0640 root:pidex`.
- Wizard and `view config` must mask secrets.
- IMPORTANT: `config/config.toml` is listed in `.gitignore` but is still tracked by git and holds
  the real Telegram bot token. Run `git rm --cached config/config.toml`. Owner must rotate the
  bot token in BotFather if the repo was ever public. Keep only `config.toml.example` in git.

## 8. Acceptance tests (manual, on the Pi)
1. Reboot with internet unplugged: PiDex starts, events queue, plug in, all arrive in order with Delayed line.
2. `systemctl stop redis-cache` -> DOWN alert after ~90s with cause + logs. Start it -> RECOVERED.
3. Bind redis to 127.0.0.1 only -> alert says listening on 127.0.0.1 only.
4. `tailscale down` -> tailscale-ip DOWN. `tailscale up` -> RECOVERED.
5. `systemctl restart cloudflared` -> systemd alert fires with plain-name watch list.
6. Restart PiDex while queue non-empty -> queue survives.
