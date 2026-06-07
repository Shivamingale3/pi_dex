# PiDex Custom Service Integration — Developer Requirements

## 1. Overview

PiDex is an event-driven notifier that watches journald and forwards matched events to Telegram. External apps can integrate by writing structured log lines to journald and providing a small TOML config file.

```
Your App  →  journald (MESSAGE field)  →  PiDex regex match  →  Telegram notification
```

## 2. The Contract

| Your app does | PiDex does |
|--------------|------------|
| Writes meaningful log lines to journald | Tails journald constantly |
| Provides a `.conf` file defining events + regex patterns | Regex-matches journald `MESSAGE` against each pattern |
| Runs as a systemd service (`systemctl list-unit-files <name>.service` succeeds) | Filters journald by `_SYSTEMD_UNIT` or `SYSLOG_IDENTIFIER` matching the config `name` |
| Nothing else | Rate-limits (cooldowns), deduplicates, formats, sends to Telegram |

## 3. Journald Output Requirements

Your app must write log lines to journald. Any of these mechanisms work:

| Method | journald field set | Example |
|--------|-------------------|---------|
| Systemd service (stdout/stderr) | `_SYSTEMD_UNIT=<name>.service` | `fmt.Println("server started on port 8080")` |
| `logger -t <name>` | `SYSLOG_IDENTIFIER=<name>` | `logger -t myapp "server started on port 8080"` |
| `syslog` library call | `SYSLOG_IDENTIFIER=<name>` | `syslog.Writer("myapp", syslog.LOG_INFO)` |

**Rules:**
- The identifier (`name`) used in journald must exactly match the `name` field in the config file.
- Only the `MESSAGE` field is regex-matched. Other journald fields (PID, hostname, timestamps) are invisible to your pattern.
- PiDex events have **no access to structured data**. The `title` and `message` sent to Telegram are static strings defined in the config file.

## 4. Config File Specification

Format: TOML. Drop in `/etc/pidex/custom.d/<name>.conf`.

### Schema

```toml
# Required. Must match the systemd unit name (without .service suffix)
# and the journald identifier (SYSLOG_IDENTIFIER or _SYSTEMD_UNIT stripped of .service).
name = "myapp"

# Optional. Shown in the setup menu for identification.
description = "My Application"

# Array of event definitions. At least one required.
[[events]]

# Required. Unique event identifier. Sent as the Telegram event type.
# Convention: UPPER_SNAKE_CASE prefixed with app name, e.g. MYAPP_STARTED.
name = "MYAPP_STARTED"

# Required. Go regex matched against the journald MESSAGE field.
# Literal strings like "server started" work fine.
# More complex: "(?i)(fatal|panic)" for case-insensitive matching.
# Pattern is compiled with Go's regexp.Compile().
pattern = "server started on port"

# Required. One of: INFO, WARNING, CRITICAL, RECOVERED.
# Controls the Telegram notification icon and severity label.
severity = "INFO"

# Required. Telegram notification headline. Static text.
title = "MyApp Started"

# Required. Telegram notification body. Static text.
# No variable substitution — sent as literal text.
message = "MyApp server is now running"
```

### Multiple Events

```toml
name = "myapp"
description = "My Application"

[[events]]
name = "MYAPP_STARTED"
pattern = "server started"
severity = "INFO"
title = "MyApp Started"
message = "MyApp server is now running"

[[events]]
name = "MYAPP_ERROR"
pattern = "(?i)(fatal|panic|segfault)"
severity = "CRITICAL"
title = "MyApp Crashed"
message = "MyApp encountered a fatal error"

[[events]]
name = "MYAPP_STOPPED"
pattern = "shutting down"
severity = "WARNING"
title = "MyApp Stopped"
message = "MyApp is shutting down"
```

### Field Reference

| Field | Required | Type | Purpose |
|-------|----------|------|---------|
| `name` | Yes | string | Matches journald identifier and systemd unit name |
| `description` | No | string | Label in setup menu |
| `events[].name` | Yes | string | Telegram event type, must be unique per service |
| `events[].pattern` | Yes | string | Go regex matched against journald `MESSAGE` |
| `events[].severity` | Yes | string | `INFO` / `WARNING` / `CRITICAL` / `RECOVERED` |
| `events[].title` | Yes | string | Telegram headline (static) |
| `events[].message` | Yes | string | Telegram body (static) |

## 5. Service Requirements

- Your app must run as a **systemd service** (`systemctl list-unit-files myapp.service` must succeed).
- PiDex validates this at registration time. If the unit doesn't exist, registration is rejected.
- The service must use its unit name consistently in log output (automatic for systemd-managed services).

## 6. Registration and Activation

End-user steps (not the app developer):

```bash
# 1. Drop the .conf file
sudo cp myapp.conf /etc/pidex/custom.d/

# 2. Register via setup wizard
sudo pidex setup
#    → 10. Manage custom services
#    → select myapp.conf
#    → R (Register)

# 3. Restart daemon to load the new parser
sudo systemctl restart pidex
```

The `.conf` file is consumed during registration — it's saved into `/etc/pidex/config.toml` and removed from the staging directory. To update, drop a new `.conf` and re-register.

## 7. Testing Integration

From the developer's machine:

```bash
# Dry-run — prints what would be emitted without writing to journald
pidex test --emit --service myapp --event MYAPP_STARTED --dry-run

# Full test — writes to journald, PiDex daemon picks it up and sends Telegram
pidex test --emit --service myapp --event MYAPP_STARTED

# For patterns with regex syntax that can't be auto-generated:
pidex test --emit --service myapp --event MYAPP_ERROR --message "FATAL: out of memory"
```

The test searches both registered services and unregistered `.conf` files in `/etc/pidex/custom.d/`. No need to register before testing.

## 8. Runtime Behavior

| Scenario | Behavior |
|----------|----------|
| Same log line matches multiple event patterns | First matching `[[events]]` entry wins (order in config) |
| Same event fires twice in a row | Deduplicated by source+type+title+message hash |
| Event fires rapidly | Rate-limited by cooldowns (default: 0s for custom events = no limit; user can override in `[cooldowns]`) |
| Service stops logging | No notification — PiDex only reacts to log lines, not absence |
| Service unit removed from system | No error. PiDex simply receives no matching entries |
| Multiple registered services with same name | Last registered wins (overwrites) |

## 9. Constraints and Limitations

- **Pattern operates on MESSAGE only** — cannot match on PID, exit code, resource usage, or structured JSON fields.
- **Messages are static** — regex capture groups are not substituted into Telegram output.
- **No structured logging support** — journald JSON fields beyond `MESSAGE` are not accessible.
- **Registration requires systemd** — `systemctl list-unit-files <name>.service` must succeed.
- **Config changes require daemon restart** — no hot-reload.
- **No event filtering at runtime** — all registered events are active. To disable one, remove it from the config and re-register.
- **Reserved names** — cannot use names matching built-in sources: `ssh`, `sudo`, `docker`, `systemd`, `network`, `shutdown`, `cpu`, `ram`, `disk`, `temperature`, `daemon`.

## 10. Example: Complete Integration

**Your app (`/usr/local/bin/myapp`):**
```go
package main
import "fmt"
func main() {
    fmt.Println("server started on port 8080")
    // ...
    fmt.Println("shutting down")
}
```

**Systemd unit (`/etc/systemd/system/myapp.service`):**
```ini
[Service]
ExecStart=/usr/local/bin/myapp
```

**PiDex config (`/etc/pidex/custom.d/myapp.conf`):**
```toml
name = "myapp"
description = "My Application"
[[events]]
name = "MYAPP_STARTED"
pattern = "server started on port"
severity = "INFO"
title = "MyApp Started"
message = "MyApp server is now running"
[[events]]
name = "MYAPP_STOPPED"
pattern = "shutting down"
severity = "WARNING"
title = "MyApp Stopped"
message = "MyApp is shutting down"
```

**Result:** When systemd starts `myapp.service`, it prints `"server started on port 8080"` to journald. PiDex regex-matches `"server started on port"`, sends a Telegram notification with title `"MyApp Started"` and body `"MyApp server is now running"`.
