# PiDex Architecture

## Overview

PiDex is an event-driven home server watchman. It listens for operating system
events and forwards them as Telegram notifications. It is NOT a monitoring
platform — no dashboards, no metrics storage, no graphs.

## Core Principle

> Let Linux detect events. Let PiDex route and notify.

## Architecture Diagram

```
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

## Concurrency Model

Threading + `queue.Queue`. Each source and poller runs in its own daemon thread.
All threads publish `Event` objects into a shared `queue.Queue`. A single
dispatcher thread reads from the queue, applies cooldowns and deduplication,
then forwards to notification backends.

### Thread Table

| Thread         | Role                              | I/O Pattern              |
|----------------|-----------------------------------|--------------------------|
| Main           | Orchestrate startup/shutdown      | None                     |
| Dispatcher     | Read queue → cooldown → dedup → notify | `queue.get()` (blocking) |
| Journal        | Follow journald for SSH, sudo, systemd | `systemd.journal.wait()` |
| Docker         | Docker Events API stream          | Docker socket stream     |
| Network        | Netlink interface events          | pyroute2 netlink         |
| Shutdown       | SIGTERM handler                   | signal                   |
| CPU poller     | Read `/proc/stat` every 15s       | Sleep-based polling      |
| RAM poller     | Read `/proc/meminfo` every 30s    | Sleep-based polling      |
| Temp poller    | Read thermal zone every 30s       | Sleep-based polling      |
| Disk poller    | Read `statvfs` every 300s         | Sleep-based polling      |

## Event Model

All internal communication uses a single `Event` dataclass:

```python
@dataclass
class Event:
    source: str
    event_type: str
    severity: str
    title: str
    message: str
    timestamp: datetime
```

No source may send Telegram messages directly. Every source emits `Event`
objects into the bus.

## Event Bus

A single `queue.Queue` is the only communication path between threads. Sources
never communicate with each other.

## Dispatcher

Responsibilities:
1. Receive events from the bus
2. Apply per-event-type cooldowns
3. Deduplicate identical events (hash of source + event_type + title + message)
4. Route to notification services

## Directory Layout

```
pidex/
├── pidex.py                  # CLI entry point
├── pidex/                    # Main package
│   ├── core/                 # Core abstractions
│   │   ├── constants.py      # All hardcoded defaults
│   │   ├── event.py          # Event dataclass
│   │   ├── bus.py            # EventBus (queue wrapper)
│   │   ├── dispatcher.py     # Dispatcher thread
│   │   ├── cooldowns.py      # Cooldown manager
│   │   └── dedup.py          # Dedup manager
│   ├── sources/              # Event sources (event-driven)
│   │   ├── base.py           # BaseSource ABC
│   │   ├── journal.py        # Journald listener
│   │   ├── ssh.py            # SSH parser
│   │   ├── sudo.py           # Sudo parser
│   │   ├── docker.py         # Docker events
│   │   ├── systemd.py        # Systemd parser
│   │   ├── network.py        # Netlink listener
│   │   └── shutdown.py       # Shutdown handler
│   ├── pollers/              # Polling sources
│   │   ├── base.py           # BasePoller ABC
│   │   ├── cpu.py
│   │   ├── ram.py
│   │   ├── disk.py
│   │   └── temperature.py
│   ├── notifiers/            # Notification backends
│   │   ├── base.py           # BaseNotifier ABC
│   │   └── telegram.py       # Telegram sender
│   └── config/               # Configuration
│       └── loader.py         # TOML config loader
├── config/
│   └── config.toml           # User configuration
├── tests/
├── docs/
│   ├── architecture.md       # This file
│   ├── decisions.md          # Decision records
│   └── changelog.md          # Development log
├── requirements.txt
└── README.md
```
