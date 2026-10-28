package queue

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

func ev(i int) core.Event {
	return core.Event{
		Source:    "test",
		EventType: "TEST",
		Severity:  core.SeverityInfo,
		Title:     "Test",
		Message:   fmt.Sprintf("event %d", i),
		Timestamp: time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC),
	}
}

func TestQueuePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.jsonl")
	q, err := Open(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		if err := q.Push(ev(i)); err != nil {
			t.Fatal(err)
		}
	}
	_, id, _ := q.Front()
	if err := q.Remove(id); err != nil {
		t.Fatal(err)
	}

	q2, err := Open(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if q2.Len() != 2 {
		t.Fatalf("Len = %d, want 2", q2.Len())
	}
	front, _, _ := q2.Front()
	if front.Message != "event 2" || !front.Timestamp.Equal(ev(2).Timestamp) {
		t.Errorf("front = %+v, want event 2", front)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
	if matches, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".queue.jsonl.tmp-*")); len(matches) != 0 {
		t.Errorf("temp files left behind: %v", matches)
	}
}

func TestQueueRemovesFileWhenEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.jsonl")
	q, _ := Open(path, 10)
	q.Push(ev(1))
	_, id, _ := q.Front()
	q.Remove(id)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("queue file should be removed when empty, stat err = %v", err)
	}
}

func TestQueueCapDropsOldest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.jsonl")
	q, _ := Open(path, 3)
	for i := 1; i <= 5; i++ {
		q.Push(ev(i))
	}
	if q.Len() != 3 || q.Dropped() != 2 {
		t.Fatalf("Len=%d Dropped=%d, want 3 and 2", q.Len(), q.Dropped())
	}
	front, _, _ := q.Front()
	if front.Message != "event 3" {
		t.Errorf("front = %q, want event 3", front.Message)
	}

	q2, _ := Open(path, 3)
	if q2.Dropped() != 2 || q2.Len() != 3 {
		t.Errorf("after reopen Len=%d Dropped=%d, want 3 and 2", q2.Len(), q2.Dropped())
	}
}

func TestRemoveAfterCapDropDoesNotLoseUnsentEvent(t *testing.T) {
	q, _ := Open("", 2)
	q.Push(ev(1))
	q.Push(ev(2))
	_, id, _ := q.Front() // flusher is sending event 1...
	q.Push(ev(3))         // ...while the cap drops it
	q.Remove(id)
	if q.Len() != 2 {
		t.Fatalf("Len = %d, want 2", q.Len())
	}
	front, _, _ := q.Front()
	if front.Message != "event 2" {
		t.Errorf("front = %q, want event 2 (unsent event was removed)", front.Message)
	}
}

func TestQueueSkipsCorruptLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.jsonl")
	data := `{"event":{"message":"a"}}` + "\n" + `{not json` + "\n" + `{"event":{"message":"b"}}` + "\n"
	os.WriteFile(path, []byte(data), 0600)
	q, err := Open(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if q.Len() != 2 {
		t.Errorf("Len = %d, want 2", q.Len())
	}
}

type fakeSender struct {
	fail    bool
	failMsg map[string]error
	sent    []core.Event
}

func (f *fakeSender) Send(e core.Event) error {
	if err := f.failMsg[e.Message]; err != nil {
		return err
	}
	if f.fail {
		return errors.New("offline")
	}
	f.sent = append(f.sent, e)
	return nil
}

var errPermanent = errors.New("permanent")

func newTestNotifier(t *testing.T, s *fakeSender, max int) (*Notifier, *Queue) {
	t.Helper()
	q, err := Open(filepath.Join(t.TempDir(), "queue.jsonl"), max)
	if err != nil {
		t.Fatal(err)
	}
	n := NewNotifier(s, q, func(err error) bool { return errors.Is(err, errPermanent) })
	n.sleep = func(time.Duration) {}
	return n, q
}

func TestNotifierQueuesWhileOfflineAndFlushesInOrder(t *testing.T) {
	s := &fakeSender{fail: true}
	n, q := newTestNotifier(t, s, 10)

	if err := n.Send(ev(1)); err != nil {
		t.Fatalf("Send while offline should queue, not fail: %v", err)
	}
	s.fail = false
	// Back online, but the queue is not empty: new events must go behind it.
	n.Send(ev(2))
	if len(s.sent) != 0 || q.Len() != 2 {
		t.Fatalf("sent=%d queued=%d, want 0 and 2", len(s.sent), q.Len())
	}

	sent, err := n.Flush()
	if err != nil || sent != 2 {
		t.Fatalf("Flush = %d, %v", sent, err)
	}
	for i, e := range s.sent {
		if e.Message != fmt.Sprintf("event %d", i+1) {
			t.Errorf("sent[%d] = %q, out of order", i, e.Message)
		}
		if !e.Delayed {
			t.Errorf("sent[%d] not marked Delayed", i)
		}
	}

	n.Send(ev(3))
	if len(s.sent) != 3 || s.sent[2].Delayed {
		t.Errorf("live send after flush: sent=%d delayed=%v", len(s.sent), s.sent[len(s.sent)-1].Delayed)
	}
}

func TestFlushStopsAtFirstFailure(t *testing.T) {
	s := &fakeSender{fail: true}
	n, q := newTestNotifier(t, s, 10)
	n.Send(ev(1))
	n.Send(ev(2))

	sent, err := n.Flush()
	if err == nil || sent != 0 || q.Len() != 2 {
		t.Fatalf("Flush offline = %d, %v, queued %d", sent, err, q.Len())
	}

	s.fail = false
	s.failMsg = map[string]error{"event 2": errors.New("flaky")}
	sent, err = n.Flush()
	if err == nil || sent != 1 || q.Len() != 1 {
		t.Fatalf("Flush partial = %d, %v, queued %d", sent, err, q.Len())
	}
	front, _, _ := q.Front()
	if front.Message != "event 2" {
		t.Errorf("front = %q, want event 2", front.Message)
	}
}

func TestFlushDiscardsPermanentFailures(t *testing.T) {
	s := &fakeSender{fail: true}
	n, q := newTestNotifier(t, s, 10)
	n.Send(ev(1))
	n.Send(ev(2))
	s.fail = false
	s.failMsg = map[string]error{"event 1": errPermanent}

	sent, err := n.Flush()
	if err != nil || sent != 1 || q.Len() != 0 {
		t.Fatalf("Flush = %d, %v, queued %d", sent, err, q.Len())
	}
	if s.sent[0].Message != "event 2" {
		t.Errorf("sent %q", s.sent[0].Message)
	}
}

func TestSendDoesNotQueuePermanentFailure(t *testing.T) {
	s := &fakeSender{failMsg: map[string]error{"event 1": errPermanent}}
	n, q := newTestNotifier(t, s, 10)
	if err := n.Send(ev(1)); !errors.Is(err, errPermanent) {
		t.Errorf("Send err = %v, want permanent", err)
	}
	if q.Len() != 0 {
		t.Errorf("permanent failure was queued")
	}
}

func TestFlushSendsDroppedSummaryFirst(t *testing.T) {
	s := &fakeSender{fail: true}
	n, q := newTestNotifier(t, s, 2)
	for i := 1; i <= 5; i++ {
		n.Send(ev(i))
	}
	if q.Dropped() != 3 {
		t.Fatalf("Dropped = %d, want 3", q.Dropped())
	}
	s.fail = false
	if _, err := n.Flush(); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 3 {
		t.Fatalf("sent %d, want summary + 2", len(s.sent))
	}
	if s.sent[0].EventType != core.EventAlertsDropped || !strings.Contains(s.sent[0].Message, "3 alerts dropped while offline") {
		t.Errorf("first message = %+v", s.sent[0])
	}
	if s.sent[1].Message != "event 4" || s.sent[2].Message != "event 5" {
		t.Errorf("remaining = %q, %q", s.sent[1].Message, s.sent[2].Message)
	}
	if q.Dropped() != 0 {
		t.Errorf("dropped counter not reset")
	}
}
