# PiDex v2 Architecture Specification

## Purpose

PiDex is an event-driven home server watchman for Raspberry Pi and Linux servers.

The primary goal is:

> Receive important operational events from the operating system and infrastructure and immediately forward them to Telegram.

PiDex is NOT intended to be a traditional monitoring system.

PiDex should not continuously poll for information that Linux, systemd, Docker, or the kernel already provide as events.

The design philosophy is:

> Let Linux detect events.
>
> Let PiDex route and notify.

---

# Core Principles
git remote add origin git@github.com:Shivamingale3/pi_dex.git
git branch -M main
git push -u origin main
## Principle 1 — Event Driven First

Always prefer subscriptions and event streams over polling.

Good:

* journald subscriptions
* Docker events
* systemd events
* netlink events
* shutdown hooks

Avoid:

* repeatedly reading log files
* repeatedly running shell commands
* repeatedly checking service status

Polling is only acceptable when Linux provides no event source.

---

## Principle 2 — Notification Broker

PiDex is an event broker.

PiDex should never become a large monitoring platform.

Responsibilities:

* receive events
* normalize events
* deduplicate events
* apply cooldowns
* send notifications

Not Responsibilities:

* dashboards
* metrics storage
* graphs
* historical analytics

Those belong in Grafana, Prometheus, Netdata, etc.

---

## Principle 3 — Minimal Resource Usage

PiDex must remain nearly invisible.

Target:

* RAM < 50 MB
* CPU ~0% idle
* Network activity only during notifications

The daemon should spend most of its life sleeping.

---

# Architecture

```text
Linux / Docker / Systemd
            │
            ▼
     Event Sources
            │
            ▼
       Event Bus
            │
            ▼
       Dispatcher
            │
            ▼
      Notification
            │
            ▼
        Telegram
```

---

# Event Model

All internal communication uses a single event structure.

```python
from dataclasses import dataclass
from datetime import datetime

@dataclass
class Event:
    source: str
    event_type: str
    severity: str
    title: str
    message: str
    timestamp: datetime
```

Example:

```python
Event(
    source="ssh",
    event_type="login",
    severity="INFO",
    title="SSH Login",
    message="shiv logged in from 192.168.1.100",
    timestamp=datetime.now()
)
```

Every source must emit Event objects.

No source may directly send Telegram messages.

---

# Event Sources

## SSH Source

Source:

* journald

Events:

* SSH_LOGIN
* SSH_LOGOUT
* SSH_BRUTEFORCE

---

## Sudo Source

Source:

* journald

Events:

* SUDO_USED

---

## Docker Source

Source:

* Docker Events API

Events:

* CONTAINER_STARTED
* CONTAINER_STOPPED
* CONTAINER_DIED
* CONTAINER_RESTARTED

---

## Systemd Source

Source:

* systemd

Events:

* SERVICE_STARTED
* SERVICE_STOPPED
* SERVICE_FAILED
* SERVICE_RESTARTED

Important services:

* cloudflared
* docker
* nginx
* caddy
* tailscale
* user-defined services

---

## Network Source

Source:

* netlink

Events:

* INTERFACE_UP
* INTERFACE_DOWN

Examples:

* eth0
* wlan0

---

## Shutdown Source

Source:

* systemd hooks

Events:

* SHUTDOWN_STARTED
* REBOOT_STARTED

Goal:

Notify before the machine goes offline.

---

# Polling Sources

Polling is allowed only where Linux does not emit usable events.

## Temperature Poller

Source:

```text
/sys/class/thermal/thermal_zone0/temp
```

Default interval:

```text
30 seconds
```

Events:

* TEMP_WARN
* TEMP_CRITICAL
* TEMP_RECOVERED

---

## CPU Poller

Default interval:

```text
15 seconds
```

Events:

* CPU_HIGH
* CPU_RECOVERED

---

## RAM Poller

Default interval:

```text
30 seconds
```

Events:

* RAM_HIGH
* RAM_RECOVERED

---

## Disk Poller

Default interval:

```text
5 minutes
```

Events:

* DISK_WARN
* DISK_CRITICAL
* DISK_RECOVERED

---

# Event Bus

All sources push events into a single queue.

Example:

```python
asyncio.Queue
```

or

```python
queue.Queue
```

The queue is the only communication path.

Sources never communicate with each other.

---

# Dispatcher

Responsibilities:

* receive events
* apply cooldowns
* deduplicate
* route to notification services

Example:

```text
SSH_LOGIN
     │
     ▼
Dispatcher
     │
     ▼
Telegram
```

---

# Deduplication

Prevent repeated identical alerts.

Example:

```text
Cloudflared crashed
Cloudflared crashed
Cloudflared crashed
```

Only first alert should be sent.

Store:

```python
last_event_hash
```

per source.

---

# Cooldowns

Each event type can have its own cooldown.

Examples:

```text
SSH_LOGIN
0 seconds

CPU_HIGH
300 seconds

TEMP_CRITICAL
300 seconds

DISK_CRITICAL
3600 seconds
```

---

# Notification Layer

Current target:

Telegram only.

Future:

* Discord
* Email
* Webhooks

Notification providers must implement a common interface.

```python
class Notifier:
    def send(event: Event):
        pass
```

---

# Testing Philosophy

Everything must be testable on a laptop.

Development must not require a Raspberry Pi.

---

## Fake Event Generator

Examples:

```bash
python pidex.py test ssh-login
python pidex.py test ssh-fail
python pidex.py test docker-down
python pidex.py test reboot
```

These commands create synthetic events.

---

## Dry Run Mode

Example:

```bash
python pidex.py test ssh-login --dry-run
```

Print message.

Do not contact Telegram.

---

# Development Order

Phase 1

* Event model
* Dispatcher
* Telegram sender

Phase 2

* Fake event generator
* Dry-run mode

Phase 3

* Config system
* Deduplication
* Cooldowns

Phase 4

* SSH source
* Docker source
* Systemd source
* Network source

Phase 5

* CPU poller
* RAM poller
* Temperature poller
* Disk poller

Phase 6

* Packaging
* systemd service
* Raspberry Pi deployment

---

# Non Goals

PiDex is NOT:

* Grafana
* Prometheus
* Netdata
* Zabbix
* Nagios

PiDex exists only to notify.

If an event matters, Telegram should receive it.

If it does not matter, PiDex should remain silent.

---

# Technology Decisions

Language:

Python 3

Reason:

* fastest development
* easiest maintenance
* sufficient performance
* event-driven architecture matters more than language choice

Future rewrites are allowed.

Current priority is functionality and reliability, not micro-optimizations.

---

## Project Structure

PiDex is a Python daemon/service.

PiDex is NOT a web application.

PiDex v1 will not use:

* Django
* FastAPI
* Flask
* SQLAlchemy
* PostgreSQL
* Redis

The goal is to keep deployment and maintenance simple.

PiDex runs as a single systemd service.

---

### Repository Layout

```text
pidex/
│
├── pidex.py
│
├── core/
│   ├── event.py
│   ├── dispatcher.py
│   ├── cooldowns.py
│   ├── dedup.py
│   └── config.py
│
├── sources/
│   ├── ssh.py
│   ├── docker.py
│   ├── systemd.py
│   ├── network.py
│   └── shutdown.py
│
├── pollers/
│   ├── cpu.py
│   ├── ram.py
│   ├── disk.py
│   └── temperature.py
│
├── notifiers/
│   ├── base.py
│   └── telegram.py
│
├── tests/
│   ├── fake_events.py
│   └── cli.py
│
├── config/
│   └── config.toml
│
├── requirements.txt
├── ARCHITECTURE.md
└── README.md
```

---

### Execution Model

The application starts with:

```python
load_config()

start_dispatcher()

start_event_sources()

start_pollers()

wait_forever()
```

There is no web server.

There are no HTTP endpoints.

There is no database.

All runtime state is held in memory.

Configuration is stored in TOML files.

---

### Future Expansion

If a dashboard is ever required:

* FastAPI may be added as a separate component.
* The dashboard must not become a dependency of the core daemon.
* The alerting engine must continue functioning without the dashboard.

The core daemon remains the primary product.


# Guiding Rule

Whenever implementing a new feature ask:

"Can Linux already tell me when this happens?"

If yes:

Use the event.

If no:

Use polling.
