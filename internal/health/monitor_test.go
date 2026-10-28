package health

import (
	"strings"
	"testing"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/config"
	"github.com/Shivamingale3/pi_dex/internal/core"
)

func TestTrackerAlertsOnceAfterThreshold(t *testing.T) {
	tr := newTracker(3)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	step := func(i int) time.Time { return t0.Add(time.Duration(i) * 30 * time.Second) }

	if got, _ := tr.observe(true, step(0)); got != NoAlert || tr.state != StateHealthy {
		t.Fatalf("first success: %v %v", got, tr.state)
	}
	for i := 1; i <= 2; i++ {
		if got, _ := tr.observe(false, step(i)); got != NoAlert || tr.state != StateFailing {
			t.Fatalf("failure %d: %v %v", i, got, tr.state)
		}
	}
	if got, _ := tr.observe(false, step(3)); got != WentDown || tr.state != StateDown {
		t.Fatalf("third failure: %v %v", got, tr.state)
	}
	for i := 4; i <= 6; i++ {
		if got, _ := tr.observe(false, step(i)); got != NoAlert {
			t.Fatalf("repeat alert at failure %d", i)
		}
	}
	got, downtime := tr.observe(true, step(7))
	if got != Recovered || tr.state != StateHealthy {
		t.Fatalf("recovery: %v %v", got, tr.state)
	}
	if want := 6 * 30 * time.Second; downtime != want {
		t.Errorf("downtime = %v, want %v (from first failure)", downtime, want)
	}
}

func TestTrackerFlappingBelowThresholdIsSilent(t *testing.T) {
	tr := newTracker(3)
	now := time.Now()
	seq := []bool{false, false, true, false, false, true}
	for i, ok := range seq {
		if got, _ := tr.observe(ok, now); got != NoAlert {
			t.Fatalf("step %d alerted: %v", i, got)
		}
	}
}

func TestTrackerStartupFailingAlertsAfterThreshold(t *testing.T) {
	tr := newTracker(2)
	now := time.Now()
	if got, _ := tr.observe(false, now); got != NoAlert || tr.state != StateFailing {
		t.Fatalf("first result failing should be silent: %v %v", got, tr.state)
	}
	if got, _ := tr.observe(false, now); got != WentDown {
		t.Fatalf("expected WentDown, got %v", got)
	}
}

func TestDownAndRecoveredEvents(t *testing.T) {
	hc := config.HealthCheckConfig{Name: "redis-cache", Type: config.HealthRedis, Address: "100.83.1.25:6379"}
	d := Diagnosis{
		Findings:  []string{"connection refused", "port 6379 is listening on 127.0.0.1:6379 only, not on 100.83.1.25"},
		LogSource: "journalctl -u redis-cache.service",
		Logs:      "line1\nline2",
	}
	down := DownEvent(hc, 3, d, time.Now())
	if down.Source != "health:redis-cache" || down.EventType != core.EventHealthDown || down.Severity != core.SeverityCritical {
		t.Errorf("down event = %+v", down)
	}
	for _, want := range []string{"redis 100.83.1.25:6379", "3 consecutive failures", "Cause:", "connection refused", "127.0.0.1:6379 only", "journalctl -u redis-cache.service"} {
		if !strings.Contains(down.Message, want) {
			t.Errorf("down message missing %q:\n%s", want, down.Message)
		}
	}
	if down.Details != "line1\nline2" {
		t.Errorf("details = %q", down.Details)
	}

	rec := RecoveredEvent(hc, 95*time.Second+400*time.Millisecond, time.Now())
	if rec.Source != down.Source || rec.Severity != core.SeverityRecovered || !strings.Contains(rec.Message, "after 1m35s down") {
		t.Errorf("recovered event = %+v", rec)
	}
}
