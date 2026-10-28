package notifier

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

func testNotifier(url string) *TelegramNotifier {
	n := NewTelegramNotifier("token", "42")
	n.apiURL = url
	n.hostname = "pi"
	n.sleep = func(time.Duration) {}
	return n
}

func sampleEvent() core.Event {
	return core.Event{
		Source:    "ssh",
		EventType: core.EventSSHLogin,
		Severity:  core.SeverityInfo,
		Title:     "SSH Login",
		Message:   "pi logged in from <10.0.0.2>",
		Timestamp: time.Date(2026, 6, 3, 15, 4, 5, 0, time.UTC),
	}
}

func TestFormatMessageEscapesAndMarksDelayed(t *testing.T) {
	n := testNotifier("")
	e := sampleEvent()
	got := n.formatMessage(e)
	if !strings.Contains(got, "pi logged in from &lt;10.0.0.2&gt;") {
		t.Errorf("message not escaped:\n%s", got)
	}
	if strings.Contains(got, "Delayed") {
		t.Errorf("live message has Delayed line")
	}

	e.Delayed = true
	got = n.formatMessage(e)
	if !strings.Contains(got, "Delayed: originally at 2026-06-03 15:04:05") {
		t.Errorf("missing Delayed line:\n%s", got)
	}
}

func TestFormatMessageTruncatesToTelegramLimit(t *testing.T) {
	n := testNotifier("")
	e := sampleEvent()
	e.Message = strings.Repeat("a&b ", 2000)
	e.Details = strings.Repeat("log line with <tags> é\n", 500)
	got := n.formatMessage(e)
	if len(got) > maxMessageLen {
		t.Fatalf("len = %d, want <= %d", len(got), maxMessageLen)
	}
	if !utf8.ValidString(got) {
		t.Fatal("truncation split a UTF-8 sequence")
	}
	if !strings.HasSuffix(got, "— PiDex v"+core.Version) {
		t.Errorf("footer lost after truncation")
	}

	e.Message = "short"
	got = n.formatMessage(e)
	if len(got) > maxMessageLen || !strings.Contains(got, "<pre>") || !strings.Contains(got, "</pre>") {
		t.Errorf("details block missing or too long (len %d)", len(got))
	}
}

func TestTruncateEscapedKeepsEntitiesWhole(t *testing.T) {
	s := "abc&amp;def"
	for max := 0; max <= len(s); max++ {
		got := truncateEscaped(s, max)
		if len(got) > max {
			t.Errorf("max %d: len %d", max, len(got))
		}
		trimmed := strings.TrimSuffix(got, "…")
		if i := strings.LastIndexByte(trimmed, '&'); i >= 0 && !strings.Contains(trimmed[i:], ";") {
			t.Errorf("max %d: split entity in %q", max, got)
		}
	}
}

func TestSendClassifiesErrors(t *testing.T) {
	status := http.StatusOK
	var calls int
	var gotText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotText, _ = body["text"].(string)
		w.WriteHeader(status)
		w.Write([]byte(`{"ok":false,"description":"Bad Request: can't parse entities"}`))
	}))
	defer srv.Close()
	n := testNotifier(srv.URL)

	if err := n.Send(sampleEvent()); err != nil {
		t.Fatalf("200: %v", err)
	}
	if !strings.Contains(gotText, "SSH Login") {
		t.Errorf("text = %q", gotText)
	}

	status, calls = http.StatusBadRequest, 0
	err := n.Send(sampleEvent())
	if !IsPermanent(err) || calls != 1 {
		t.Errorf("400: err=%v calls=%d, want permanent after 1 call", err, calls)
	}
	if !strings.Contains(err.Error(), "can't parse entities") {
		t.Errorf("400 error lacks description: %v", err)
	}

	status, calls = http.StatusBadGateway, 0
	err = n.Send(sampleEvent())
	if err == nil || IsPermanent(err) || calls != maxRetries {
		t.Errorf("502: err=%v calls=%d, want transient after %d calls", err, calls, maxRetries)
	}
}

func TestSendNetworkErrorIsTransient(t *testing.T) {
	n := testNotifier("http://127.0.0.1:1/unreachable")
	err := n.Send(sampleEvent())
	if err == nil || IsPermanent(err) {
		t.Errorf("err = %v, want transient", err)
	}
}
