from pidex.sources.systemd import make_parser


def make_entry(identifier="systemd", unit="nginx.service", message=""):
    return {
        "SYSLOG_IDENTIFIER": identifier,
        "_SYSTEMD_UNIT": unit,
        "MESSAGE": message,
        "__REALTIME_TIMESTAMP": 1735689600000000,
    }


def test_service_started():
    parser = make_parser(["nginx*"])
    entry = make_entry(message="Started nginx.service.")
    event = parser(entry)
    assert event is not None
    assert event.event_type == "SERVICE_STARTED"
    assert event.severity == "INFO"
    assert "nginx.service" in event.message


def test_service_stopped():
    parser = make_parser(["nginx*"])
    entry = make_entry(message="Stopped nginx.service.")
    event = parser(entry)
    assert event is not None
    assert event.event_type == "SERVICE_STOPPED"
    assert event.severity == "WARN"


def test_service_failed():
    parser = make_parser(["cloudflared*"])
    entry = make_entry(message="Failed to start cloudflared.service.")
    event = parser(entry)
    assert event is not None
    assert event.event_type == "SERVICE_FAILED"
    assert event.severity == "CRITICAL"


def test_service_restarted():
    parser = make_parser(["docker*"])
    entry = make_entry(message="Restarted docker.service.")
    event = parser(entry)
    assert event is not None
    assert event.event_type == "SERVICE_RESTARTED"
    assert event.severity == "INFO"


def test_unwatched_service_not_matched():
    parser = make_parser(["nginx*"])
    entry = make_entry(message="Started apache2.service.")
    event = parser(entry)
    assert event is None


def test_watch_all_if_empty():
    parser = make_parser([])
    entry = make_entry(message="Started random.service.")
    event = parser(entry)
    assert event is not None
    assert event.event_type == "SERVICE_STARTED"


def test_exact_pattern_match():
    parser = make_parser(["docker.service"])
    entry = make_entry(message="Started docker.service.")
    event = parser(entry)
    assert event is not None
    assert event.event_type == "SERVICE_STARTED"


def test_rejects_non_systemd():
    parser = make_parser(["anything"])
    entry = make_entry(identifier="sshd", unit="", message="Started anything.service.")
    event = parser(entry)
    assert event is None


def test_unknown_message_returns_none():
    parser = make_parser(["*"])
    entry = make_entry(message="Some random systemd message.")
    event = parser(entry)
    assert event is None
