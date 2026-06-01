import argparse
import logging
import signal
import sys
import threading
import time

from pidex.config.loader import apply_config, get_cooldown_overrides, get_telegram_config, load_config
from pidex.core.bus import EventBus
from pidex.core.constants import EVENT_SHUTDOWN_STARTED, SEVERITY_WARN, SOURCE_SHUTDOWN, VERSION
from pidex.core.cooldowns import CooldownManager
from pidex.core.dispatcher import Dispatcher
from pidex.core.event import Event
from pidex.notifiers.telegram import TelegramNotifier
from pidex.sources.docker import DockerSource
from pidex.sources.journal import JournalSource
from pidex.sources.network import NetworkSource

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    stream=sys.stdout,
)
logger = logging.getLogger("pidex")


def _resolve_telegram(args: argparse.Namespace, cfg: dict) -> tuple[str, str]:
    cfg_token, cfg_chat = get_telegram_config(cfg)
    token = args.bot_token or cfg_token
    chat_id = args.chat_id or cfg_chat
    return token, chat_id


def cmd_run(args: argparse.Namespace, cfg: dict) -> None:
    apply_config(cfg)

    token, chat_id = _resolve_telegram(args, cfg)
    if not token or not chat_id:
        logger.error("Telegram bot_token and chat_id required (config or --flags)")
        sys.exit(1)

    bus = EventBus()
    notifier = TelegramNotifier(bot_token=token, chat_id=chat_id)
    cooldowns = CooldownManager(overrides=get_cooldown_overrides(cfg))
    dispatcher = Dispatcher(bus=bus, notifier=notifier, cooldowns=cooldowns)

    stop_event = threading.Event()
    shutdown_requested = threading.Event()

    def handle_signal(signum, frame):
        logger.info("Shutdown requested (signal %s)...", signum)
        shutdown_requested.set()

    signal.signal(signal.SIGTERM, handle_signal)
    signal.signal(signal.SIGINT, handle_signal)

    dispatcher.start(stop_event)

    sources = []
    monitor = cfg.get("monitor", {})

    if monitor.get("ssh", True) or monitor.get("sudo", True) or monitor.get("systemd", True):
        journal = JournalSource(bus, cfg)
        from pidex.sources import ssh as ssh_parser
        from pidex.sources import sudo as sudo_parser
        from pidex.sources.systemd import make_parser as make_systemd_parser

        if monitor.get("ssh", True):
            journal.register(ssh_parser.parse)
        if monitor.get("sudo", True):
            journal.register(sudo_parser.parse)
        if monitor.get("systemd", True):
            watch = cfg.get("services", {}).get("watch", [])
            journal.register(make_systemd_parser(watch))

        journal.start(stop_event)
        sources.append(journal)

    if monitor.get("docker", True):
        docker = DockerSource(bus, cfg)
        docker.start(stop_event)
        sources.append(docker)

    if monitor.get("network", True):
        network = NetworkSource(bus, cfg)
        network.start(stop_event)
        sources.append(network)

    from pidex.core.constants import (
        DEFAULT_CPU_INTERVAL,
        DEFAULT_CPU_WARN,
        DEFAULT_CPU_CRITICAL,
        DEFAULT_RAM_INTERVAL,
        DEFAULT_RAM_WARN,
        DEFAULT_RAM_CRITICAL,
        DEFAULT_DISK_INTERVAL,
        DEFAULT_DISK_WARN,
        DEFAULT_DISK_CRITICAL,
        DEFAULT_TEMP_INTERVAL,
        DEFAULT_TEMP_WARN,
        DEFAULT_TEMP_CRITICAL,
    )
    from pidex.pollers.cpu import CpuPoller
    from pidex.pollers.ram import RamPoller
    from pidex.pollers.disk import DiskPoller
    from pidex.pollers.temperature import TemperaturePoller

    pollers = []

    if monitor.get("cpu", True):
        p = CpuPoller(bus, DEFAULT_CPU_INTERVAL, DEFAULT_CPU_WARN, DEFAULT_CPU_CRITICAL)
        p.start(stop_event)
        pollers.append(p)

    if monitor.get("ram", True):
        p = RamPoller(bus, DEFAULT_RAM_INTERVAL, DEFAULT_RAM_WARN, DEFAULT_RAM_CRITICAL)
        p.start(stop_event)
        pollers.append(p)

    if monitor.get("disk", True):
        p = DiskPoller(bus, DEFAULT_DISK_INTERVAL, DEFAULT_DISK_WARN, DEFAULT_DISK_CRITICAL)
        p.start(stop_event)
        pollers.append(p)

    if monitor.get("temperature", True):
        p = TemperaturePoller(bus, DEFAULT_TEMP_INTERVAL, DEFAULT_TEMP_WARN, DEFAULT_TEMP_CRITICAL)
        p.start(stop_event)
        pollers.append(p)

    logger.info("PiDex v%s started — %d event source(s), %d poller(s)", VERSION, len(sources), len(pollers))
    shutdown_requested.wait()

    bus.publish(Event(
        source=SOURCE_SHUTDOWN,
        event_type=EVENT_SHUTDOWN_STARTED,
        severity=SEVERITY_WARN,
        title="Shutdown Initiated",
        message="PiDex daemon is shutting down",
    ))

    time.sleep(0.5)
    stop_event.set()
    logger.info("PiDex stopped")


def cmd_test(args: argparse.Namespace, cfg: dict) -> None:
    events = {
        "ssh-login": Event(
            source="ssh",
            event_type="SSH_LOGIN",
            severity="INFO",
            title="SSH Login",
            message="shiv logged in from 192.168.1.100",
        ),
        "ssh-fail": Event(
            source="ssh",
            event_type="SSH_BRUTEFORCE",
            severity="WARN",
            title="SSH Brute Force",
            message="5 failed attempts from 10.0.0.50 in 30 seconds",
        ),
        "sudo-used": Event(
            source="sudo",
            event_type="SUDO_USED",
            severity="INFO",
            title="Sudo Used",
            message="shiv ran sudo apt update",
        ),
        "docker-down": Event(
            source="docker",
            event_type="CONTAINER_DIED",
            severity="CRITICAL",
            title="Container Died",
            message="nginx container 'web-prod' exited with code 1",
        ),
        "reboot": Event(
            source="shutdown",
            event_type="REBOOT_STARTED",
            severity="WARN",
            title="Reboot Initiated",
            message="System rebooting by shiv",
        ),
    }

    event = events.get(args.event_type)
    if event is None:
        logger.error("Unknown event type: %s", args.event_type)
        logger.info("Available: %s", ", ".join(sorted(events.keys())))
        sys.exit(1)

    if args.dry_run:
        print(f"[DRY RUN] Would send: {event.title}")
        print(f"  Source: {event.source}")
        print(f"  Type: {event.event_type}")
        print(f"  Severity: {event.severity}")
        print(f"  Message: {event.message}")
        return

    token, chat_id = _resolve_telegram(args, cfg)
    if not token or not chat_id:
        logger.error("--bot-token and --chat-id required (or use --dry-run)")
        sys.exit(1)

    notifier = TelegramNotifier(bot_token=token, chat_id=chat_id)
    notifier.send(event)
    print("Sent.")


def cmd_version(args: argparse.Namespace, cfg: dict) -> None:
    print(f"PiDex v{VERSION}")


def main() -> None:
    parser = argparse.ArgumentParser(prog="pidex", description="PiDex - Home Server Watchman")
    parser.add_argument("--bot-token", help="Telegram bot token (overrides config)")
    parser.add_argument("--chat-id", help="Telegram chat ID (overrides config)")
    parser.add_argument("--config", help="Path to config.toml")

    sub = parser.add_subparsers(dest="command", required=True)

    run_parser = sub.add_parser("run", help="Start the daemon")
    run_parser.set_defaults(func=cmd_run)

    test_parser = sub.add_parser("test", help="Send a test event")
    test_parser.add_argument("event_type", help="Event type (ssh-login, ssh-fail, sudo-used, docker-down, reboot)")
    test_parser.add_argument("--dry-run", action="store_true", help="Print the event without sending")
    test_parser.set_defaults(func=cmd_test)

    version_parser = sub.add_parser("version", help="Print version")
    version_parser.set_defaults(func=cmd_version)

    args = parser.parse_args()
    cfg = load_config(path=args.config)
    args.func(args, cfg)
