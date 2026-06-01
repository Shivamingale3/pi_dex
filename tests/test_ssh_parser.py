from pidex.sources.ssh import parse, _bruteforce_tracker


def make_entry(comm="sshd", identifier="sshd", message=""):
    return {
        "_COMM": comm,
        "SYSLOG_IDENTIFIER": identifier,
        "MESSAGE": message,
        "__REALTIME_TIMESTAMP": 1735689600000000,
    }


def test_ssh_login():
    entry = make_entry(
        message="Accepted password for leadows from 192.168.1.100 port 22 ssh2"
    )
    event = parse(entry)
    assert event is not None
    assert event.source == "ssh"
    assert event.event_type == "SSH_LOGIN"
    assert event.severity == "INFO"
    assert "leadows" in event.message
    assert "192.168.1.100" in event.message


def test_ssh_logout():
    entry = make_entry(
        message="Disconnected from user leadows 192.168.1.100 port 22"
    )
    event = parse(entry)
    assert event is not None
    assert event.event_type == "SSH_LOGOUT"


def test_ssh_failed_not_bruteforce():
    entry = make_entry(
        message="Failed password for root from 10.0.0.1 port 22 ssh2"
    )
    event = parse(entry)
    assert event is None


def test_ssh_bruteforce():
    entry = make_entry(
        message="Failed password for root from 10.0.0.50 port 22 ssh2"
    )
    _bruteforce_tracker.clear()
    for _ in range(3):
        parse(entry)
    event = parse(entry)
    assert event is not None
    assert event.event_type == "SSH_BRUTEFORCE"
    assert event.severity == "WARN"


def test_accepts_publickey():
    entry = make_entry(
        message="Accepted publickey for ubuntu from 10.0.0.1 port 22 ssh2: ED25519 SHA256:xxx"
    )
    event = parse(entry)
    assert event is not None
    assert event.event_type == "SSH_LOGIN"


def test_rejects_non_ssh():
    entry = make_entry(comm="sudo", identifier="sudo", message="anything")
    event = parse(entry)
    assert event is None


def test_rejects_unknown_message():
    entry = make_entry(message="Connection closed by authenticating user root")
    event = parse(entry)
    assert event is None
