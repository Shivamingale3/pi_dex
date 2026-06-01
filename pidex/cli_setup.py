import json
import os
import sys

from pidex.config import Config
from pidex.config.loader import load_config
from pidex.notifiers.telegram import TelegramNotifier

_DEFAULT_COOLDOWNS = {
    "SSH_LOGIN": 0, "SSH_LOGOUT": 0, "SSH_BRUTEFORCE": 300,
    "SUDO_USED": 0, "CPU_HIGH": 300, "CPU_RECOVERED": 300,
    "TEMP_WARN": 300, "TEMP_CRITICAL": 300,
    "DISK_WARN": 3600, "DISK_CRITICAL": 3600, "RAM_HIGH": 300,
}


def _warn(msg: str) -> None:
    print(f"\033[33m{msg}\033[0m")


def _ok(msg: str) -> None:
    print(f"\033[32m{msg}\033[0m")


def _toml(v) -> str:
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, str):
        return json.dumps(v)
    if isinstance(v, list):
        return "[" + ", ".join(_toml(i) for i in v) + "]"
    if isinstance(v, dict):
        items = ", ".join(f"{k} = {_toml(val)}" for k, val in v.items())
        return "{" + items + "}"
    return str(v)


def _write_config(path: str, cfg: Config) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    lines = [
        "[monitor]",
        f"ssh = {_toml(cfg.monitor_ssh)}",
        f"sudo = {_toml(cfg.monitor_sudo)}",
        f"docker = {_toml(cfg.monitor_docker)}",
        f"systemd = {_toml(cfg.monitor_systemd)}",
        f"network = {_toml(cfg.monitor_network)}",
        f"cpu = {_toml(cfg.monitor_cpu)}",
        f"ram = {_toml(cfg.monitor_ram)}",
        f"disk = {_toml(cfg.monitor_disk)}",
        f"temperature = {_toml(cfg.monitor_temperature)}",
        "",
        "[services]",
        f"watch = {_toml(cfg.service_watch or [])}",
        "",
        "[containers]",
        f"watch = {_toml(cfg.container_watch or [])}",
        "",
        "[pollers]",
        f"cpu_interval = {cfg.cpu_interval}",
        f"ram_interval = {cfg.ram_interval}",
        f"temp_interval = {cfg.temp_interval}",
        f"disk_interval = {cfg.disk_interval}",
        "",
        "[thresholds]",
        f"cpu_warn = {cfg.cpu_warn}",
        f"cpu_critical = {cfg.cpu_critical}",
        f"ram_warn = {cfg.ram_warn}",
        f"ram_critical = {cfg.ram_critical}",
        f"disk_warn = {cfg.disk_warn}",
        f"disk_critical = {cfg.disk_critical}",
        f"temp_warn = {cfg.temp_warn}",
        f"temp_critical = {cfg.temp_critical}",
        "",
        "[cooldowns]",
    ]
    overrides = cfg.cooldown_overrides or {}
    if overrides:
        for k, v in overrides.items():
            lines.append(f"{k} = {int(v) if v == int(v) else v}")
    else:
        lines.append("# Use defaults")
    lines.append("")

    with open(path, "w") as f:
        f.write("\n".join(lines))
    os.chmod(path, 0o640)
    _ok(f"Wrote {path}")


def _write_env(path: str, token: str, chat_id: str) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        f.write(f"TELEGRAM_BOT_TOKEN={token}\n")
        f.write(f"TELEGRAM_CHAT_ID={chat_id}\n")
    os.chmod(path, 0o600)
    _ok(f"Wrote {path} (mode 600)")


def _prompt(label: str, current, parser=str) -> tuple[bool, any]:
    """Prompt user, return (changed, new_value)."""
    val = input(f"  {label} [{current}]: ").strip()
    if not val:
        return False, current
    try:
        return True, parser(val)
    except (ValueError, TypeError):
        _warn(f"Invalid input, keeping [{current}]")
        return False, current


def _resolve_paths(args) -> tuple[str, str]:
    """Return (config_path, env_path)."""
    if args.config:
        config_path = args.config
        env_path = os.path.join(os.path.dirname(config_path), "env")
    elif os.geteuid() == 0:
        config_path = "/etc/pidex/config.toml"
        env_path = "/etc/pidex/env"
    else:
        config_path = os.path.expanduser("~/.config/pidex/config.toml")
        env_path = os.path.expanduser("~/.config/pidex/env")
    return config_path, env_path


def _read_env(path: str) -> tuple[str, str]:
    """Read token and chat_id from env file."""
    token, chat_id = "", ""
    if os.path.isfile(path):
        for line in open(path):
            line = line.strip()
            if line.startswith("TELEGRAM_BOT_TOKEN="):
                token = line.split("=", 1)[1]
            elif line.startswith("TELEGRAM_CHAT_ID="):
                chat_id = line.split("=", 1)[1]
    return token, chat_id


def _require_tty() -> None:
    if not sys.stdin.isatty() or not sys.stdout.isatty():
        print("pidex setup requires an interactive terminal.", file=sys.stderr)
        sys.exit(1)


def _view_config(cfg: Config, env_path: str) -> None:
    token, chat_id = _read_env(env_path)
    print("\nCredentials:")
    print(f"  bot_token: {'***' if cfg.telegram_token else '(not set)'}")
    print(f"  chat_id:   {'***' if cfg.telegram_chat_id else '(not set)'}")
    print(f"  source:    env file ({env_path})" if token else "  source:    config.toml")

    print(f"\nMonitor:")
    for key in ["ssh", "sudo", "docker", "systemd", "network", "cpu", "ram", "disk", "temperature"]:
        val = getattr(cfg, f"monitor_{key}")
        print(f"  {key}: {'on' if val else 'off'}")

    print(f"\nPoller intervals (seconds):")
    for key, label in [("cpu", "CPU"), ("ram", "RAM"), ("temp", "Temperature"), ("disk", "Disk")]:
        print(f"  {label}: {getattr(cfg, f'{key.lower()}_interval')}")

    print(f"\nThresholds (%):")
    for key, label in [("cpu", "CPU"), ("ram", "RAM"), ("disk", "Disk"), ("temp", "Temp")]:
        print(f"  {label} warn: {getattr(cfg, f'{key}_warn')}  crit: {getattr(cfg, f'{key}_critical')}")

    print(f"\nWatch lists:")
    print(f"  services:   {cfg.service_watch or '(all)'}")
    print(f"  containers: {cfg.container_watch or '(all)'}")

    print(f"\nCooldown overrides (seconds):")
    overrides = cfg.cooldown_overrides or {}
    if overrides:
        for k, v in overrides.items():
            print(f"  {k}: {v}")
    else:
        print("  (defaults)")


def _set_credentials(cfg: Config, env_path: str) -> Config:
    cur_token, cur_chat = _read_env(env_path)
    token = cur_token or cfg.telegram_token
    chat_id = cur_chat or cfg.telegram_chat_id

    print("\nTelegram credentials (leave blank to keep current):")
    changed_t, token = _prompt("Bot token", "***" if token else "")
    changed_c, chat_id = _prompt("Chat ID", "***" if chat_id else "")

    if changed_t or changed_c:
        if not token or not chat_id:
            _warn("Both token and chat_id are required.")
            return cfg
        if ":" not in token:
            _warn("Invalid bot token format (expected digits:hex).")
            return cfg
        if not chat_id.lstrip("-").isdigit():
            _warn("Chat ID must be numeric.")
            return cfg
        _write_env(env_path, token, chat_id)
        cfg.telegram_token = token
        cfg.telegram_chat_id = chat_id
        _ok("Credentials saved.")
    return cfg


def _set_monitor(cfg: Config, config_path: str) -> Config:
    print("\nMonitor toggles (y/n, blank to keep):")
    for key in ["ssh", "sudo", "docker", "systemd", "network", "cpu", "ram", "disk", "temperature"]:
        current = getattr(cfg, f"monitor_{key}")
        val = input(f"  Monitor {key}? [{'Y' if current else 'y'}/{'N' if not current else 'n'}]: ").strip().lower()
        if val == "y":
            setattr(cfg, f"monitor_{key}", True)
        elif val == "n":
            setattr(cfg, f"monitor_{key}", False)
    _write_config(config_path, cfg)
    return cfg


def _set_intervals(cfg: Config, config_path: str) -> Config:
    print("\nPoller intervals in seconds (blank to keep):")
    for attr, label in [("cpu_interval", "CPU"), ("ram_interval", "RAM"),
                         ("temp_interval", "Temperature"), ("disk_interval", "Disk")]:
        changed, val = _prompt(label, getattr(cfg, attr), int)
        if changed:
            setattr(cfg, attr, val)
    _write_config(config_path, cfg)
    return cfg


def _set_thresholds(cfg: Config, config_path: str) -> Config:
    print("\nThresholds in percent (blank to keep):")
    for prefix, label in [("cpu", "CPU"), ("ram", "RAM"), ("disk", "Disk"), ("temp", "Temp")]:
        for suffix in ("warn", "critical"):
            attr = f"{prefix}_{suffix}"
            changed, val = _prompt(f"{label} {suffix}", getattr(cfg, attr), float)
            if changed:
                setattr(cfg, attr, val)
    _write_config(config_path, cfg)
    return cfg


def _set_watch_lists(cfg: Config, config_path: str) -> Config:
    print("\nWatch lists (comma-separated glob patterns, blank to keep):")
    for attr, label in [("service_watch", "Services"), ("container_watch", "Containers")]:
        current = getattr(cfg, attr) or []
        val = input(f"  {label} [{','.join(current)}]: ").strip()
        if val:
            setattr(cfg, attr, [p.strip() for p in val.split(",") if p.strip()])
    _write_config(config_path, cfg)
    return cfg


def _set_cooldowns(cfg: Config, config_path: str) -> Config:
    print("\nCooldown overrides in seconds (blank to keep, 0 = no cooldown):")
    overrides = dict(cfg.cooldown_overrides or {})
    keys = list(_DEFAULT_COOLDOWNS.keys())
    for key in keys:
        current = int(overrides.get(key, _DEFAULT_COOLDOWNS[key]))
        val = input(f"  {key} [{current}]: ").strip()
        if val:
            parsed = int(val)
            if parsed == _DEFAULT_COOLDOWNS[key]:
                overrides.pop(key, None)
            else:
                overrides[key] = parsed
    cfg.cooldown_overrides = overrides if overrides else None
    _write_config(config_path, cfg)
    return cfg


def _send_test(cfg: Config) -> None:
    if not cfg.telegram_token or not cfg.telegram_chat_id:
        _warn("No Telegram credentials configured. Run option 1 first.")
        return
    notifier = TelegramNotifier(bot_token=cfg.telegram_token, chat_id=cfg.telegram_chat_id)
    from pidex.core.constants import SEVERITY_INFO
    from pidex.core.event import Event
    event = Event(
        source="daemon",
        event_type="TEST",
        severity=SEVERITY_INFO,
        title="Test Notification",
        message="This is a test message from PiDex setup wizard",
    )
    try:
        notifier.send(event)
        _ok("Test notification sent!")
    except Exception as e:
        _warn(f"Failed: {e}")


def _reset_defaults(cfg: Config, config_path: str, env_path: str) -> Config:
    print("\nReset to factory defaults? This will erase your configuration.")
    confirm = input("Type 'reset' to confirm: ").strip().lower()
    if confirm != "reset":
        _warn("Reset cancelled.")
        return cfg
    cfg = Config()
    _write_config(config_path, cfg)
    if os.path.isfile(env_path):
        os.remove(env_path)
        _ok(f"Removed {env_path}")
    _ok("Configuration reset to defaults.")
    return cfg


def cmd_setup(args, cfg) -> None:
    _require_tty()
    config_path, env_path = _resolve_paths(args)

    if not os.path.isfile(config_path):
        print(f"Config not found at {config_path}. Starting with defaults.")

    while True:
        print(f"\n\033[1mPiDex Setup Wizard\033[0m")
        print(f"  Config: {config_path}")
        token, _ = _read_env(env_path)
        print(f"  Credentials: {'set' if token or cfg.telegram_token else 'NOT SET'}")
        print()
        print("  1. View current config")
        print("  2. Set Telegram credentials")
        print("  3. Set monitor toggles")
        print("  4. Set poller intervals")
        print("  5. Set thresholds")
        print("  6. Set watch lists")
        print("  7. Set cooldowns")
        print("  8. Send test notification")
        print("  9. Reset to defaults")
        print("  0. Save & exit")
        raw = input("\nChoice [0-9]: ").strip()

        if raw == "0" or raw == "":
            print("Exiting. Configuration saved.")
            break
        elif raw == "1":
            _view_config(cfg, env_path)
        elif raw == "2":
            cfg = _set_credentials(cfg, env_path)
        elif raw == "3":
            cfg = _set_monitor(cfg, config_path)
        elif raw == "4":
            cfg = _set_intervals(cfg, config_path)
        elif raw == "5":
            cfg = _set_thresholds(cfg, config_path)
        elif raw == "6":
            cfg = _set_watch_lists(cfg, config_path)
        elif raw == "7":
            cfg = _set_cooldowns(cfg, config_path)
        elif raw == "8":
            _send_test(cfg)
        elif raw == "9":
            cfg = _reset_defaults(cfg, config_path, env_path)
        else:
            print("Enter 0-9")
