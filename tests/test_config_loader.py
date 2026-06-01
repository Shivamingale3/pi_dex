import os
import tempfile

import pytest

from pidex.config.loader import apply_config, get_cooldown_overrides, get_telegram_config, load_config


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


def test_load_config_with_path(temp_config):
    cfg = load_config(path=temp_config)
    assert cfg["telegram"]["bot_token"] == "cfg_token"
    assert cfg["pollers"]["cpu_interval"] == 30
    assert cfg["thresholds"]["cpu_warn"] == 90


def test_load_config_nonexistent_path():
    import pytest
    with pytest.raises(FileNotFoundError):
        load_config(path="/nonexistent/path.toml")


def test_get_telegram_config(temp_config, monkeypatch):
    monkeypatch.delenv("TELEGRAM_BOT_TOKEN", raising=False)
    monkeypatch.delenv("TELEGRAM_CHAT_ID", raising=False)
    cfg = load_config(path=temp_config)
    token, chat = get_telegram_config(cfg)
    assert token == "cfg_token"
    assert chat == "cfg_chat"


def test_get_telegram_config_empty(monkeypatch):
    monkeypatch.delenv("TELEGRAM_BOT_TOKEN", raising=False)
    monkeypatch.delenv("TELEGRAM_CHAT_ID", raising=False)
    token, chat = get_telegram_config({})
    assert token == ""
    assert chat == ""


def test_get_cooldown_overrides(temp_config):
    cfg = load_config(path=temp_config)
    overrides = get_cooldown_overrides(cfg)
    assert overrides["SSH_LOGIN"] == 60


def test_get_cooldown_overrides_empty():
    assert get_cooldown_overrides({}) == {}


def test_apply_config(temp_config):
    import pidex.core.constants as C

    cfg = load_config(path=temp_config)
    original_interval = C.DEFAULT_CPU_INTERVAL
    original_warn = C.DEFAULT_CPU_WARN

    try:
        apply_config(cfg)
        assert C.DEFAULT_CPU_INTERVAL == 30
        assert C.DEFAULT_CPU_WARN == 90
    finally:
        C.DEFAULT_CPU_INTERVAL = original_interval
        C.DEFAULT_CPU_WARN = original_warn
