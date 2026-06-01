from collections import OrderedDict, deque
import re
import time
from datetime import datetime

from pidex.core.constants import EVENT_SSH_BRUTEFORCE, EVENT_SSH_LOGIN, EVENT_SSH_LOGOUT, SEVERITY_INFO, SEVERITY_WARN, SOURCE_SSH
from pidex.core.event import Event

_ACCEPTED_RE = re.compile(
    r"Accepted (?:publickey|password|keyboard-interactive) for (\S+) from (\S+)"
)
_FAILED_RE = re.compile(
    r"Failed (?:password|publickey) for (\S+) from (\S+)"
)
_DISCONNECTED_RE = re.compile(
    r"Disconnected from (?:user\s+)?(\S+) (\S+)"
)

_BRUTEFORCE_WINDOW = 30
_BRUTEFORCE_THRESHOLD = 3


def parse(entry: dict) -> Event | None:
    if entry.get("_COMM") != "sshd" and entry.get("SYSLOG_IDENTIFIER") != "sshd":
        return None

    message = entry.get("MESSAGE", "")

    m = _ACCEPTED_RE.search(message)
    if m:
        user, ip = m.group(1), m.group(2)
        return Event(
            source=SOURCE_SSH,
            event_type=EVENT_SSH_LOGIN,
            severity=SEVERITY_INFO,
            title="SSH Login",
            message=f"{user} logged in from {ip}",
            timestamp=_ts(entry),
        )

    m = _DISCONNECTED_RE.search(message)
    if m:
        user, ip = m.group(1), m.group(2)
        return Event(
            source=SOURCE_SSH,
            event_type=EVENT_SSH_LOGOUT,
            severity=SEVERITY_INFO,
            title="SSH Logout",
            message=f"{user} disconnected from {ip}",
            timestamp=_ts(entry),
        )

    m = _FAILED_RE.search(message)
    if m:
        user, ip = m.group(1), m.group(2)
        if _is_bruteforce(ip, entry):
            return Event(
                source=SOURCE_SSH,
                event_type=EVENT_SSH_BRUTEFORCE,
                severity=SEVERITY_WARN,
                title="SSH Brute Force",
                message=f"Repeated failed attempts for {user} from {ip}",
                timestamp=_ts(entry),
            )

    return None


from collections import deque

_BRUTEFORCE_MAX_IPS = 1000
_bruteforce_tracker: OrderedDict[str, deque[float]] = OrderedDict()


def _is_bruteforce(ip: str, entry: dict) -> bool:
    now = time.time()
    if ip not in _bruteforce_tracker:
        if len(_bruteforce_tracker) >= _BRUTEFORCE_MAX_IPS:
            _bruteforce_tracker.popitem(last=False)
        _bruteforce_tracker[ip] = deque()
    _bruteforce_tracker.move_to_end(ip)
    attempts = _bruteforce_tracker[ip]
    attempts.append(now)
    cutoff = now - _BRUTEFORCE_WINDOW
    while attempts and attempts[0] < cutoff:
        attempts.popleft()
    return len(attempts) >= _BRUTEFORCE_THRESHOLD


def _ts(entry: dict) -> datetime:
    micros = entry.get("__REALTIME_TIMESTAMP")
    if micros is not None:
        return datetime.fromtimestamp(int(micros) / 1_000_000)
    return datetime.now()
