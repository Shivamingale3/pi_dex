import argparse
import logging
import signal
import sys
import threading
import time

from pidex.cli_setup import cmd_setup
from pidex.config.loader import load_config
from pidex.core.bus import EventBus
from pidex.core.constants import EVENT_DAEMON_START, EVENT_SHUTDOWN_STARTED, SEVERITY_INFO, SEVERITY_WARN, SOURCE_DAEMON, SOURCE_SHUTDOWN, VERSION
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


def _cmd_run(cfg) -> None:
    if not cfg.telegram_token or not cfg.telegram_chat_id:
        logger.error("Telegram bot_token and chat_id required (config or --flags)")
        sys.exit(1)

    bus = EventBus()
    notifier = TelegramNotifier(bot_token=cfg.telegram_token, chat_id=cfg.telegram_chat_id)
    cooldowns = CooldownManager(overrides=cfg.cooldown_overrides)
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

    if cfg.monitor_ssh or cfg.monitor_sudo or cfg.monitor_systemd:
        journal = JournalSource(bus, cfg)
        from pidex.sources import ssh as ssh_parser
        from pidex.sources import sudo as sudo_parser
        from pidex.sources.systemd import make_parser as make_systemd_parser

        if cfg.monitor_ssh:
            journal.register(ssh_parser.parse)
        if cfg.monitor_sudo:
            journal.register(sudo_parser.parse)
        if cfg.monitor_systemd:
            journal.register(make_systemd_parser(cfg.service_watch or []))

        journal.start(stop_event)
        sources.append(journal)

    if cfg.monitor_docker:
        docker = DockerSource(bus, cfg)
        docker.start(stop_event)
        sources.append(docker)

    if cfg.monitor_network:
        network = NetworkSource(bus, cfg)
        network.start(stop_event)
        sources.append(network)

    from pidex.pollers.cpu import CpuPoller
    from pidex.pollers.ram import RamPoller
    from pidex.pollers.disk import DiskPoller
    from pidex.pollers.temperature import TemperaturePoller

    pollers = []

    if cfg.monitor_cpu:
        p = CpuPoller(bus, cfg.cpu_interval, cfg.cpu_warn, cfg.cpu_critical)
        p.start(stop_event)
        pollers.append(p)

    if cfg.monitor_ram:
        p = RamPoller(bus, cfg.ram_interval, cfg.ram_warn, cfg.ram_critical)
        p.start(stop_event)
        pollers.append(p)

    if cfg.monitor_disk:
        p = DiskPoller(bus, cfg.disk_interval, cfg.disk_warn, cfg.disk_critical)
        p.start(stop_event)
        pollers.append(p)

    if cfg.monitor_temperature:
        p = TemperaturePoller(bus, cfg.temp_interval, cfg.temp_warn, cfg.temp_critical)
        p.start(stop_event)
        pollers.append(p)

    bus.publish(Event(
        source=SOURCE_DAEMON,
        event_type=EVENT_DAEMON_START,
        severity=SEVERITY_INFO,
        title="PiDex Started",
        message=f"PiDex v{VERSION} started — {len(sources)} event source(s), {len(pollers)} poller(s)",
    ))
    logger.info("PiDex v%s started — %d event source(s), %d poller(s)", VERSION, len(sources), len(pollers))
    shutdown_requested.wait()

    bus.publish(Event(
        source=SOURCE_SHUTDOWN,
        event_type=EVENT_SHUTDOWN_STARTED,
        severity=SEVERITY_WARN,
        title="Shutdown Initiated",
        message="PiDex daemon is shutting down",
    ))

    for _ in range(50):
        if bus.qsize == 0:
            break
        time.sleep(0.1)
    stop_event.set()
    logger.info("PiDex stopped")


def cmd_run(args: argparse.Namespace, cfg) -> None:
    if args.bot_token:
        cfg.telegram_token = args.bot_token
    if args.chat_id:
        cfg.telegram_chat_id = args.chat_id
    if not cfg.telegram_token or not cfg.telegram_chat_id:
        if sys.stdin.isatty() and sys.stdout.isatty() and os.geteuid() == 0:
            print("No Telegram credentials found. Launching setup wizard...")
            cmd_setup(args, cfg)
            cfg = load_config(path=args.config)
    _cmd_run(cfg)


def cmd_test(args: argparse.Namespace, cfg) -> None:
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

    token = args.bot_token or cfg.telegram_token
    chat_id = args.chat_id or cfg.telegram_chat_id
    if not token or not chat_id:
        logger.error("--bot-token and --chat-id required (or use --dry-run)")
        sys.exit(1)

    notifier = TelegramNotifier(bot_token=token, chat_id=chat_id)
    notifier.send(event)
    print("Sent.")


def cmd_version(args: argparse.Namespace, cfg) -> None:
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

    setup_parser = sub.add_parser("setup", help="Interactive configuration wizard")
    setup_parser.add_argument("--config", help="Path to config.toml")
    setup_parser.set_defaults(func=cmd_setup)

    version_parser = sub.add_parser("version", help="Print version")
    version_parser.set_defaults(func=cmd_version)

    args = parser.parse_args()
    cfg = load_config(path=args.config)
    args.func(args, cfg)
