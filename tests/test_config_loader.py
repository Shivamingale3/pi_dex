import os
import tempfile

import pytest

from pidex.config.loader import load_config


@pytest.fixture
def temp_config():
    content = """
[telegram]
bot_token = "cfg_token"
chat_id = "cfg_chat"

[pollers]
cpu_interval = 30

[thresholds]
cpu_warn = 90

[cooldowns]
SSH_LOGIN = 60

[services]
watch = ["nginx"]

[containers]
watch = ["web"]
"""
    f = tempfile.NamedTemporaryFile(mode="w", delete=False, suffix=".toml")
    f.write(content)
    f.close()
    yield f.name
    os.unlink(f.name)


def test_load_config_with_path(temp_config, monkeypatch):
    monkeypatch.delenv("TELEGRAM_BOT_TOKEN", raising=False)
    monkeypatch.delenv("TELEGRAM_CHAT_ID", raising=False)
    cfg = load_config(path=temp_config)
    assert cfg.telegram_token == "cfg_token"
    assert cfg.telegram_chat_id == "cfg_chat"
    assert cfg.cpu_interval == 30
    assert cfg.cpu_warn == 90
    assert cfg.cooldown_overrides == {"SSH_LOGIN": 60}
    assert cfg.service_watch == ["nginx"]
    assert cfg.container_watch == ["web"]


def test_load_config_nonexistent_path():
    with pytest.raises(FileNotFoundError):
        load_config(path="/nonexistent/path.toml")


def test_telegram_env_overrides_config(temp_config, monkeypatch):
    monkeypatch.setenv("TELEGRAM_BOT_TOKEN", "env_token")
    monkeypatch.setenv("TELEGRAM_CHAT_ID", "env_chat")
    cfg = load_config(path=temp_config)
    assert cfg.telegram_token == "env_token"
    assert cfg.telegram_chat_id == "env_chat"


def test_load_config_empty(monkeypatch):
    monkeypatch.delenv("TELEGRAM_BOT_TOKEN", raising=False)
    monkeypatch.delenv("TELEGRAM_CHAT_ID", raising=False)
    cfg = load_config(path="/dev/null")
    assert cfg.telegram_token == ""
    assert cfg.cpu_interval == 15
    assert cfg.cpu_warn == 80.0
    assert cfg.cooldown_overrides is None
    assert cfg.service_watch is None


def test_load_config_defaults():
    cfg = load_config()
    assert cfg.cpu_interval == 15
    assert cfg.ram_interval == 30
    assert cfg.monitor_ssh is True
    assert cfg.monitor_docker is True


def test_config_from_dict():
    from pidex.config import Config
    cfg = Config.from_dict({
        "pollers": {"cpu_interval": 60},
        "thresholds": {"cpu_warn": 85},
        "cooldowns": {"SSH_LOGIN": 120},
        "services": {"watch": ["nginx", "docker"]},
        "containers": {"watch": ["web*"]},
        "monitor": {"docker": False},
    })
    assert cfg.cpu_interval == 60
    assert cfg.cpu_warn == 85.0
    assert cfg.cooldown_overrides == {"SSH_LOGIN": 120}
    assert cfg.service_watch == ["nginx", "docker"]
    assert cfg.container_watch == ["web*"]
    assert cfg.monitor_docker is False
