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

Goroutines + channels (Go). Each source and poller runs in its own goroutine.
All sources publish `Event` values into a shared `EventBus` via a buffered
channel. A single dispatcher goroutine reads from the bus, applies cooldowns
and deduplication, then forwards to notification backends.

### Goroutine Table

| Goroutine      | Role                              | I/O Pattern              |
|----------------|-----------------------------------|--------------------------|
| Main           | Orchestrate startup/shutdown      | None                     |
| Dispatcher     | Read bus → cooldown → dedup → notify | channel receive (blocking) |
| Journal        | Follow journald for SSH, sudo, systemd | sdjournal wait         |
| Docker         | Docker Events API stream          | Docker socket stream     |
| Network        | Netlink interface events          | netlink (unix socket)    |
| Shutdown       | SIGTERM/SIGINT handler            | signal                   |
| CPU poller     | Read `/proc/stat` every interval  | Sleep-based polling      |
| RAM poller     | Read `/proc/meminfo` every interval | Sleep-based polling    |
| Temp poller    | Read thermal zone every interval  | Sleep-based polling      |
| Disk poller    | Read `statfs` every interval      | Sleep-based polling      |

## Event Model

All internal communication uses a single `Event` struct:

```go
type Event struct {
    Source    string
    EventType string
    Severity  string
    Title     string
    Message   string
    Timestamp time.Time
}
```

No source may send Telegram messages directly. Every source emits `Event`
values into the bus.

## Event Bus

A single buffered channel (`chan Event`) is the only communication path
between goroutines. Sources never communicate with each other.

## Dispatcher

Responsibilities:
1. Receive events from the bus
2. Apply per-event-type cooldowns
3. Deduplicate identical events (hash of source + event_type + title + message)
4. Route to notification services

## Directory Layout

```
pi_dex/
├── cmd/
│   ├── pidex/                  # CLI entry point (run, setup, test, uninstall)
│   └── pidex-shutdown/         # Shutdown notification entry point
├── internal/
│   ├── core/                   # Core abstractions
│   │   ├── constants.go        # All hardcoded defaults
│   │   ├── event.go            # Event struct
│   │   ├── bus.go              # EventBus (channel wrapper)
│   │   ├── dispatcher.go       # Dispatcher goroutine
│   │   ├── cooldowns.go        # Cooldown manager
│   │   └── dedup.go            # Dedup manager
│   ├── source/                 # Event sources (event-driven)
│   │   ├── source.go           # Source interface
│   │   ├── journal.go          # Journald listener
│   │   ├── docker.go           # Docker events
│   │   ├── network.go          # Netlink listener
│   │   ├── parser.go           # Parser interface
│   │   ├── ssh.go              # SSH parser
│   │   ├── sudo.go             # Sudo parser
│   │   └── systemd.go          # Systemd parser
│   ├── poller/                 # Polling sources
│   │   ├── poller.go           # BasePoller (state machine)
│   │   ├── cpu.go
│   │   ├── ram.go
│   │   ├── disk.go
│   │   └── temperature.go
│   ├── notifier/               # Notification backends
│   │   ├── notifier.go         # Notifier interface
│   │   └── telegram.go         # Telegram sender
│   └── config/                 # Configuration
│       ├── config.go           # Config struct + loader
│       └── setup.go            # Interactive wizard
├── config/
│   └── config.toml.example     # User configuration template
├── deploy/
│   ├── pidex.service           # systemd unit
│   ├── pidex-shutdown.service  # systemd shutdown unit
│   └── install.sh              # Installation script
├── docs/
│   ├── architecture.md         # This file
│   ├── decisions.md            # Decision records
│   └── changelog.md            # Development log
├── go.mod
├── go.sum
└── README.md
```
