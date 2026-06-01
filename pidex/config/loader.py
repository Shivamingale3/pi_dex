import os
import tomllib

from dotenv import load_dotenv

from pidex.config import Config

load_dotenv()

DEFAULT_CONFIG_PATHS = [
    "./config/config.toml",
    "/etc/pidex/config.toml",
    os.path.expanduser("~/.config/pidex/config.toml"),
]


def load_config(path: str | None = None) -> Config:
    if path is not None:
        return _read_config(path)

    for candidate in DEFAULT_CONFIG_PATHS:
        expanded = os.path.expanduser(candidate)
        if os.path.isfile(expanded):
            return _read_config(expanded)

    return Config.from_dict({})


def _read_config(path: str) -> Config:
    with open(path, "rb") as f:
        data = tomllib.load(f)
    return Config.from_dict(data)
