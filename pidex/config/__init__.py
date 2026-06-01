from __future__ import annotations

import os
from dataclasses import dataclass, field
from datetime import timedelta


@dataclass
class Config:
    telegram_token: str = ""
    telegram_chat_id: str = ""

    cpu_interval: int = 15
    ram_interval: int = 30
    temp_interval: int = 30
    disk_interval: int = 300

    cpu_warn: float = 80.0
    cpu_critical: float = 95.0
    ram_warn: float = 85.0
    ram_critical: float = 95.0
    disk_warn: float = 85.0
    disk_critical: float = 95.0
    temp_warn: float = 75.0
    temp_critical: float = 85.0

    service_watch: list[str] | None = None
    container_watch: list[str] | None = None

    monitor_ssh: bool = True
    monitor_sudo: bool = True
    monitor_systemd: bool = True
    monitor_docker: bool = True
    monitor_network: bool = True
    monitor_cpu: bool = True
    monitor_ram: bool = True
    monitor_disk: bool = True
    monitor_temperature: bool = True

    cooldown_overrides: dict[str, float] | None = None

    @classmethod
    def from_dict(cls, data: dict) -> Config:
        pollers = data.get("pollers", {})
        thresholds = data.get("thresholds", {})
        services = data.get("services", {})
        containers = data.get("containers", {})
        monitor = data.get("monitor", {})
        telegram = data.get("telegram", {})

        token = os.environ.get("TELEGRAM_BOT_TOKEN") or telegram.get("bot_token", "") or ""
        chat_id = os.environ.get("TELEGRAM_CHAT_ID") or telegram.get("chat_id", "") or ""

        return cls(
            telegram_token=str(token),
            telegram_chat_id=str(chat_id),
            cpu_interval=int(pollers.get("cpu_interval", 15)),
            ram_interval=int(pollers.get("ram_interval", 30)),
            temp_interval=int(pollers.get("temp_interval", 30)),
            disk_interval=int(pollers.get("disk_interval", 300)),
            cpu_warn=float(thresholds.get("cpu_warn", 80)),
            cpu_critical=float(thresholds.get("cpu_critical", 95)),
            ram_warn=float(thresholds.get("ram_warn", 85)),
            ram_critical=float(thresholds.get("ram_critical", 95)),
            disk_warn=float(thresholds.get("disk_warn", 85)),
            disk_critical=float(thresholds.get("disk_critical", 95)),
            temp_warn=float(thresholds.get("temp_warn", 75)),
            temp_critical=float(thresholds.get("temp_critical", 85)),
            service_watch=services.get("watch") or None,
            container_watch=containers.get("watch") or None,
            monitor_ssh=monitor.get("ssh", True),
            monitor_sudo=monitor.get("sudo", True),
            monitor_systemd=monitor.get("systemd", True),
            monitor_docker=monitor.get("docker", True),
            monitor_network=monitor.get("network", True),
            monitor_cpu=monitor.get("cpu", True),
            monitor_ram=monitor.get("ram", True),
            monitor_disk=monitor.get("disk", True),
            monitor_temperature=monitor.get("temperature", True),
            cooldown_overrides=data.get("cooldowns") or None,
        )
