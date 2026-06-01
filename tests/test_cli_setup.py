import argparse
import os
import tempfile

import pytest

from pidex.config import Config
from pidex.cli_setup import _resolve_paths, _toml, _write_config, _write_env


def test_toml_bool():
    assert _toml(True) == "true"
    assert _toml(False) == "false"


def test_toml_str():
    assert _toml("hello") == '"hello"'


def test_toml_list():
    assert _toml(["a", "b"]) == '["a", "b"]'
    assert _toml([1, 2]) == "[1, 2]"


def test_toml_int():
    assert _toml(42) == "42"


def test_toml_float():
    assert _toml(3.14) == "3.14"


def test_resolve_paths_root():
    args = argparse.Namespace(config=None)
    orig_euid = os.geteuid
    os.geteuid = lambda: 0
    try:
        config_path, env_path = _resolve_paths(args)
        assert config_path == "/etc/pidex/config.toml"
        assert env_path == "/etc/pidex/env"
    finally:
        os.geteuid = orig_euid


def test_resolve_paths_user():
    args = argparse.Namespace(config=None)
    orig_euid = os.geteuid
    os.geteuid = lambda: 1000
    try:
        config_path, env_path = _resolve_paths(args)
        assert "config/pidex/config.toml" in config_path
        assert "config/pidex/env" in env_path
    finally:
        os.geteuid = orig_euid


def test_resolve_paths_custom():
    args = argparse.Namespace(config="/tmp/custom.toml")
    config_path, env_path = _resolve_paths(args)
    assert config_path == "/tmp/custom.toml"
    assert env_path == "/tmp/env"


def test_write_and_read_env():
    with tempfile.TemporaryDirectory() as tmpdir:
        path = os.path.join(tmpdir, "env")
        _write_env(path, "token123", "chat456")
        assert os.path.isfile(path)
        mode = os.stat(path).st_mode
        assert mode & 0o777 == 0o600

        with open(path) as f:
            content = f.read()
        assert "TELEGRAM_BOT_TOKEN=token123" in content
        assert "TELEGRAM_CHAT_ID=chat456" in content


def test_write_config_creates_file():
    cfg = Config()
    with tempfile.TemporaryDirectory() as tmpdir:
        path = os.path.join(tmpdir, "config.toml")
        _write_config(path, cfg)
        assert os.path.isfile(path)
        mode = os.stat(path).st_mode
        assert mode & 0o777 == 0o640

        with open(path) as f:
            content = f.read()
        assert "[monitor]" in content
        assert "[pollers]" in content
        assert "[thresholds]" in content
        assert "[cooldowns]" in content


def test_write_config_roundtrip():
    cfg = Config(
        cpu_interval=60,
        cpu_warn=90.0,
        monitor_docker=False,
        service_watch=["nginx", "docker"],
    )
    with tempfile.TemporaryDirectory() as tmpdir:
        path = os.path.join(tmpdir, "config.toml")
        _write_config(path, cfg)
        from pidex.config.loader import _read_config
        loaded = _read_config(path)
        assert loaded.cpu_interval == 60
        assert loaded.cpu_warn == 90.0
        assert loaded.monitor_docker is False
        assert loaded.service_watch == ["nginx", "docker"]
