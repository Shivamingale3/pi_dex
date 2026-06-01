import os
import tomllib

from dotenv import load_dotenv

load_dotenv()

DEFAULT_CONFIG_PATHS = [
    "./config/config.toml",
    "/etc/pidex/config.toml",
    os.path.expanduser("~/.config/pidex/config.toml"),
]


def load_config(path: str | None = None) -> dict:
    if path is not None:
        return _read_file(path)

    for candidate in DEFAULT_CONFIG_PATHS:
        expanded = os.path.expanduser(candidate)
        if os.path.isfile(expanded):
            return _read_file(expanded)

    return {}


def _read_file(path: str) -> dict:
    with open(path, "rb") as f:
        return tomllib.load(f)


_VALID_POLLER_KEYS = {"cpu_interval", "ram_interval", "temp_interval", "disk_interval"}
_VALID_THRESHOLD_KEYS = {
    "cpu_warn", "cpu_critical", "ram_warn", "ram_critical",
    "disk_warn", "disk_critical", "temp_warn", "temp_critical",
}


def _validate_config(cfg: dict) -> None:
    pollers = cfg.get("pollers", {})
    for k, v in pollers.items():
        if k in _VALID_POLLER_KEYS and not isinstance(v, (int, float)):
            raise ValueError(f"poller.{k} must be a number, got {type(v).__name__}")

    thresholds = cfg.get("thresholds", {})
    for k, v in thresholds.items():
        if k in _VALID_THRESHOLD_KEYS and not isinstance(v, (int, float)):
            raise ValueError(f"thresholds.{k} must be a number, got {type(v).__name__}")

    services = cfg.get("services", {})
    watch = services.get("watch", [])
    if not isinstance(watch, list):
        raise ValueError("services.watch must be a list")

    containers = cfg.get("containers", {})
    cwatch = containers.get("watch", [])
    if not isinstance(cwatch, list):
        raise ValueError("containers.watch must be a list")


def apply_config(cfg: dict) -> None:
    import pidex.core.constants as C

    _validate_config(cfg)

    pollers = cfg.get("pollers", {})
    C.DEFAULT_CPU_INTERVAL = pollers.get("cpu_interval", C.DEFAULT_CPU_INTERVAL)
    C.DEFAULT_RAM_INTERVAL = pollers.get("ram_interval", C.DEFAULT_RAM_INTERVAL)
    C.DEFAULT_TEMP_INTERVAL = pollers.get("temp_interval", C.DEFAULT_TEMP_INTERVAL)
    C.DEFAULT_DISK_INTERVAL = pollers.get("disk_interval", C.DEFAULT_DISK_INTERVAL)

    thresholds = cfg.get("thresholds", {})
    C.DEFAULT_CPU_WARN = thresholds.get("cpu_warn", C.DEFAULT_CPU_WARN)
    C.DEFAULT_CPU_CRITICAL = thresholds.get("cpu_critical", C.DEFAULT_CPU_CRITICAL)
    C.DEFAULT_RAM_WARN = thresholds.get("ram_warn", C.DEFAULT_RAM_WARN)
    C.DEFAULT_RAM_CRITICAL = thresholds.get("ram_critical", C.DEFAULT_RAM_CRITICAL)
    C.DEFAULT_DISK_WARN = thresholds.get("disk_warn", C.DEFAULT_DISK_WARN)
    C.DEFAULT_DISK_CRITICAL = thresholds.get("disk_critical", C.DEFAULT_DISK_CRITICAL)
    C.DEFAULT_TEMP_WARN = thresholds.get("temp_warn", C.DEFAULT_TEMP_WARN)
    C.DEFAULT_TEMP_CRITICAL = thresholds.get("temp_critical", C.DEFAULT_TEMP_CRITICAL)


def get_cooldown_overrides(cfg: dict) -> dict[str, float]:
    return cfg.get("cooldowns", {})


def get_telegram_config(cfg: dict) -> tuple[str, str]:
    token = os.environ.get("TELEGRAM_BOT_TOKEN")
    chat_id = os.environ.get("TELEGRAM_CHAT_ID")
    if token and chat_id:
        return token, chat_id
    tg = cfg.get("telegram", {})
    return tg.get("bot_token", "") or "", tg.get("chat_id", "") or ""
