from pidex.sources.sudo import parse


def make_entry(comm="sudo", identifier="sudo", message=""):
    return {
        "_COMM": comm,
        "SYSLOG_IDENTIFIER": identifier,
        "MESSAGE": message,
        "__REALTIME_TIMESTAMP": 1735689600000000,
    }


def test_sudo_used():
    entry = make_entry(
        message="leadows ; user=root ; command=/usr/bin/apt update ; COMMAND=/usr/bin/apt update"
    )
    event = parse(entry)
    assert event is not None
    assert event.source == "sudo"
    assert event.event_type == "SUDO_USED"
    assert event.severity == "INFO"
    assert "leadows" in event.message
    assert "apt update" in event.message


def test_rejects_non_sudo():
    entry = make_entry(comm="sshd", identifier="sshd", message="anything")
    event = parse(entry)
    assert event is None


def test_rejects_short_message():
    entry = make_entry(message="too short")
    event = parse(entry)
    assert event is None


def test_handles_different_sudo_format():
    entry = make_entry(
        message="alice ; user=root ; command=/bin/ls ; COMMAND=/bin/ls -la"
    )
    event = parse(entry)
    assert event is not None
    assert "alice" in event.message
    assert "ls -la" in event.message
