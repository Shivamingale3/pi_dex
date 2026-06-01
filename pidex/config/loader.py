import os

import tomli

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
        return tomli.load(f)


def apply_config(cfg: dict) -> None:
    import pidex.core.constants as C

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
    tg = cfg.get("telegram", {})
    return tg.get("bot_token", ""), tg.get("chat_id", "")
